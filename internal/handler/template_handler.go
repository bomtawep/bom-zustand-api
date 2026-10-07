// internal/handler/template_handler.go
package handler

import (
	"context"
	"net/http"
	"strconv"

	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type templateServicer interface {
	CreateTemplate(ctx context.Context, name, description, htmlContent string) (*model.Template, error)
	ListTemplates(ctx context.Context, limit, skip int64) ([]*model.Template, error)
	GetTemplate(ctx context.Context, id primitive.ObjectID) (*model.Template, error)
	UpdateTemplate(ctx context.Context, id primitive.ObjectID, name, description, htmlContent *string) (*model.Template, error)
	DeleteTemplate(ctx context.Context, id primitive.ObjectID) error
}

type TemplateHandler struct {
	service templateServicer
}

func NewTemplateHandler(svc templateServicer) *TemplateHandler {
	return &TemplateHandler{service: svc}
}

type createTemplateRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description"`
	HTMLContent string `json:"htmlContent" validate:"required"`
}

// Create godoc
//
//	@Summary		Create a template
//	@Description	Creates an HTML report template. Requires the template:create permission.
//	@Tags			templates
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		createTemplateRequest	true	"New template"
//	@Success		201		{object}	model.Template
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		409		{object}	handler.errorResponse
//	@Router			/templates [post]
func (h *TemplateHandler) Create(c *echo.Context) error {
	var req createTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	t, err := h.service.CreateTemplate(c.Request().Context(), req.Name, req.Description, req.HTMLContent)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, t)
}

// List godoc
//
//	@Summary		List templates
//	@Description	Lists templates with pagination. Requires the template:read permission.
//	@Tags			templates
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query		int	false	"Max number of templates to return"
//	@Param			skip	query		int	false	"Number of templates to skip"
//	@Success		200		{array}		model.Template
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Router			/templates [get]
func (h *TemplateHandler) List(c *echo.Context) error {
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	skip, _ := strconv.ParseInt(c.QueryParam("skip"), 10, 64)

	templates, err := h.service.ListTemplates(c.Request().Context(), limit, skip)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, templates)
}

// Get godoc
//
//	@Summary		Get a template
//	@Description	Returns a single template, including its raw HTML content. Requires the template:read permission.
//	@Tags			templates
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Template ID"
//	@Success		200	{object}	model.Template
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/templates/{id} [get]
func (h *TemplateHandler) Get(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	t, err := h.service.GetTemplate(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, t)
}

type updateTemplateRequest struct {
	Name        *string `json:"name" validate:"omitempty"`
	Description *string `json:"description" validate:"omitempty"`
	HTMLContent *string `json:"htmlContent" validate:"omitempty"`
}

// Update godoc
//
//	@Summary		Update a template
//	@Description	Partially updates a template's name, description, and/or HTML content. Requires the template:update permission.
//	@Tags			templates
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string					true	"Template ID"
//	@Param			body	body		updateTemplateRequest	true	"Fields to update"
//	@Success		200		{object}	model.Template
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		404		{object}	handler.errorResponse
//	@Router			/templates/{id} [patch]
func (h *TemplateHandler) Update(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	var req updateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	t, err := h.service.UpdateTemplate(c.Request().Context(), id, req.Name, req.Description, req.HTMLContent)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, t)
}

// Delete godoc
//
//	@Summary		Delete a template
//	@Description	Deletes a template. Requires the template:delete permission.
//	@Tags			templates
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Template ID"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/templates/{id} [delete]
func (h *TemplateHandler) Delete(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template id")
	}
	if err := h.service.DeleteTemplate(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
