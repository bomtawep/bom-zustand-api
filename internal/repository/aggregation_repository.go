package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AggregationRepository runs ad-hoc aggregation pipelines against a named
// collection. It exists so ReportService can execute admin-authored report
// queries without any service/handler code touching *mongo.Database
// directly.
type AggregationRepository struct {
	database *mongo.Database
}

func NewAggregationRepository(database *mongo.Database) *AggregationRepository {
	return &AggregationRepository{database: database}
}

func (r *AggregationRepository) Run(ctx context.Context, collection string, pipeline bson.A) ([]bson.M, error) {
	cursor, err := r.database.Collection(collection).Aggregate(ctx, pipeline, options.Aggregate().SetBatchSize(500))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []bson.M
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
