package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"resizer/api/model"
	"resizer/config"
	"resizer/converter/image"
	"resizer/shared/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.uber.org/zap"
)

var ErrNotFound = errors.New("object not found in S3")

type ImageService struct {
	config *config.Config

	s3 *s3.S3

	strategy *image.Strategy

	logger *zap.Logger

	// HTTP клиент для внешних запросов (переиспользуется)
	httpClient *http.Client

	// для записи неуспешных URL
	failedURLsFile  *os.File
	failedURLsMutex sync.Mutex

	// semaphore для ограничения количества одновременных операций кеширования
	cacheSemaphore chan struct{}

	// circuit breaker для S3
	s3Available      bool
	s3AvailableMutex sync.RWMutex
	s3FailureCount   int
	s3LastCheck      time.Time
}

func NewImageService(s3 *s3.S3, c *config.Config, strategy *image.Strategy, logger *zap.Logger) *ImageService {
	service := &ImageService{
		s3:       s3,
		config:   c,
		strategy: strategy,
		logger:   logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:          200,
				MaxIdleConnsPerHost:   100,
				MaxConnsPerHost:       0, // без лимита
				IdleConnTimeout:       90 * time.Second,
				DisableKeepAlives:     false,
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
		cacheSemaphore: make(chan struct{}, 50), // Ограничиваем до 50 одновременных операций кеширования
		s3Available:    true,                    // Изначально считаем S3 доступным
		s3LastCheck:    time.Now(),
	}
	service.initFailedURLsFile()

	// Запускаем горутину для мониторинга состояния S3
	go service.monitorS3Health()

	return service
}

func (i *ImageService) Process(ctx context.Context, params model.ImageRequest) (*model.ImageResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	result, err := i.getFromS3(ctx, params)
	if err != nil {
		logger.Error("Error getting image from S3", zap.Error(err))
		return nil, err
	}

	if *result.ContentType == "image/svg+xml" {
		return &model.ImageResponse{
			Body:               result.Body,
			ContentLength:      *result.ContentLength,
			ContentDisposition: fmt.Sprintf("inline; filename=%s.%s", params.File, result.ContentType),
			Type:               params.Type.String(),
		}, nil
	}

	customImage := image.NewCustomImage(i.strategy.Apply(params.Type))
	if err = customImage.Decode(result.Body); err != nil {
		logger.Error("Error decoding format type", zap.Error(err))
		return nil, err
	}

	customImage.Transform(image.WithWidth(params.Width))

	img, contentLength, err := customImage.Encode(ctx, params.Quality)
	if err != nil {
		logger.Error("Error encoding format type", zap.Error(err))
		return nil, err
	}

	logger.Debug(fmt.Sprintf("Image %s converted to %s, quality: %f, width: %d", params.File, params.Type, params.Quality, params.Width))

	return &model.ImageResponse{
		Body:               img,
		ContentLength:      contentLength,
		ContentDisposition: fmt.Sprintf("inline; filename=%s.%s", params.File, params.Type),
		Type:               params.Type.String(),
	}, nil
}

func (i *ImageService) getFromS3(ctx context.Context, params model.ImageRequest) (*s3.GetObjectOutput, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	fileKey := fmt.Sprintf("%s/%s", params.Entity, params.File)
	logger = logger.With(zap.String("fileKey", fileKey))

	result, err := i.s3.GetObject(&s3.GetObjectInput{Bucket: &i.config.S3Bucket, Key: &fileKey})
	if err != nil {
		logger.Error(fmt.Sprintf("Error getting object from bucket %s", i.config.S3Bucket), zap.Error(err))
		return nil, err
	}

	logger.Debug("Image was fetched from S3")

	return result, nil
}

type ProxyResponse struct {
	Body        io.ReadCloser
	Headers     http.Header
	StatusCode  int
	contentType string
}

func (i *ImageService) ProxyImage(ctx context.Context, serviceType model.ServiceName, rawPath string) (*ProxyResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	key := path.Join("proxy", serviceType.String(), rawPath)
	bucket := i.config.S3Bucket
	url := serviceType.ToProxyURL(i.config.TMDBImageProxy) + rawPath

	// 1. Проверяем доступность S3 перед попыткой чтения
	s3Available := i.isS3Available()

	if s3Available {
		// Пробуем получить из S3 только если он доступен
		logger.Debug("проверяем S3 кеш", zap.String("key", key))
		imageData, err := i.tryGetFromS3(ctx, bucket, key)
		if err == nil && imageData != nil {
			logger.Info("изображение получено из S3", zap.String("key", key))
			return imageData, nil
		}

		// Если нашли HTML в кеше - удаляем его и получаем свежие данные
		if err != nil && strings.Contains(err.Error(), "object is HTML page") {
			logger.Warn("обнаружен HTML в кеше, удаляем и перезагружаем", zap.String("key", key))
			go i.deleteFromS3(bucket, key)
		} else if !isNotFoundError(err) {
			logger.Warn("ошибка при получении из S3, используем fallback на внешний сервис", zap.Error(err))
			i.recordS3Failure()
		}
	} else {
		logger.Debug("S3 недоступен, пропускаем попытку чтения из кеша", zap.String("key", key))
	}

	logger.Debug("не найдено в S3, запрашиваем у внешнего сервиса", zap.String("url", url))

	// 2. Получаем от внешнего сервиса со стримингом
	return i.fetchAndStream(ctx, url, serviceType, rawPath, bucket, key)
}

