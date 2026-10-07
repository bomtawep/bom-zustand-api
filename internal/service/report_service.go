package service

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	"bom-zustand-api/internal/apperr"
	"bom-zustand-api/internal/model"
	"bom-zustand-api/internal/pdf"
	"bom-zustand-api/internal/reportquery"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type reportDefinitionRepository interface {
	Create(ctx context.Context, r *model.ReportDefinition) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error)
	List(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type templateLookup interface {
	FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error)
}

type reportDataRunner interface {
	Run(ctx context.Context, collection string, pipeline bson.A) ([]bson.M, error)
}

type ReportService struct {
	reports   reportDefinitionRepository
	templates templateLookup
	data      reportDataRunner
	renderer  pdf.Renderer
}

func NewReportService(reports reportDefinitionRepository, templates templateLookup, data reportDataRunner, renderer pdf.Renderer) *ReportService {
	return &ReportService{reports: reports, templates: templates, data: data, renderer: renderer}
}

func dummyValueForType(t string) interface{} {
	switch t {
	case "number":
		return float64(0)
	case "bool":
		return false
	case "date":
		return time.Now().UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

func validatePipelineTemplate(pipelineTemplate string, schema []model.ReportParam) error {
	dummyParams := make(map[string]interface{}, len(schema))
	for _, p := range schema {
		dummyParams[p.Name] = dummyValueForType(p.Type)
	}
	if _, err := reportquery.BuildPipeline(pipelineTemplate, dummyParams); err != nil {
		return fmt.Errorf("%w: %v", apperr.ErrPipelineInvalid, err)
	}
	return nil
}

func (s *ReportService) CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	if _, err := s.templates.FindByID(ctx, templateID); err != nil {
		return nil, err
	}
	if err := validatePipelineTemplate(pipelineTemplate, paramSchema); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	def := &model.ReportDefinition{
		ID:               primitive.NewObjectID(),
		Name:             name,
		TemplateID:       templateID,
		Collection:       collection,
		PipelineTemplate: pipelineTemplate,
		ParamSchema:      paramSchema,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.reports.Create(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

func (s *ReportService) ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}
	if skip < 0 {
		skip = 0
	}
	return s.reports.List(ctx, limit, skip)
}

func (s *ReportService) GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	return s.reports.FindByID(ctx, id)
}

func (s *ReportService) UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error) {
	existing, err := s.reports.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	update := bson.M{}
	if name != nil {
		update["name"] = *name
	}
	if collection != nil {
		update["collection"] = *collection
	}
	effectiveSchema := existing.ParamSchema
	if paramSchema != nil {
		effectiveSchema = paramSchema
		update["param_schema"] = paramSchema
	}
	if pipelineTemplate != nil {
		if err := validatePipelineTemplate(*pipelineTemplate, effectiveSchema); err != nil {
			return nil, err
		}
		update["pipeline_template"] = *pipelineTemplate
	}
	if len(update) > 0 {
		if err := s.reports.Update(ctx, id, update); err != nil {
			return nil, err
		}
	}
	return s.reports.FindByID(ctx, id)
}

func (s *ReportService) DeleteReport(ctx context.Context, id primitive.ObjectID) error {
	return s.reports.Delete(ctx, id)
}

func (s *ReportService) render(ctx context.Context, reportID primitive.ObjectID, params map[string]interface{}) (string, error) {
	def, err := s.reports.FindByID(ctx, reportID)
	if err != nil {
		return "", err
	}
	if err := reportquery.ValidateParams(def.ParamSchema, params); err != nil {
		return "", fmt.Errorf("%w: %v", apperr.ErrInvalidReportParams, err)
	}
	// Drop any client-supplied key not declared in the report's ParamSchema
	// before it reaches pipeline template execution — otherwise an
	// undeclared key referenced by an authoring mistake in the pipeline
	// template could let a caller who only holds report:generate permission
	// inject arbitrary JSON structure into the aggregation pipeline.
	filteredParams := reportquery.FilterDeclaredParams(def.ParamSchema, params)
	pipeline, err := reportquery.BuildPipeline(def.PipelineTemplate, filteredParams)
	if err != nil {
		return "", fmt.Errorf("report %s has an invalid pipeline: %w", def.ID.Hex(), err)
	}

	tpl, err := s.templates.FindByID(ctx, def.TemplateID)
	if err != nil {
		return "", err
	}

	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	rows, err := s.data.Run(runCtx, def.Collection, pipeline)
	if err != nil {
		return "", fmt.Errorf("run report aggregation: %w", err)
	}

	htmlTpl, err := template.New("report").Parse(tpl.HTMLContent)
	if err != nil {
		return "", fmt.Errorf("report %s has an invalid template: %w", def.ID.Hex(), err)
	}

	var buf bytes.Buffer
	data := struct {
		Rows        []bson.M
		Params      map[string]interface{}
		GeneratedAt time.Time
	}{Rows: rows, Params: params, GeneratedAt: time.Now().UTC()}
	if err := htmlTpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render report template: %w", err)
	}
	return buf.String(), nil
}

func (s *ReportService) PreviewReport(ctx context.Context, reportID primitive.ObjectID, params map[string]interface{}) (string, error) {
	return s.render(ctx, reportID, params)
}

func (s *ReportService) GenerateReportPDF(ctx context.Context, reportID primitive.ObjectID, params map[string]interface{}) ([]byte, error) {
	html, err := s.render(ctx, reportID, params)
	if err != nil {
		return nil, err
	}
	return s.renderer.RenderHTML(ctx, html)
}
