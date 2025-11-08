package mongo

import (
	"context"
	"time"

	"resizer/domain/model"
	"resizer/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type imageRepository struct {
	collection *mongo.Collection
}

func NewImageRepository(db *mongo.Database) repository.ImageRepository {
	return &imageRepository{
		collection: db.Collection("images"),
	}
}

func (r *imageRepository) Create(ctx context.Context, image *model.Image) error {
	if image.ID.IsZero() {
		image.ID = bson.NewObjectID()
	}
	image.Metadata.UploadedAt = time.Now()
	image.Metadata.LastAccessedAt = time.Now()
	image.Metadata.AccessCount = 0

	_, err := r.collection.InsertOne(ctx, image)
	return err
}

func (r *imageRepository) FindByID(ctx context.Context, id bson.ObjectID) (*model.Image, error) {
	var image model.Image
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&image)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

func (r *imageRepository) FindBySource(ctx context.Context, service, path string) (*model.Image, error) {
	var image model.Image
	filter := bson.M{
		"source.service": service,
		"source.path":    path,
	}
	err := r.collection.FindOne(ctx, filter).Decode(&image)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

func (r *imageRepository) UpdateAccessTime(ctx context.Context, id bson.ObjectID) error {
	update := bson.M{
		"$set": bson.M{
			"metadata.lastAccessedAt": time.Now(),
		},
		"$inc": bson.M{
			"metadata.accessCount": 1,
		},
	}
	_, err := r.collection.UpdateByID(ctx, id, update)
	return err
}

func (r *imageRepository) FindUnusedImages(ctx context.Context, daysUnused int) ([]*model.Image, error) {
	threshold := time.Now().AddDate(0, 0, -daysUnused)
	filter := bson.M{
		"metadata.lastAccessedAt": bson.M{"$lt": threshold},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var images []*model.Image
	if err = cursor.All(ctx, &images); err != nil {
		return nil, err
	}

	return images, nil
}

func (r *imageRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "source.service", Value: 1}, {Key: "source.path", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "type", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "metadata.lastAccessedAt", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
