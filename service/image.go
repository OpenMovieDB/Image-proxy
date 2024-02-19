package service

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
	"image"
	"log/slog"
	"resizer/api/model"
	"resizer/config"
	"resizer/converter"
)

type ImageService struct {
	config *config.Config

	s3 *s3.S3

	converter *converter.StrategyImpl
}

func NewImageService(s3 *s3.S3, c *config.Config, converter *converter.StrategyImpl) *ImageService {
	return &ImageService{s3: s3, config: c, converter: converter}
}

func (i *ImageService) Process(params model.ImageRequest) (*model.ImageResponse, error) {
	result, err := i.imageFromS3(params)
	if err != nil {
		return nil, err
	}

	formatType, err := converter.MakeFromString(params.Type)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	converterStrategy := i.converter.Apply(formatType)

	img, err := converterStrategy.Convert(result.Body, params.Quality, func(img image.Image) (image.Image, error) {
		width := params.Width
		height := img.Bounds().Dy() * width / img.Bounds().Dx()

		if width != img.Bounds().Dx() && width != 0 {
			return imaging.Resize(img, width, height, imaging.MitchellNetravali), nil
		}

		return img, nil
	})
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	return &model.ImageResponse{
		Body:               img,
		ContentDisposition: fmt.Sprintf("inline; filename=%s.%s", params.FileID, params.Type),
		Type:               params.Type,
	}, nil
}

func (i *ImageService) imageFromS3(params model.ImageRequest) (*s3.GetObjectOutput, error) {
	fileKey := fmt.Sprintf("%s/%s", params.EntityID, params.FileID)

	input := &s3.GetObjectInput{
		Bucket: &i.config.S3Bucket,
		Key:    &fileKey,
	}

	result, err := i.s3.GetObject(input)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}
	return result, nil
}
