// internal/handler/template_handler_test.go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeTemplateServicer struct {
	created   *model.Template
	createErr error
	templates []*model.Template
	got       *model.Template
	getErr    error
}

func (f *fakeTemplateServicer) CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error) {
	return f.created, f.createErr
}
func (f *fakeTemplateServicer) ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	return f.templates, nil
}
func (f *fakeTemplateServicer) GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	return f.got, f.getErr
}
func (f *fakeTemplateServicer) UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error) {
	return f.got, f.getErr
}
func (f *fakeTemplateServicer) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	return f.getErr
}

func TestTemplateHandler_Create_Success(t *testing.T) {
	e := newTestEcho()
	svc := &fakeTemplateServicer{created: &model.Template{Name: "invoice"}}
	h := NewTemplateHandler(svc)
	body := strings.NewReader(`{"name":"invoice","description":"d","htmlContent":"<html></html>"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestTemplateHandler_Create_MissingHTMLContentFailsValidation(t *testing.T) {
	e := newTestEcho()
	h := NewTemplateHandler(&fakeTemplateServicer{})
	body := strings.NewReader(`{"name":"invoice"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.Error(t, err)
}

func TestTemplateHandler_Get_NotFoundPropagatesDomainError(t *testing.T) {
	e := newTestEcho()
	svc := &fakeTemplateServicer{getErr: apperr.ErrTemplateNotFound}
	h := NewTemplateHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Get(c)

	assert.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestTemplateHandler_Delete_Success(t *testing.T) {
	e := newTestEcho()
	h := NewTemplateHandler(&fakeTemplateServicer{})
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}
