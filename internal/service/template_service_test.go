// internal/service/template_service_test.go
package service

import (
	"context"
	"testing"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeTemplateRepo struct {
	byID           map[primitive.ObjectID]*model.Template
	lastUpdateID   primitive.ObjectID
	lastUpdateDoc  bson.M
	deletedID      primitive.ObjectID
	createErr      error
}

func newFakeTemplateRepo() *fakeTemplateRepo {
	return &fakeTemplateRepo{byID: map[primitive.ObjectID]*model.Template{}}
}

func (f *fakeTemplateRepo) Create(ctx context.Context, t *model.Template) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.byID[t.ID] = t
	return nil
}
func (f *fakeTemplateRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, apperr.ErrTemplateNotFound
	}
	return t, nil
}
func (f *fakeTemplateRepo) List(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	var out []*model.Template
	for _, t := range f.byID {
		out = append(out, t)
	}
	return out, nil
}
func (f *fakeTemplateRepo) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	t, ok := f.byID[id]
	if !ok {
		return apperr.ErrTemplateNotFound
	}
	f.lastUpdateID, f.lastUpdateDoc = id, update
	if desc, ok := update["description"].(string); ok {
		t.Description = desc
	}
	if html, ok := update["html_content"].(string); ok {
		t.HTMLContent = html
	}
	return nil
}
func (f *fakeTemplateRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.ErrTemplateNotFound
	}
	f.deletedID = id
	delete(f.byID, id)
	return nil
}

func TestTemplateService_CreateTemplate_Succeeds(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)

	tmpl, err := svc.CreateTemplate(context.Background(), "invoice", "an invoice", "<html><body>{{.Rows}}</body></html>")

	require.NoError(t, err)
	assert.Equal(t, "invoice", tmpl.Name)
	assert.Contains(t, repo.byID, tmpl.ID)
}

func TestTemplateService_CreateTemplate_RejectsInvalidHTMLTemplateSyntax(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)

	_, err := svc.CreateTemplate(context.Background(), "broken", "", "<html>{{.Unterminated</html>")

	require.ErrorIs(t, err, apperr.ErrTemplateInvalid)
}

func TestTemplateService_UpdateTemplate_RejectsInvalidHTMLTemplateSyntax(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)
	tmpl, err := svc.CreateTemplate(context.Background(), "invoice", "", "<html></html>")
	require.NoError(t, err)

	badHTML := "<html>{{.Unterminated</html>"
	_, err = svc.UpdateTemplate(context.Background(), tmpl.ID, nil, nil, &badHTML)

	require.ErrorIs(t, err, apperr.ErrTemplateInvalid)
}

func TestTemplateService_DeleteTemplate_RemovesIt(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)
	tmpl, err := svc.CreateTemplate(context.Background(), "invoice", "", "<html></html>")
	require.NoError(t, err)

	require.NoError(t, svc.DeleteTemplate(context.Background(), tmpl.ID))

	assert.Equal(t, tmpl.ID, repo.deletedID)
}

func TestTemplateService_GetTemplate_NotFoundPropagatesDomainError(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := NewTemplateService(repo)

	_, err := svc.GetTemplate(context.Background(), primitive.NewObjectID())

	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}