// tryGetFromS3 пытается получить изображение из S3 (стримит напрямую)
func (i *ImageService) tryGetFromS3(ctx context.Context, bucket, key string) (*ProxyResponse, error) {
	s3Ctx, cancel := context.WithTimeout(ctx, 10*time.Second)

	getOut, err := i.s3.GetObjectWithContext(s3Ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		cancel()
		return nil, err
	}

	if getOut.Body == nil {
		cancel()
		return nil, errors.New("S3 returned nil body")
	}

	headers := http.Header{}
	contentType := ""
	if getOut.ContentType != nil {
		contentType = aws.StringValue(getOut.ContentType)
		headers.Set("Content-Type", contentType)
	}
	if getOut.ContentLength != nil {
		headers.Set("Content-Length", fmt.Sprint(*getOut.ContentLength))
	}

	// Проверяем Content-Type на HTML (без чтения body)
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		getOut.Body.Close()
		cancel()
		return nil, errors.New("object is HTML page")
	}

	// Оборачиваем body чтобы cancel вызвался при закрытии
	wrappedBody := &cancelOnCloseReader{
		ReadCloser: getOut.Body,
		cancel:     cancel,
	}

	return &ProxyResponse{
		Body:        wrappedBody,
		Headers:     headers,
		StatusCode:  http.StatusOK,
		contentType: contentType,
	}, nil
}

// cancelOnCloseReader вызывает cancel при закрытии reader
type cancelOnCloseReader struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelOnCloseReader) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

// fetchAndStream получает изображение от внешнего сервиса и стримит клиенту,
// параллельно записывая в S3 если он доступен
func (i *ImageService) fetchAndStream(ctx context.Context, url string, serviceType model.ServiceName, rawPath, bucket, key string) (*ProxyResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		failedPath := fmt.Sprintf("/%s/%s", serviceType.String(), rawPath)
		i.logFailedURL(failedPath, res.StatusCode)
		return nil, fmt.Errorf("external service returned status %d for %s", res.StatusCode, url)
	}

	contentType := res.Header.Get("Content-Type")

	// Проверяем что это не HTML
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		res.Body.Close()
		return nil, fmt.Errorf("external service returned HTML for %s", url)
	}

	headers := make(http.Header)
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	if res.ContentLength > 0 {
		headers.Set("Content-Length", fmt.Sprint(res.ContentLength))
	}

	// Если S3 доступен и это валидный image content-type - используем TeeReader для кеширования
	if i.isS3Available() && i.isValidImageContentType(contentType) {
		pr, pw := io.Pipe()

		// Горутина для записи в S3
		go func() {
			defer pw.Close()

			// Проверяем semaphore
			select {
			case i.cacheSemaphore <- struct{}{}:
				defer func() { <-i.cacheSemaphore }()
			default:
				logger.Warn("пропускаем кеширование - достигнут лимит", zap.String("key", key))
				// Читаем и отбрасываем данные из pipe чтобы не заблокировать writer
				io.Copy(io.Discard, pr)
				return
			}

			uploadCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			uploader := s3manager.NewUploaderWithClient(i.s3)
			_, err := uploader.UploadWithContext(uploadCtx, &s3manager.UploadInput{
				Bucket:      aws.String(bucket),
				Key:         aws.String(key),
				Body:        pr,
				ContentType: aws.String(contentType),
			})

			if err != nil {
				logger.Error("ошибка кеширования в S3", zap.Error(err), zap.String("key", key))
			} else {
				logger.Debug("успешно сохранено в S3", zap.String("key", key))
			}
		}()

		// TeeReader: читаем из res.Body, пишем в pw (который читает горутина S3)
		teeReader := io.TeeReader(res.Body, pw)

		// Оборачиваем в ReadCloser который закроет оригинальный body
		body := &teeReadCloser{
			Reader: teeReader,
			closer: res.Body,
			pipe:   pw,
		}

		logger.Info("стримим изображение с параллельным кешированием", zap.String("url", url))
		return &ProxyResponse{
			Body:        body,
			Headers:     headers,
			StatusCode:  http.StatusOK,
			contentType: contentType,
		}, nil
	}

	// S3 недоступен или невалидный content-type - просто стримим
	logger.Info("стримим изображение без кеширования", zap.String("url", url))
	return &ProxyResponse{
		Body:        res.Body,
		Headers:     headers,
		StatusCode:  http.StatusOK,
		contentType: contentType,
	}, nil
}

