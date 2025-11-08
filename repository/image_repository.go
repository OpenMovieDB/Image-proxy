package repository

import (
	"context"
	"resizer/domain/model"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ImageRepository interface {
	Create(ctx context.Context, image *model.Image) error
	FindByID(ctx context.Context, id bson.ObjectID) (*model.Image, error)
	FindBySource(ctx context.Context, service, path string) (*model.Image, error)
	UpdateAccessTime(ctx context.Context, id bson.ObjectID) error
	FindUnusedImages(ctx context.Context, daysUnused int) ([]*model.Image, error)
	EnsureIndexes(ctx context.Context) error
}
