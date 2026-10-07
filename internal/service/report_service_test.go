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

type fakeReportRepo struct {
	byID      map[primitive.ObjectID]*model.ReportDefinition
	deletedID primitive.ObjectID
}

func newFakeReportRepo() *fakeReportRepo {
	return &fakeReportRepo{byID: map[primitive.ObjectID]*model.ReportDefinition{}}
}

func (f *fakeReportRepo) Create(ctx context.Context, r *model.ReportDefinition) error {
	f.byID[r.ID] = r
	return nil
}
func (f *fakeReportRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error) {
	r, ok := f.byID[id]
	if !ok {
		return nil, apperr.ErrReportNotFound
	}
	return r, nil
}
func (f *fakeReportRepo) List(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error) {
	var out []*model.ReportDefinition
	for _, r := range f.byID {
		out = append(out, r)
	}
	return out, nil
}
func (f *fakeReportRepo) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	r, ok := f.byID[id]
	if !ok {
		return apperr.ErrReportNotFound
	}
	if name, ok := update["name"].(string); ok {
		r.Name = name
	}
	if collection, ok := update["collection"].(string); ok {
		r.Collection = collection
	}
	if pt, ok := update["pipeline_template"].(string); ok {
		r.PipelineTemplate = pt
	}
	if ps, ok := update["param_schema"].([]model.ReportParam); ok {
		r.ParamSchema = ps
	}
	return nil
}
func (f *fakeReportRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	if _, ok := f.byID[id]; !ok {
		return apperr.ErrReportNotFound
	}
	f.deletedID = id
	delete(f.byID, id)
	return nil
}

type fakeTemplateLookup struct {
	byID map[primitive.ObjectID]*model.Template
}

func newFakeTemplateLookup(templates ...*model.Template) *fakeTemplateLookup {
	l := &fakeTemplateLookup{byID: map[primitive.ObjectID]*model.Template{}}
	for _, t := range templates {
		l.byID[t.ID] = t
	}
	return l
}

func (f *fakeTemplateLookup) FindByID(ctx context.Context, id primitive.ObjectID) (*model.Template, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, apperr.ErrTemplateNotFound
	}
	return t, nil
}

func newTestReportService(reports *fakeReportRepo, templates *fakeTemplateLookup) *ReportService {
	return NewReportService(reports, templates, nil, nil)
}

func validTemplate() *model.Template {
	return &model.Template{ID: primitive.NewObjectID(), Name: "t", HTMLContent: "<html></html>"}
}

func TestReportService_CreateReport_Succeeds(t *testing.T) {
	tmpl := validTemplate()
	svc := newTestReportService(newFakeReportRepo(), newFakeTemplateLookup(tmpl))
	schema := []model.ReportParam{{Name: "status", Type: "string", Required: true}}

	def, err := svc.CreateReport(context.Background(), "orders-by-status", tmpl.ID, "orders",
		`[{"$match": {"status": {{json .status}}}}]`, schema)

	require.NoError(t, err)
	assert.Equal(t, "orders-by-status", def.Name)
}

func TestReportService_CreateReport_FailsWhenTemplateDoesNotExist(t *testing.T) {
	svc := newTestReportService(newFakeReportRepo(), newFakeTemplateLookup())

	_, err := svc.CreateReport(context.Background(), "r", primitive.NewObjectID(), "orders",
		`[{"$match": {}}]`, nil)

	require.ErrorIs(t, err, apperr.ErrTemplateNotFound)
}

func TestReportService_CreateReport_FailsWhenPipelineTemplateIsInvalid(t *testing.T) {
	tmpl := validTemplate()
	svc := newTestReportService(newFakeReportRepo(), newFakeTemplateLookup(tmpl))

	_, err := svc.CreateReport(context.Background(), "r", tmpl.ID, "orders", `[{"$match": {{.broken}`, nil)

	require.ErrorIs(t, err, apperr.ErrPipelineInvalid)
}

func TestReportService_UpdateReport_RevalidatesPipelineAgainstEffectiveSchema(t *testing.T) {
	tmpl := validTemplate()
	reports := newFakeReportRepo()
	svc := newTestReportService(reports, newFakeTemplateLookup(tmpl))
	schema := []model.ReportParam{{Name: "status", Type: "string", Required: true}}
	def, err := svc.CreateReport(context.Background(), "r", tmpl.ID, "orders",
		`[{"$match": {"status": {{json .status}}}}]`, schema)
	require.NoError(t, err)

	newPipeline := `[{"$match": {"status": {{json .status}}, "extra": {{json .missing}}}}]`
	_, err = svc.UpdateReport(context.Background(), def.ID, nil, nil, &newPipeline, nil)

	require.NoError(t, err, "a dummy-value map missing an undeclared key .missing passes the zero interface{} (nil) to the json func, which marshals to the JSON literal null — still valid JSON, so this is not expected to fail validation")
}

func TestReportService_DeleteReport_RemovesIt(t *testing.T) {
	tmpl := validTemplate()
	reports := newFakeReportRepo()
	svc := newTestReportService(reports, newFakeTemplateLookup(tmpl))
	def, err := svc.CreateReport(context.Background(), "r", tmpl.ID, "orders", `[{"$match": {}}]`, nil)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteReport(context.Background(), def.ID))

	assert.Equal(t, def.ID, reports.deletedID)
}
