package service

import (
	"context"
	"fmt"
	"html/template"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type templateRepository interface {
	Create(ctx context.Context, t *model.Template) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error)
	List(ctx context.Context, limit, skip int64) ([]*model.Template, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type TemplateService struct {
	templates templateRepository
}

func NewTemplateService(templates templateRepository) *TemplateService {
	return &TemplateService{templates: templates}
}

func validateTemplateHTML(html string) error {
	if _, err := template.New("validate").Parse(html); err != nil {
		return fmt.Errorf("%w: %v", apperr.ErrTemplateInvalid, err)
	}
	return nil
}

func (s *TemplateService) CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error) {
	if err := validateTemplateHTML(htmlContent); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	t := &model.Template{
		ID:          primitive.NewObjectID(),
		Name:        name,
		Description: description,
		HTMLContent: htmlContent,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.templates.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TemplateService) ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if skip < 0 {
		skip = 0
	}
	return s.templates.List(ctx, limit, skip)
}

func (s *TemplateService) GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	return s.templates.FindByID(ctx, id)
}

func (s *TemplateService) UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error) {
	update := bson.M{}
	if name != nil {
		update["name"] = *name
	}
	if description != nil {
		update["description"] = *description
	}
	if htmlContent != nil {
		if err := validateTemplateHTML(*htmlContent); err != nil {
			return nil, err
		}
		update["html_content"] = *htmlContent
	}
	if len(update) > 0 {
		if err := s.templates.Update(ctx, id, update); err != nil {
			return nil, err
		}
	}
	return s.templates.FindByID(ctx, id)
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, id primitive.ObjectID) error {
	return s.templates.Delete(ctx, id)
}