// teeReadCloser оборачивает TeeReader и закрывает underlying body и pipe
type teeReadCloser struct {
	io.Reader
	closer io.Closer
	pipe   *io.PipeWriter
}

func (t *teeReadCloser) Close() error {
	t.pipe.Close()
	return t.closer.Close()
}

// isValidImageContentType проверяет content-type
func (i *ImageService) isValidImageContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	validTypes := []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml", "image/avif"}
	for _, vt := range validTypes {
		if strings.Contains(ct, vt) {
			return true
		}
	}
	return false
}

func isNotFoundError(err error) bool {
	if aerr, ok := err.(s3.RequestFailure); ok && aerr.StatusCode() == http.StatusNotFound {
		return true
	}
	return false
}

// deleteFromS3 удаляет объект из S3
func (i *ImageService) deleteFromS3(bucket, key string) {
	logger := i.logger
	_, err := i.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		logger.Error("ошибка удаления из S3", zap.Error(err), zap.String("key", key))
	} else {
		logger.Info("объект удален из S3", zap.String("key", key))
	}
}

// initFailedURLsFile инициализирует файл для записи неуспешных URL
func (i *ImageService) initFailedURLsFile() {
	file, err := os.OpenFile("failed_urls.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		i.logger.Error("не удалось создать файл для неуспешных URL", zap.Error(err))
		return
	}
	i.failedURLsFile = file
}

// logFailedURL записывает неуспешную ссылку в файл
func (i *ImageService) logFailedURL(url string, statusCode int) {
	if i.failedURLsFile == nil {
		return
	}

	i.failedURLsMutex.Lock()
	defer i.failedURLsMutex.Unlock()

	logLine := fmt.Sprintf("%s\n", url)

	_, err := i.failedURLsFile.WriteString(logLine)
	if err != nil {
		i.logger.Error("ошибка записи в файл неуспешных URL", zap.Error(err))
	}

	// Принудительно сбрасываем буфер
	i.failedURLsFile.Sync()

	i.logger.Info("записана неуспешная ссылка", zap.String("url", url), zap.Int("status", statusCode))
}

// monitorS3Health периодически проверяет состояние S3
func (i *ImageService) monitorS3Health() {
	ticker := time.NewTicker(10 * time.Second) // Проверка каждые 10 секунд
	defer ticker.Stop()

	for range ticker.C {
		i.checkS3Health()
	}
}

// checkS3Health проверяет доступность S3
func (i *ImageService) checkS3Health() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Пробуем сделать простой HEAD запрос к бакету
	_, err := i.s3.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(i.config.S3Bucket),
	})

	i.s3AvailableMutex.Lock()
	defer i.s3AvailableMutex.Unlock()

	if err != nil {
		i.s3FailureCount++
		// Если 3 проверки подряд не удались - считаем S3 недоступным
		if i.s3FailureCount >= 3 {
			if i.s3Available {
				i.logger.Warn("S3 недоступен, переключаемся на прямые запросы к источникам",
					zap.Int("failures", i.s3FailureCount),
					zap.Error(err))
			}
			i.s3Available = false
		}
	} else {
		// S3 доступен
		if !i.s3Available {
			i.logger.Info("S3 снова доступен, возобновляем кеширование")
		}
		i.s3Available = true
		i.s3FailureCount = 0
	}
	i.s3LastCheck = time.Now()
}

// isS3Available проверяет доступность S3 (быстрая read-only операция)
func (i *ImageService) isS3Available() bool {
	i.s3AvailableMutex.RLock()
	defer i.s3AvailableMutex.RUnlock()
	return i.s3Available
}

// recordS3Failure записывает ошибку S3 для быстрого открытия circuit breaker
func (i *ImageService) recordS3Failure() {
	i.s3AvailableMutex.Lock()
	defer i.s3AvailableMutex.Unlock()

	i.s3FailureCount++
	// Если получаем много ошибок подряд - сразу открываем circuit
	if i.s3FailureCount >= 5 && i.s3Available {
		i.logger.Warn("слишком много ошибок S3, временно отключаем",
			zap.Int("failures", i.s3FailureCount))
		i.s3Available = false
	}
}

// Close закрывает файл неуспешных URL
func (i *ImageService) Close() error {
	if i.failedURLsFile != nil {
		return i.failedURLsFile.Close()
	}
	return nil
}

// ClearFailedURLs очищает файл с битыми URL
func (i *ImageService) ClearFailedURLs() error {
	i.failedURLsMutex.Lock()
	defer i.failedURLsMutex.Unlock()

	// Закрываем текущий файл
	if i.failedURLsFile != nil {
		i.failedURLsFile.Close()
	}

	// Удаляем существующий файл
	if err := os.Remove("failed_urls.txt"); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Создаем новый пустой файл
	file, err := os.OpenFile("failed_urls.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	i.failedURLsFile = file
	i.logger.Info("файл с битыми URL очищен")
	return nil
}
