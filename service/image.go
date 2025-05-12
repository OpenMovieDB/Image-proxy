package service

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.uber.org/zap"
	"io"
	"net/http"
	"regexp"
	"resizer/api/model"
	"resizer/config"
	"resizer/converter/image"
	"resizer/shared/log"
)

var kinopoiskSizes = regexp.MustCompile(`(x1000|orig)$`)

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

func (i *ImageService) ProxyImage(ctx context.Context, serviceType model.ServiceName, path string) (*model.ImageResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	fileKey := fmt.Sprintf("%s/%s", serviceType.String(), path)
	logger = logger.With(zap.String("fileKey", fileKey))

	result, err := i.s3.GetObject(&s3.GetObjectInput{Bucket: &i.config.S3Bucket, Key: &fileKey})
	if err == nil {
		logger.Debug("Image found in S3")
		return &model.ImageResponse{
			Body:               result.Body,
			ContentLength:      *result.ContentLength,
			ContentDisposition: fmt.Sprintf("inline; filename=%s", fileKey),
			Type:               *result.ContentType,
		}, nil
	}

	url := serviceType.ToProxyURL(i.config.TMDBImageProxy) + path
	if serviceType.String() == "kinopoisk-images" {
		url = kinopoiskSizes.ReplaceAllString(url, "440x660")
	}

	logger.Debug(fmt.Sprintf("Proxying image from external service with URL: %s", url))

	res, err := http.Get(url)
	if err != nil {
		logger.Error("Error proxying image from external service", zap.Error(err))
		return nil, err
	}
	defer res.Body.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, res.Body)
	if err != nil {
		logger.Error("Error reading response body", zap.Error(err))
		return nil, err
	}

	contentType := res.Header.Get("Content-Type")

	_, err = i.s3.PutObject(&s3.PutObjectInput{
		Bucket:      &i.config.S3Bucket,
		Key:         &fileKey,
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: &contentType,
	})
	if err != nil {
		logger.Error("Error uploading image to S3", zap.Error(err))
		return nil, err
	}

	logger.Debug("Image uploaded to S3")

	return &model.ImageResponse{
		Body:               res.Body,
		ContentLength:      res.ContentLength,
		ContentDisposition: fmt.Sprintf("inline; filename=%s", fileKey),
		Type:               res.Header.Get("Content-Type"),
	}, nil
}
