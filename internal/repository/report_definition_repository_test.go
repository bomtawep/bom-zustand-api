//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newTestReportDefinition(name string) *model.ReportDefinition {
	now := time.Now().UTC()
	return &model.ReportDefinition{
		ID:               primitive.NewObjectID(),
		Name:             name,
		TemplateID:       primitive.NewObjectID(),
		Collection:       "orders",
		PipelineTemplate: `[{"$match": {"status": {{json .status}}}}]`,
		ParamSchema: []model.ReportParam{
			{Name: "status", Type: "string", Required: true, Label: "Status"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestReportDefinitionRepository_CreateAndFindByID(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	def := newTestReportDefinition("orders-by-status")

	require.NoError(t, repo.Create(ctx, def))

	found, err := repo.FindByID(ctx, def.ID)
	require.NoError(t, err)
	assert.Equal(t, def.Name, found.Name)
	assert.Equal(t, def.ParamSchema, found.ParamSchema)
}

func TestReportDefinitionRepository_CreateDuplicateNameFails(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestReportDefinition("dup-report")))

	err := repo.Create(ctx, newTestReportDefinition("dup-report"))

	require.ErrorIs(t, err, apperr.ErrNameAlreadyExists)
}

func TestReportDefinitionRepository_FindByIDNotFoundReturnsDomainError(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))

	_, err := repo.FindByID(context.Background(), primitive.NewObjectID())

	require.ErrorIs(t, err, apperr.ErrReportNotFound)
}

func TestReportDefinitionRepository_UpdateChangesFields(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	def := newTestReportDefinition("update-report")
	require.NoError(t, repo.Create(ctx, def))

	require.NoError(t, repo.Update(ctx, def.ID, bson.M{"collection": "shipments"}))

	found, err := repo.FindByID(ctx, def.ID)
	require.NoError(t, err)
	assert.Equal(t, "shipments", found.Collection)
}

func TestReportDefinitionRepository_DeleteRemovesDocument(t *testing.T) {
	repo := NewReportDefinitionRepository(newTestDatabase(t))
	ctx := context.Background()
	def := newTestReportDefinition("delete-report")
	require.NoError(t, repo.Create(ctx, def))

	require.NoError(t, repo.Delete(ctx, def.ID))

	_, err := repo.FindByID(ctx, def.ID)
	require.ErrorIs(t, err, apperr.ErrReportNotFound)
}
