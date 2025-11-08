package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ImageType string

const (
	ImageTypePoster     ImageType = "poster"
	ImageTypeBackground ImageType = "background"
)

type ImageVariant string

const (
	ImageVariantOriginal ImageVariant = "original"
	ImageVariantPreview  ImageVariant = "preview"
)

type Image struct {
	ID       bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Type     ImageType     `bson:"type" json:"type"`
	Source   ImageSource   `bson:"source" json:"source"`
	Storage  ImageStorage  `bson:"storage" json:"storage"`
	Metadata ImageMetadata `bson:"metadata" json:"metadata"`
}

type ImageSource struct {
	Service string `bson:"service" json:"service"`
	Path    string `bson:"path" json:"path"`
}

type ImageStorage struct {
	OriginalPath string `bson:"originalPath" json:"originalPath"`
	PreviewPath  string `bson:"previewPath" json:"previewPath"`
	OriginalSize int64  `bson:"originalSize" json:"originalSize"`
	PreviewSize  int64  `bson:"previewSize" json:"previewSize"`
}

type ImageMetadata struct {
	UploadedAt     time.Time `bson:"uploadedAt" json:"uploadedAt"`
	LastAccessedAt time.Time `bson:"lastAccessedAt" json:"lastAccessedAt"`
	AccessCount    int64     `bson:"accessCount" json:"accessCount"`
}
