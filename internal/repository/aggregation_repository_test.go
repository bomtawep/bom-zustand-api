//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestAggregationRepository_RunExecutesPipelineAgainstNamedCollection(t *testing.T) {
	database := newTestDatabase(t)
	ctx := context.Background()
	_, err := database.Collection("orders").InsertMany(ctx, []interface{}{
		bson.M{"status": "paid", "amount": 10},
		bson.M{"status": "paid", "amount": 20},
		bson.M{"status": "pending", "amount": 5},
	})
	require.NoError(t, err)

	repo := NewAggregationRepository(database)
	pipeline := bson.A{
		bson.M{"$match": bson.M{"status": "paid"}},
		bson.M{"$group": bson.M{"_id": "$status", "total": bson.M{"$sum": "$amount"}}},
	}

	rows, err := repo.Run(ctx, "orders", pipeline)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "paid", rows[0]["_id"])
	assert.EqualValues(t, 30, rows[0]["total"])
}
