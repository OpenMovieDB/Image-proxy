package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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
		s3:             s3,
		config:         c,
		strategy:       strategy,
		logger:         logger,
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
	rawBytes    []byte
	contentType string
}

func (i *ImageService) ProxyImage(ctx context.Context, serviceType model.ServiceName, rawPath string) (*ProxyResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	key := path.Join("proxy", serviceType.String(), rawPath)
	bucket := i.config.S3Bucket
	url := serviceType.ToProxyURL(i.config.TMDBImageProxy) + rawPath

	// 1. Проверяем доступность S3 перед попыткой чтения
	s3Available := i.isS3Available()

	var imageData *ProxyResponse
	var err error

	if s3Available {
		// Пробуем получить из S3 только если он доступен
		logger.Debug("проверяем S3 кеш", zap.String("key", key))
		imageData, err = i.tryGetFromS3(ctx, bucket, key)
		if err == nil && imageData != nil {
			logger.Info("изображение получено из S3", zap.String("key", key))
			return imageData, nil
		}

		// Если нашли HTML в кеше - удаляем его и получаем свежие данные
		if err != nil && strings.Contains(err.Error(), "object is HTML page") {
			logger.Warn("обнаружен HTML в кеше, удаляем и перезагружаем", zap.String("key", key))
			go i.deleteFromS3(bucket, key) // Удаляем асинхронно
		} else if !isNotFoundError(err) {
			logger.Warn("ошибка при получении из S3, используем fallback на внешний сервис", zap.Error(err))
			// Записываем ошибку для быстрого открытия circuit breaker
			i.recordS3Failure()
		}
	} else {
		logger.Debug("S3 недоступен, пропускаем попытку чтения из кеша", zap.String("key", key))
	}

	logger.Debug("не найдено в S3 или найден некорректный контент, используем внешний сервис", zap.String("key", key))

	// 2. Получаем от внешнего сервиса
	logger.Debug("запрашиваем у внешнего сервиса", zap.String("url", url))
	imageData, err = i.fetchFromExternalService(ctx, url, serviceType, rawPath)
	if err != nil {
		logger.Error("ошибка при запросе к внешнему сервису", zap.Error(err))
		return nil, err
	}

	// Дополнительная проверка на nil (защита от багов)
	if imageData == nil {
		logger.Error("fetchFromExternalService вернул nil response без ошибки")
		return nil, errors.New("internal error: nil response from external service")
	}

	// 3. Кешируем результат в S3 (асинхронно), если это валидное изображение и S3 доступен
	if i.isValidImageResponse(imageData) && i.isS3Available() {
		go i.cacheInS3(bucket, key, imageData, url)
	} else if !i.isS3Available() {
		logger.Debug("пропускаем кеширование - S3 недоступен", zap.String("key", key))
	} else {
		logger.Warn("не кешируем невалидный ответ", zap.String("url", url), zap.String("content_type", imageData.contentType))
	}

	logger.Info("изображение получено от внешнего сервиса", zap.String("url", url))
	return imageData, nil
}

// tryGetFromS3 пытается получить изображение из S3
func (i *ImageService) tryGetFromS3(ctx context.Context, bucket, key string) (*ProxyResponse, error) {
	// Создаем context с таймаутом для S3
	s3Ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	getOut, err := i.s3.GetObjectWithContext(s3Ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, err
	}

	// Проверка на nil Body
	if getOut.Body == nil {
		return nil, errors.New("S3 returned nil body")
	}

	// Читаем все содержимое
	bodyBytes, err := ioutil.ReadAll(getOut.Body)
	getOut.Body.Close()
	if err != nil {
		return nil, err
	}

	// Проверяем, что получили данные
	if len(bodyBytes) == 0 {
		return nil, errors.New("S3 returned empty body")
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

	// Проверяем, что это не HTML ошибка
	if i.isHTMLContent(contentType, bodyBytes) {
		return nil, errors.New("object is HTML page")
	}

	return &ProxyResponse{
		Body:        io.NopCloser(bytes.NewReader(bodyBytes)),
		Headers:     headers,
		StatusCode:  http.StatusOK,
		rawBytes:    bodyBytes,
		contentType: contentType,
	}, nil
}

// fetchFromExternalService получает изображение от внешнего сервиса
func (i *ImageService) fetchFromExternalService(ctx context.Context, url string, serviceType model.ServiceName, rawPath string) (*ProxyResponse, error) {
	// Создаем HTTP клиент с таймаутом для предотвращения зависания
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		// Записываем неуспешную ссылку в файл в формате /service-type/path
		failedPath := fmt.Sprintf("/%s/%s", serviceType.String(), rawPath)
		i.logFailedURL(failedPath, res.StatusCode)
		// Возвращаем error вместо пустого response, чтобы избежать nil pointers
		return nil, fmt.Errorf("external service returned status %d for %s", res.StatusCode, url)
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}

	// Проверяем, что получили данные
	if len(bodyBytes) == 0 {
		return nil, fmt.Errorf("external service returned empty body for %s", url)
	}

	headers := make(http.Header)
	contentType := res.Header.Get("Content-Type")
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	headers.Set("Content-Length", fmt.Sprint(len(bodyBytes)))

	return &ProxyResponse{
		Body:        io.NopCloser(bytes.NewReader(bodyBytes)),
		Headers:     headers,
		StatusCode:  http.StatusOK,
		rawBytes:    bodyBytes,
		contentType: contentType,
	}, nil
}

