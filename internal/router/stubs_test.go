// internal/router/stubs_test.go
package router

import (
	"context"

	"bom-zustand-api/internal/model"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type stubAuthServicer struct{}

func (s *stubAuthServicer) Login(ctx context.Context, email, password, userAgent string) (string, string, error) {
	return "access", "refresh", nil
}
func (s *stubAuthServicer) Refresh(ctx context.Context, refreshToken, userAgent string) (string, string, error) {
	return "access", "refresh", nil
}
func (s *stubAuthServicer) Logout(ctx context.Context, refreshToken string) error { return nil }
func (s *stubAuthServicer) LogoutAll(ctx context.Context, userID primitive.ObjectID) error {
	return nil
}
func (s *stubAuthServicer) ForgotPassword(ctx context.Context, email string) error { return nil }
func (s *stubAuthServicer) ResetPassword(ctx context.Context, token, newPassword string) error {
	return nil
}
func (s *stubAuthServicer) ChangePassword(ctx context.Context, userID primitive.ObjectID, oldPassword, newPassword string) error {
	return nil
}
func (s *stubAuthServicer) Me(ctx context.Context, userID primitive.ObjectID) (*model.User, error) {
	return &model.User{ID: userID}, nil
}

type stubUserServicer struct{}

func (s *stubUserServicer) CreateUser(ctx context.Context, email, name, role string) (*model.User, error) {
	return &model.User{}, nil
}
func (s *stubUserServicer) ListUsers(ctx context.Context, limit, skip int64) ([]*model.User, error) {
	return nil, nil
}
func (s *stubUserServicer) GetUser(ctx context.Context, id primitive.ObjectID) (*model.User, error) {
	return &model.User{}, nil
}
func (s *stubUserServicer) UpdateUser(ctx context.Context, id primitive.ObjectID, name, email, role *string) (*model.User, error) {
	return &model.User{}, nil
}
func (s *stubUserServicer) DeactivateUser(ctx context.Context, id primitive.ObjectID) error {
	return nil
}

type stubTemplateServicer struct{}

func (s *stubTemplateServicer) CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error) {
	return &model.Template{}, nil
}
func (s *stubTemplateServicer) ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	return nil, nil
}
func (s *stubTemplateServicer) GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	return &model.Template{}, nil
}
func (s *stubTemplateServicer) UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error) {
	return &model.Template{}, nil
}
func (s *stubTemplateServicer) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	return nil
}

type stubReportServicer struct{}

func (s *stubReportServicer) CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return &model.ReportDefinition{}, nil
}
func (s *stubReportServicer) ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	return nil, nil
}
func (s *stubReportServicer) GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	return &model.ReportDefinition{}, nil
}
func (s *stubReportServicer) UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	return &model.ReportDefinition{}, nil
}
func (s *stubReportServicer) DeleteReport(ctx context.Context, id primitive.ObjectID) error {
	return nil
}
func (s *stubReportServicer) PreviewReport(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) (string, error) {
	return "<html></html>", nil
}
func (s *stubReportServicer) GenerateReportPDF(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) ([]byte, error) {
	return []byte("%PDF-fake"), nil
}
