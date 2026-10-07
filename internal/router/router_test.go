// internal/router/router_test.go
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bom-zustand-api/internal/auth"
	"bom-zustand-api/internal/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRouter_Healthz_NoAuthRequired(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRouter_CreateUser_WithoutTokenReturns401(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRouter_CreateUser_WithStaffTokenReturns403(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	token, err := auth.GenerateAccessToken("user-1", "staff", "secret", time.Minute)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRouter_Me_WithValidTokenReturns200(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	token, err := auth.GenerateAccessToken("507f1f77bcf86cd799439011", "admin", "secret", time.Minute)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRouter_CreateTemplate_WithStaffTokenReturns403(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	token, err := auth.GenerateAccessToken("user-1", "staff", "secret", time.Minute)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/templates", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRouter_GenerateReport_WithStaffTokenReturns200(t *testing.T) {
	e := New("secret", handler.NewAuthHandler(&stubAuthServicer{}), handler.NewUserHandler(&stubUserServicer{}),
		handler.NewTemplateHandler(&stubTemplateServicer{}), handler.NewReportHandler(&stubReportServicer{}))
	token, err := auth.GenerateAccessToken("user-1", "staff", "secret", time.Minute)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/"+primitive.NewObjectID().Hex()+"/generate", strings.NewReader(`{"params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
