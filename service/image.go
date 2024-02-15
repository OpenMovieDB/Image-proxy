package service

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"resizer/api/model"
)

type ImageService struct {
	s3 *s3.S3
}

func NewImageService(s3 *s3.S3) *ImageService {
	return &ImageService{s3: s3}
}

func (i *ImageService) Image(entityID int, imageID string, params model.ImageRequest) {

}
