package service

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"golang.org/x/image/webp"
	"image/png"
	"io"
	"log/slog"
	"resizer/api/model"
	"resizer/config"
)

type ImageService struct {
	config *config.Config

	s3 *s3.S3
}

func NewImageService(s3 *s3.S3, c *config.Config) *ImageService {
	return &ImageService{s3: s3, config: c}
}

func (i *ImageService) Process(params model.ImageRequest) (*model.ImageResponse, error) {
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

	// Изменение размера изображения и изменение качества
	img, err := webp.Decode(result.Body)
	if err != nil {
		slog.Error(err.Error())
		return nil, err
	}

	// конвертация webp в png
	pngImage := &bytes.Buffer{}
	png.Encode(pngImage, img)

	// io.Writer в io.Reader
	pngImage1 := io.Reader(pngImage)

	fileName := fmt.Sprintf("%s.%s", params.FileID, params.Type)

	response := &model.ImageResponse{
		Body:               pngImage1,
		ContentLength:      *result.ContentLength,
		ContentDisposition: fmt.Sprintf("inline; filename=%s", fileName),
		Type:               params.Type,
	}

	fmt.Println(fmt.Sprintf("result: %++v", result))
	return response, nil
}
