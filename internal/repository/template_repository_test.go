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

func newTestTemplate(name string) *model.Template {
	now := time.Now().UTC()
	return &model.Template{
		ID:          primitive.NewObjectID(),
		Name:        name,
		Description: "a test template",
		HTMLContent: "<html><body>{{.Rows}}</body></html>",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestTemplateRepository_CreateAndFindByID(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	tmpl := newTestTemplate("invoice-template")

	require.NoError(t, repo.Create(ctx, tmpl))

	found, err := repo.FindByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, tmpl.Name, found.Name)
}

func TestTemplateRepository_CreateDuplicateNameFails(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestTemplate("dup-template")))

	err := repo.Create(ctx, newTestTemplate("dup-template"))

	require.ErrorIs(t, err, apperr.ErrNameAlreadyExists)
}

func TestTemplateRepository_FindByIDNotFoundReturnsDomainError(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))

	_, err := repo.FindByID(context.Background(), primitive.NewObjectID())

	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestTemplateRepository_UpdateChangesFields(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	tmpl := newTestTemplate("update-template")
	require.NoError(t, repo.Create(ctx, tmpl))

	require.NoError(t, repo.Update(ctx, tmpl.ID, bson.M{"description": "updated"}))

	found, err := repo.FindByID(ctx, tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated", found.Description)
}

func TestTemplateRepository_DeleteRemovesDocument(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	tmpl := newTestTemplate("delete-template")
	require.NoError(t, repo.Create(ctx, tmpl))

	require.NoError(t, repo.Delete(ctx, tmpl.ID))

	_, err := repo.FindByID(ctx, tmpl.ID)
	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestTemplateRepository_List(t *testing.T) {
	repo := NewTemplateRepository(newTestDatabase(t))
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, newTestTemplate("list-template-1")))
	require.NoError(t, repo.Create(ctx, newTestTemplate("list-template-2")))

	found, err := repo.List(ctx, 10, 0)

	require.NoError(t, err)
	assert.Len(t, found, 2)
}