// cacheInS3 асинхронно кеширует изображение в S3
func (i *ImageService) cacheInS3(bucket, key string, resp *ProxyResponse, url string) {
	// Используем semaphore для ограничения количества одновременных операций
	select {
	case i.cacheSemaphore <- struct{}{}:
		// Получили слот, продолжаем
		defer func() { <-i.cacheSemaphore }() // Освобождаем слот после завершения
	default:
		// Нет свободных слотов, пропускаем кеширование
		i.logger.Warn("пропускаем кеширование - достигнут лимит одновременных операций", zap.String("key", key))
		return
	}

	logger := i.logger

	if resp.rawBytes == nil || len(resp.rawBytes) == 0 {
		logger.Error("пропускаем кеширование в S3 - нет доступных байтов")
		return
	}

	contentType := resp.Headers.Get("Content-Type")
	if contentType == "" && resp.contentType != "" {
		contentType = resp.contentType
	}

	logger.Debug("начинаем кеширование в S3", zap.String("key", key))

	// Создаем context с таймаутом для предотвращения зависания
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	uploader := s3manager.NewUploaderWithClient(i.s3)
	_, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(resp.rawBytes),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("таймаут кеширования в S3", zap.String("key", key))
		} else {
			logger.Error("ошибка кеширования в S3", zap.Error(err), zap.String("key", key))
		}
	} else {
		logger.Debug("успешно сохранено в S3", zap.String("key", key))
	}

	logger.Info("изображение прокcировано", zap.String("url", url), zap.String("key", key))
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

// isHTMLContent проверяет, является ли контент HTML
func (i *ImageService) isHTMLContent(contentType string, bodyBytes []byte) bool {
	// Проверяем Content-Type
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}

	// Проверяем начало содержимого
	if len(bodyBytes) > 0 {
		content := strings.ToLower(string(bodyBytes[:min(len(bodyBytes), 512)]))
		if strings.Contains(content, "<html") || strings.Contains(content, "<!doctype html") {
			return true
		}
	}

	return false
}

// isValidImageResponse проверяет, является ли ответ валидным изображением
func (i *ImageService) isValidImageResponse(resp *ProxyResponse) bool {
	if resp == nil || resp.rawBytes == nil || len(resp.rawBytes) == 0 {
		return false
	}

	// Проверяем, что это не HTML
	if i.isHTMLContent(resp.contentType, resp.rawBytes) {
		return false
	}

	// Проверяем Content-Type на валидные типы изображений
	contentType := strings.ToLower(resp.contentType)
	validTypes := []string{"image/jpeg", "image/png", "image/gif", "image/webp", "image/svg+xml", "image/avif"}
	for _, validType := range validTypes {
		if strings.Contains(contentType, validType) {
			return true
		}
	}

	// Если Content-Type не задан, проверяем по сигнатуре файла
	if contentType == "" || contentType == "application/octet-stream" {
		return i.isValidImageBySignature(resp.rawBytes)
	}

	return false
}

// isValidImageBySignature проверяет сигнатуру файла
func (i *ImageService) isValidImageBySignature(data []byte) bool {
	if len(data) < 8 {
		return false
	}

	// JPEG
	if data[0] == 0xFF && data[1] == 0xD8 {
		return true
	}
	// PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return true
	}
	// GIF
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return true
	}
	// WebP
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
