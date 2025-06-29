package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"resizer/api/model"
	"resizer/config"
	"resizer/converter/image"
	"resizer/shared/log"
)

var ErrNotFound = errors.New("object not found in S3")

type ImageService struct {
	config *config.Config

	s3 *s3.S3

	strategy *image.Strategy

	redis *redis.Client

	logger *zap.Logger
}

func NewImageService(s3 *s3.S3, c *config.Config, strategy *image.Strategy, logger *zap.Logger, redis *redis.Client) *ImageService {
	return &ImageService{s3: s3, config: c, strategy: strategy, logger: logger, redis: redis}
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
	redisKey := fmt.Sprintf("img:%s", key)

	// 1. Пробуем получить из Redis если кеширование включено
	if i.config.UseRedisCache && i.redis != nil {
		logger.Debug("проверяем Redis кеш", zap.String("key", redisKey))
		imageData, err := i.tryGetFromRedis(ctx, redisKey)
		if err == nil {
			logger.Info("изображение получено из Redis кеша", zap.String("key", redisKey))
			return imageData, nil
		}
		logger.Debug("не найдено в Redis", zap.Error(err))
	}

	// 2. Пробуем получить из S3
	logger.Debug("проверяем S3 кеш", zap.String("key", key))
	imageData, err := i.tryGetFromS3(ctx, bucket, key)
	if err == nil {
		// Если нашли в S3 и Redis включен, кешируем в Redis
		if i.config.UseRedisCache && i.redis != nil {
			i.cacheInRedis(ctx, redisKey, imageData)
		}
		logger.Info("изображение получено из S3", zap.String("key", key))
		return imageData, nil
	}

	if !isNotFoundError(err) {
		logger.Error("ошибка при получении из S3", zap.Error(err))
		return nil, err
	}

	logger.Debug("не найдено в S3", zap.String("key", key))

	// 3. Получаем от внешнего сервиса
	logger.Debug("запрашиваем у внешнего сервиса", zap.String("url", url))
	imageData, err = i.fetchFromExternalService(ctx, url)
	if err != nil {
		logger.Error("ошибка при запросе к внешнему сервису", zap.Error(err))
		return nil, err
	}

	// 4. Кешируем результат
	// В Redis (если включено)
	if i.config.UseRedisCache && i.redis != nil {
		i.cacheInRedis(ctx, redisKey, imageData)
	}

	// В S3 (асинхронно)
	go i.cacheInS3(bucket, key, imageData, url)

	logger.Info("изображение получено от внешнего сервиса", zap.String("url", url))
	return imageData, nil
}

// tryGetFromRedis пытается получить изображение из Redis кеша
func (i *ImageService) tryGetFromRedis(ctx context.Context, key string) (*ProxyResponse, error) {
	cachedData, err := i.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	// Формат данных: [2 байта размера заголовка][заголовки][содержимое]
	headerSize := int(cachedData[0])*256 + int(cachedData[1])

	headers := http.Header{}
	contentType := ""
	if headerSize > 0 {
		contentType = string(cachedData[2 : 2+headerSize])
		headers.Set("Content-Type", contentType)
		headers.Set("Content-Length", fmt.Sprint(len(cachedData)-2-headerSize))
	}

	imageBytes := cachedData[2+headerSize:]
	bodyReader := io.NopCloser(bytes.NewReader(imageBytes))

	return &ProxyResponse{
		Body:        bodyReader,
		Headers:     headers,
		StatusCode:  http.StatusOK,
		rawBytes:    imageBytes,
		contentType: contentType,
	}, nil
}

// tryGetFromS3 пытается получить изображение из S3
func (i *ImageService) tryGetFromS3(ctx context.Context, bucket, key string) (*ProxyResponse, error) {
	getOut, err := i.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, err
	}

	// Проверяем, что это не HTML ошибка (некоторые S3 сервисы возвращают HTML страницы ошибок)
	if getOut.ContentType != nil && *getOut.ContentType == "text/html" {
		getOut.Body.Close()
		return nil, errors.New("object is HTML page")
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

	// Читаем все содержимое для кеширования в Redis
	bodyBytes, err := ioutil.ReadAll(getOut.Body)
	getOut.Body.Close()
	if err != nil {
		return nil, err
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
func (i *ImageService) fetchFromExternalService(ctx context.Context, url string) (*ProxyResponse, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return &ProxyResponse{StatusCode: res.StatusCode}, nil
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
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

// cacheInRedis кеширует изображение в Redis
func (i *ImageService) cacheInRedis(ctx context.Context, key string, resp *ProxyResponse) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	// Если байты еще не извлечены, использовать пустой массив
	var imageBytes []byte
	var contentType string

	if resp.rawBytes != nil {
		imageBytes = resp.rawBytes
	} else {
		logger.Debug("пропускаем кеширование в Redis - нет доступных байтов")
		return
	}

	if ct := resp.Headers.Get("Content-Type"); ct != "" {
		contentType = ct
	} else if resp.contentType != "" {
		contentType = resp.contentType
	}

	headerSize := len(contentType)
	cacheData := make([]byte, 2+headerSize+len(imageBytes))

	// Формат: [2 байта размера заголовка][заголовок contentType][содержимое]
	cacheData[0] = byte(headerSize / 256)
	cacheData[1] = byte(headerSize % 256)
	copy(cacheData[2:2+headerSize], []byte(contentType))
	copy(cacheData[2+headerSize:], imageBytes)

	err := i.redis.Set(ctx, key, cacheData, i.config.RedisCacheTTL).Err()
	if err != nil {
		logger.Error("ошибка сохранения в Redis", zap.Error(err), zap.String("key", key))
	} else {
		logger.Debug("успешно сохранено в Redis", zap.String("key", key))
	}
}

// cacheInS3 асинхронно кеширует изображение в S3
func (i *ImageService) cacheInS3(bucket, key string, resp *ProxyResponse, url string) {
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

	uploader := s3manager.NewUploaderWithClient(i.s3)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(resp.rawBytes),
		ContentType: aws.String(contentType),
	})

	if err != nil {
		logger.Error("ошибка кеширования в S3", zap.Error(err), zap.String("key", key))
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
