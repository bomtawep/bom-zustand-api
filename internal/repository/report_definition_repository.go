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

type ReportDefinitionRepository struct {
	col *mongo.Collection
}

func NewReportDefinitionRepository(database *mongo.Database) *ReportDefinitionRepository {
	return &ReportDefinitionRepository{col: database.Collection("report_definitions")}
}

func (r *ReportDefinitionRepository) Create(ctx context.Context, def *model.ReportDefinition) error {
	_, err := r.col.InsertOne(ctx, def)
	if mongo.IsDuplicateKeyError(err) {
		return apperr.ErrNameAlreadyExists
	}
	return err
}

func (r *ReportDefinitionRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	var def model.ReportDefinition
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&def)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, apperr.ErrReportNotFound
	}
	if err != nil {
		return nil, err
	}
	return &def, nil
}

func (r *ReportDefinitionRepository) List(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	opts := options.Find().SetLimit(limit).SetSkip(skip).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var defs []*model.ReportDefinition
	if err := cursor.All(ctx, &defs); err != nil {
		return nil, err
	}
	return defs, nil
}

func (r *ReportDefinitionRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return apperr.ErrNameAlreadyExists
		}
		return err
	}
	if res.MatchedCount == 0 {
		return apperr.ErrReportNotFound
	}
	return nil
}

func (r *ReportDefinitionRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return apperr.ErrReportNotFound
	}
	return nil
}
