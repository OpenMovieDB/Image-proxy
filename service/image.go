package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.uber.org/zap"
	"io"
	"net/http"
	"path"
	"regexp"
	"resizer/api/model"
	"resizer/config"
	"resizer/converter/image"
	"resizer/shared/log"
)

var kinopoiskSizes = regexp.MustCompile(`(x1000|orig)$`)
var ErrNotFound = errors.New("object not found in S3")

type ImageService struct {
	config *config.Config

	s3 *s3.S3

	strategy *image.Strategy

	logger *zap.Logger
}

func NewImageService(s3 *s3.S3, c *config.Config, strategy *image.Strategy, logger *zap.Logger) *ImageService {
	return &ImageService{s3: s3, config: c, strategy: strategy, logger: logger}
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
	Body       io.ReadCloser
	Headers    http.Header
	StatusCode int
}

func (i *ImageService) ProxyImage(ctx context.Context, serviceType model.ServiceName, rawPath string) (*ProxyResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	key := path.Join("proxy", serviceType.String(), rawPath)
	bucket := i.config.S3Bucket

	getOut, err := i.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		logger.Debug("cache hit", zap.String("key", key))
		return &ProxyResponse{
			Body: getOut.Body,
			Headers: http.Header{
				"Content-Type":   {aws.StringValue(getOut.ContentType)},
				"Content-Length": {fmt.Sprint(*getOut.ContentLength)},
			},
			StatusCode: http.StatusOK,
		}, nil
	}

	if aerr, ok := err.(s3.RequestFailure); !ok || aerr.StatusCode() != http.StatusNotFound {
		logger.Error("error getting from S3", zap.Error(err), zap.String("key", key))
		return nil, err
	}

	url := serviceType.ToProxyURL(i.config.TMDBImageProxy) + rawPath
	if serviceType.String() == "kinopoisk-images" {
		url = kinopoiskSizes.ReplaceAllString(url, "440x660")
	}
	logger.Debug("cache miss, fetching remotely", zap.String("url", url))

	res, err := http.Get(url)
	if err != nil {
		logger.Error("http.Get failed", zap.Error(err), zap.String("url", url))
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		logger.Error("remote returned non-200", zap.Int("status", res.StatusCode))
		return &ProxyResponse{StatusCode: res.StatusCode}, nil
	}

	pipeR, pipeW := io.Pipe()

	go func() {
		defer res.Body.Close()
		defer pipeW.Close()

		// Создаем TeeReader для одновременной записи в S3 и передачи клиенту
		teeReader := io.TeeReader(res.Body, pipeW)

		// Используем s3manager для эффективной загрузки
		uploader := s3manager.NewUploaderWithClient(i.s3)
		_, err := uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        teeReader,
			ContentType: aws.String(res.Header.Get("Content-Type")),
		})

		if err != nil {
			logger.Error("failed to put object to S3", zap.Error(err), zap.String("key", key))
		} else {
			logger.Debug("cached image in S3", zap.String("key", key))
		}

		logger.Info("image proxied", zap.String("url", url), zap.String("key", key))
	}()

	return &ProxyResponse{
		Body:       pipeR,
		Headers:    res.Header,
		StatusCode: http.StatusOK,
	}, nil
}
