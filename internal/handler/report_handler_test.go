// internal/handler/report_handler_test.go
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

type fakeReportServicer struct {
	created      *model.ReportDefinition
	createErr    error
	got          *model.ReportDefinition
	getErr       error
	previewHTML  string
	previewErr   error
	generatedPDF []byte
	generateErr  error
}

func (f *fakeReportServicer) CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return f.created, f.createErr
}
func (f *fakeReportServicer) ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	return nil, nil
}
func (f *fakeReportServicer) GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	return f.got, f.getErr
}
func (f *fakeReportServicer) UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return f.got, f.getErr
}
func (f *fakeReportServicer) DeleteReport(ctx context.Context, id primitive.ObjectID) error {
	return f.getErr
}
func (f *fakeReportServicer) PreviewReport(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) (string, error) {
	return f.previewHTML, f.previewErr
}
func (f *fakeReportServicer) GenerateReportPDF(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) ([]byte, error) {
	return f.generatedPDF, f.generateErr
}

func TestReportHandler_Create_Success(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{created: &model.ReportDefinition{Name: "orders-by-status"}}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"name":"orders-by-status","templateId":"` + primitive.NewObjectID().Hex() +
		`","collection":"orders","pipelineTemplate":"[{\"$match\":{}}]"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestReportHandler_Create_InvalidTemplateIdFails(t *testing.T) {
	e := newTestEcho()
	h := NewReportHandler(&fakeReportServicer{})
	body := strings.NewReader(`{"name":"r","templateId":"not-an-objectid","collection":"orders","pipelineTemplate":"[]"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.Create(c)

	assert.Error(t, err)
}

func TestReportHandler_Preview_ReturnsHTML(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{previewHTML: "<html>ok</html>"}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"params":{"status":"paid"}}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Preview(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html>ok</html>")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
}

func TestReportHandler_Generate_ReturnsPDFBytes(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{generatedPDF: []byte("%PDF-fake")}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"params":{"status":"paid"}}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Generate(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, []byte("%PDF-fake"), rec.Body.Bytes())
}

func TestReportHandler_Generate_InvalidParamsPropagatesDomainError(t *testing.T) {
	e := newTestEcho()
	svc := &fakeReportServicer{generateErr: apperr.ErrInvalidReportParams}
	h := NewReportHandler(svc)
	body := strings.NewReader(`{"params":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "id", Value: primitive.NewObjectID().Hex()}})

	err := h.Generate(c)

	assert.ErrorIs(t, err, apperr.ErrInvalidReportParams)
}
