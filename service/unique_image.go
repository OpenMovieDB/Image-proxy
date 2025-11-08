package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"resizer/api/model"
	domainModel "resizer/domain/model"
	"resizer/converter/image"
	"resizer/shared/log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

func (i *ImageService) CreateImageFromSource(ctx context.Context, imageType domainModel.ImageType, service, path, sourceURL string) (*domainModel.Image, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	existing, err := i.repo.FindBySource(ctx, service, path)
	if err == nil {
		logger.Info("Image already exists in database", zap.String("id", existing.ID.Hex()))
		return existing, nil
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		logger.Error("Error checking for existing image", zap.Error(err))
		return nil, err
	}

	logger.Info("Fetching image from source", zap.String("url", sourceURL))
	resp, err := http.Get(sourceURL)
	if err != nil {
		logger.Error("Error fetching image from source", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Bad status from source", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("bad status from source: %d", resp.StatusCode)
	}

	sourceData, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading source data", zap.Error(err))
		return nil, err
	}

	img, err := i.processAndStore(ctx, sourceData, imageType)
	if err != nil {
		logger.Error("Error processing and storing image", zap.Error(err))
		return nil, err
	}

	img.Source = domainModel.ImageSource{
		Service: service,
		Path:    path,
	}

	if err := i.repo.Create(ctx, img); err != nil {
		logger.Error("Error creating image in database", zap.Error(err))
		i.cleanupS3(img)
		return nil, err
	}

	logger.Info("Image created successfully", zap.String("id", img.ID.Hex()))
	return img, nil
}

func (i *ImageService) GetImageByID(ctx context.Context, id bson.ObjectID, variant domainModel.ImageVariant) (*model.ImageResponse, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	img, err := i.repo.FindByID(ctx, id)
	if err != nil {
		logger.Error("Error finding image by ID", zap.Error(err))
		return nil, err
	}

	go i.repo.UpdateAccessTime(context.Background(), id)

	var s3Path string
	var size int64
	if variant == domainModel.ImageVariantOriginal {
		s3Path = img.Storage.OriginalPath
		size = img.Storage.OriginalSize
	} else {
		s3Path = img.Storage.PreviewPath
		size = img.Storage.PreviewSize
	}

	result, err := i.s3.GetObject(&s3.GetObjectInput{
		Bucket: &i.config.S3Bucket,
		Key:    &s3Path,
	})
	if err != nil {
		logger.Error("Error getting image from S3", zap.Error(err))
		return nil, err
	}

	return &model.ImageResponse{
		Body:               result.Body,
		ContentLength:      size,
		ContentDisposition: fmt.Sprintf("inline; filename=%s_%s.jpg", id.Hex(), variant),
		Type:               "image/jpeg",
	}, nil
}

func (i *ImageService) GetImageBySource(ctx context.Context, service, path string) (*domainModel.Image, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	img, err := i.repo.FindBySource(ctx, service, path)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			logger.Debug("Image not found in database", zap.String("service", service), zap.String("path", path))
			return nil, nil
		}
		logger.Error("Error finding image by source", zap.Error(err))
		return nil, err
	}

	return img, nil
}

func (i *ImageService) processAndStore(ctx context.Context, sourceData []byte, imageType domainModel.ImageType) (*domainModel.Image, error) {
	logger := log.LoggerWithTrace(ctx, i.logger)

	customImage := image.NewCustomImage(i.strategy.Apply(image.JPEG))
	if err := customImage.Decode(bytes.NewReader(sourceData)); err != nil {
		logger.Error("Error decoding image", zap.Error(err))
		return nil, err
	}

	customImage.Transform(image.WithUniquify())

	originalData, originalSize, err := customImage.Encode(ctx, 85)
	if err != nil {
		logger.Error("Error encoding original", zap.Error(err))
		return nil, err
	}

	originalBytes, err := io.ReadAll(originalData)
	if err != nil {
		logger.Error("Error reading original data", zap.Error(err))
		return nil, err
	}

	previewImage := image.NewCustomImage(i.strategy.Apply(image.JPEG))
	if err := previewImage.Decode(bytes.NewReader(originalBytes)); err != nil {
		logger.Error("Error decoding preview image", zap.Error(err))
		return nil, err
	}

	imgSize, err := previewImage.Size()
	if err != nil {
		logger.Error("Error getting image size", zap.Error(err))
		return nil, err
	}
	previewWidth := imgSize.Width / 2
	previewImage.Transform(image.WithWidth(previewWidth))

	previewData, previewSize, err := previewImage.Encode(ctx, 85)
	if err != nil {
		logger.Error("Error encoding preview", zap.Error(err))
		return nil, err
	}

	previewBytes, err := io.ReadAll(previewData)
	if err != nil {
		logger.Error("Error reading preview data", zap.Error(err))
		return nil, err
	}

	img := &domainModel.Image{
		ID:   bson.NewObjectID(),
		Type: imageType,
	}

	typePrefix := string(imageType)
	idHex := img.ID.Hex()
	img.Storage.OriginalPath = fmt.Sprintf("%s/%s/%s/original.jpg", typePrefix, idHex[:2], idHex)
	img.Storage.PreviewPath = fmt.Sprintf("%s/%s/%s/preview.jpg", typePrefix, idHex[:2], idHex)

	uploader := s3manager.NewUploaderWithClient(i.s3)

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(i.config.S3Bucket),
		Key:         aws.String(img.Storage.OriginalPath),
		Body:        bytes.NewReader(originalBytes),
		ContentType: aws.String("image/jpeg"),
	})
	if err != nil {
		logger.Error("Error uploading original to S3", zap.Error(err))
		return nil, err
	}

	img.Storage.OriginalSize = originalSize

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(i.config.S3Bucket),
		Key:         aws.String(img.Storage.PreviewPath),
		Body:        bytes.NewReader(previewBytes),
		ContentType: aws.String("image/jpeg"),
	})
	if err != nil {
		logger.Error("Error uploading preview to S3", zap.Error(err))
		i.deleteFromS3(i.config.S3Bucket, img.Storage.OriginalPath)
		return nil, err
	}

	img.Storage.PreviewSize = previewSize

	logger.Info("Image processed and stored successfully")
	return img, nil
}

func (i *ImageService) cleanupS3(img *domainModel.Image) {
	if img.Storage.OriginalPath != "" {
		i.deleteFromS3(i.config.S3Bucket, img.Storage.OriginalPath)
	}
	if img.Storage.PreviewPath != "" {
		i.deleteFromS3(i.config.S3Bucket, img.Storage.PreviewPath)
	}
}
