package service

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
	"go.uber.org/zap"
	"image"
	"resizer/api/model"
	"resizer/config"
	"resizer/converter"
	"resizer/shared/log"
)

type ImageService struct {
	config *config.Config

	s3 *s3.S3

	converter *converter.StrategyImpl

	logger *zap.Logger
}

func NewImageService(s3 *s3.S3, c *config.Config, converter *converter.StrategyImpl, logger *zap.Logger) *ImageService {
	return &ImageService{s3: s3, config: c, converter: converter, logger: logger}
}

func (i *ImageService) Process(ctx context.Context, params model.ImageRequest) (*model.ImageResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	result, err := i.imageFromS3(ctx, params)
	if err != nil {
		logger.Error("Error getting image from S3", zap.Error(err))
		return nil, err
	}

	formatType, err := converter.MakeFromString(params.Type)
	if err != nil {
		logger.Error("Error converting format type", zap.Error(err))
		return nil, err
	}

	converterStrategy := i.converter.Apply(formatType)

	img, contentLength, err := converterStrategy.Convert(ctx, result.Body, params.Quality, func(img image.Image) (image.Image, error) {
		width := params.Width
		height := img.Bounds().Dy() * width / img.Bounds().Dx()

		if width != img.Bounds().Dx() && width != 0 {
			return imaging.Resize(img, width, height, imaging.MitchellNetravali), nil
		}

		return img, nil
	})
	if err != nil {
		logger.Error("Error converting format type", zap.Error(err))
		return nil, err
	}

	response := &model.ImageResponse{
		Body:               img,
		ContentLength:      contentLength,
		ContentDisposition: fmt.Sprintf("inline; filename=%s.%s", params.FileID, params.Type),
		Type:               params.Type,
	}

	logger.Debug(fmt.Sprintf("Image processed with params: %++v", params))

	return response, nil
}

func (i *ImageService) imageFromS3(ctx context.Context, params model.ImageRequest) (*s3.GetObjectOutput, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	fileKey := fmt.Sprintf("%s/%s", params.EntityID, params.FileID)

	input := &s3.GetObjectInput{Bucket: &i.config.S3Bucket, Key: &fileKey}

	result, err := i.s3.GetObject(input)
	if err != nil {
		logger.Error(fmt.Sprintf("Error getting object %s from bucket %s", fileKey, i.config.S3Bucket), zap.Error(err))
		return nil, err
	}

	logger.Debug(fmt.Sprintf("Image %s fetched from S3", fileKey))

	return result, nil
}
