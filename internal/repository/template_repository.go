package repository

import (
	"context"
	"errors"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TemplateRepository struct {
	col *mongo.Collection
}

func NewTemplateRepository(database *mongo.Database) *TemplateRepository {
	return &TemplateRepository{col: database.Collection("templates")}
}

func (r *TemplateRepository) Create(ctx context.Context, t *model.Template) error {
	_, err := r.col.InsertOne(ctx, t)
	if mongo.IsDuplicateKeyError(err) {
		return apperr.ErrNameAlreadyExists
	}
	return err
}

func (r *TemplateRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	var t model.Template
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TemplateRepository) List(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	opts := options.Find().SetLimit(limit).SetSkip(skip).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var templates []*model.Template
	if err := cursor.All(ctx, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

func (r *TemplateRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperr.ErrNameAlreadyExists
		}
		return err
	}
	if res.MatchedCount == 0 {
		return apperr.ErrTemplateNotFound
	}
	return nil
}

func (r *TemplateRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apperr.ErrTemplateNotFound
	}
	return nil
}
