// internal/handler/report_handler.go
package handler

import (
	"context"
	"net/http"
	"strconv"

	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type reportServicer interface {
	CreateReport(ctx context.Context, name string, templateID primitive.ObjectID, collection, pipelineTemplate string, paramSchema []model.ReportParam) (*model.ReportDefinition, error)
	ListReports(ctx context.Context, limit, skip int64) ([]*model.ReportDefinition, error)
	GetReport(ctx context.Context, id primitive.ObjectID) (*model.ReportDefinition, error)
	UpdateReport(ctx context.Context, id primitive.ObjectID, name, collection, pipelineTemplate *string, paramSchema []model.ReportParam) (*model.ReportDefinition, error)
	DeleteReport(ctx context.Context, id primitive.ObjectID) error
	PreviewReport(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) (string, error)
	GenerateReportPDF(ctx context.Context, id primitive.ObjectID, params map[string]interface{}) ([]byte, error)
}

type ReportHandler struct {
	service reportServicer
}

func NewReportHandler(svc reportServicer) *ReportHandler {
	return &ReportHandler{service: svc}
}

type createReportRequest struct {
	Name             string              `json:"name" validate:"required"`
	TemplateID       string              `json:"templateId" validate:"required"`
	Collection       string              `json:"collection" validate:"required"`
	PipelineTemplate string              `json:"pipelineTemplate" validate:"required"`
	ParamSchema      []model.ReportParam `json:"paramSchema"`
}

// Create godoc
//
//	@Summary		Create a report definition
//	@Description	Pairs a template with a MongoDB aggregation pipeline and a param schema. Requires the report:create permission. Pipeline placeholders use {{json .paramName}}; wrap date params needing BSON Date comparison as {"$date": {{json .paramName}}}.
//	@Tags			reports
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		createReportRequest	true	"New report definition"
//	@Success		201		{object}	model.ReportDefinition
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		404		{object}	handler.errorResponse
//	@Failure		409		{object}	handler.errorResponse
//	@Router			/reports [post]
func (h *ReportHandler) Create(c *echo.Context) error {
	var req createReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	templateID, err := primitive.ObjectIDFromHex(req.TemplateID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid templateId")
	}

	def, err := h.service.CreateReport(c.Request().Context(), req.Name, templateID, req.Collection, req.PipelineTemplate, req.ParamSchema)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, def)
}

// List godoc
//
//	@Summary		List report definitions
//	@Description	Lists report definitions with pagination, including each one's param schema. Requires the report:read permission.
//	@Tags			reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query		int	false	"Max number of report definitions to return"
//	@Param			skip	query		int	false	"Number of report definitions to skip"
//	@Success		200		{array}		model.ReportDefinition
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Router			/reports [get]
func (h *ReportHandler) List(c *echo.Context) error {
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	skip, _ := strconv.ParseInt(c.QueryParam("skip"), 10, 64)

	reports, err := h.service.ListReports(c.Request().Context(), limit, skip)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, reports)
}

// Get godoc
//
//	@Summary		Get a report definition
//	@Description	Returns a report definition's param schema and template reference — this is the primary lookup the frontend uses to build a generation form. Requires the report:read permission.
//	@Tags			reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Report definition ID"
//	@Success		200	{object}	model.ReportDefinition
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id} [get]
func (h *ReportHandler) Get(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	def, err := h.service.GetReport(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, def)
}

type updateReportRequest struct {
	Name             *string             `json:"name" validate:"omitempty"`
	Collection       *string             `json:"collection" validate:"omitempty"`
	PipelineTemplate *string             `json:"pipelineTemplate" validate:"omitempty"`
	ParamSchema      []model.ReportParam `json:"paramSchema"`
}

// Update godoc
//
//	@Summary		Update a report definition
//	@Description	Partially updates a report definition's name, collection, pipeline, and/or param schema. Requires the report:update permission.
//	@Tags			reports
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"Report definition ID"
//	@Param			body	body		updateReportRequest	true	"Fields to update"
//	@Success		200		{object}	model.ReportDefinition
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		404		{object}	handler.errorResponse
//	@Router			/reports/{id} [patch]
func (h *ReportHandler) Update(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	var req updateReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	def, err := h.service.UpdateReport(c.Request().Context(), id, req.Name, req.Collection, req.PipelineTemplate, req.ParamSchema)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, def)
}

// Delete godoc
//
//	@Summary		Delete a report definition
//	@Description	Deletes a report definition. Requires the report:delete permission.
//	@Tags			reports
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Report definition ID"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id} [delete]
func (h *ReportHandler) Delete(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	if err := h.service.DeleteReport(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type generateReportRequest struct {
	Params map[string]interface{} `json:"params"`
}

// Preview godoc
//
//	@Summary		Preview a report as HTML
//	@Description	Runs the report's query and renders its template, returning raw HTML (no PDF conversion) — used by the frontend to show a live preview before generating. Requires the report:generate permission.
//	@Tags			reports
//	@Accept			json
//	@Produce		html
//	@Security		BearerAuth
//	@Param			id		path	string					true	"Report definition ID"
//	@Param			body	body	generateReportRequest	true	"Report params"
//	@Success		200
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id}/preview [post]
func (h *ReportHandler) Preview(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	var req generateReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	html, err := h.service.PreviewReport(c.Request().Context(), id, req.Params)
	if err != nil {
		return err
	}
	return c.HTML(http.StatusOK, html)
}

// Generate godoc
//
//	@Summary		Generate a report PDF
//	@Description	Runs the report's query, renders its template, and converts the result to PDF via headless Chrome. Requires the report:generate permission.
//	@Tags			reports
//	@Accept			json
//	@Produce		application/pdf
//	@Security		BearerAuth
//	@Param			id		path	string					true	"Report definition ID"
//	@Param			body	body	generateReportRequest	true	"Report params"
//	@Success		200
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/reports/{id}/generate [post]
func (h *ReportHandler) Generate(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report id")
	}
	var req generateReportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	pdfBytes, err := h.service.GenerateReportPDF(c.Request().Context(), id, req.Params)
	if err != nil {
		return err
	}
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}
