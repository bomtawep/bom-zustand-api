// internal/handler/user_handler.go
package handler

import (
	"context"
	"net/http"
	"strconv"

	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type userServicer interface {
	CreateUser(ctx context.Context, email, name string, role string) (*model.User, error)
	ListUsers(ctx context.Context, limit, skip int64) ([]*model.User, error)
	GetUser(ctx context.Context, id primitive.ObjectID) (*model.User, error)
	UpdateUser(ctx context.Context, id primitive.ObjectID, name, email, role *string) (*model.User, error)
	DeactivateUser(ctx context.Context, id primitive.ObjectID) error
}

type UserHandler struct {
	service userServicer
}

func NewUserHandler(svc userServicer) *UserHandler {
	return &UserHandler{service: svc}
}

func paramObjectID(c *echo.Context) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(c.Param("id"))
}

type createUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required"`
	Role  string `json:"role" validate:"required,oneof=admin manager staff viewer"`
}

// Create godoc
//
//	@Summary		Create a user
//	@Description	Creates a new user account. Requires the user:create permission.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		createUserRequest	true	"New user"
//	@Success		201		{object}	model.User
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		409		{object}	handler.errorResponse
//	@Router			/users [post]
func (h *UserHandler) Create(c *echo.Context) error {
	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	u, err := h.service.CreateUser(c.Request().Context(), req.Email, req.Name, req.Role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, u)
}

// List godoc
//
//	@Summary		List users
//	@Description	Lists users with pagination. Requires the user:read permission.
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			limit	query		int	false	"Max number of users to return"
//	@Param			skip	query		int	false	"Number of users to skip"
//	@Success		200		{array}		model.User
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Router			/users [get]
func (h *UserHandler) List(c *echo.Context) error {
	limit, _ := strconv.ParseInt(c.QueryParam("limit"), 10, 64)
	skip, _ := strconv.ParseInt(c.QueryParam("skip"), 10, 64)

	users, err := h.service.ListUsers(c.Request().Context(), limit, skip)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

// Get godoc
//
//	@Summary		Get a user
//	@Description	Returns a single user by id. Requires the user:read permission.
//	@Tags			users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	model.User
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/users/{id} [get]
func (h *UserHandler) Get(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	u, err := h.service.GetUser(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, u)
}

type updateUserRequest struct {
	Name  *string `json:"name" validate:"omitempty"`
	Email *string `json:"email" validate:"omitempty,email"`
	Role  *string `json:"role" validate:"omitempty,oneof=admin manager staff viewer"`
}

// Update godoc
//
//	@Summary		Update a user
//	@Description	Partially updates a user's name, email, and/or role. Requires the user:update permission.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string				true	"User ID"
//	@Param			body	body		updateUserRequest	true	"Fields to update"
//	@Success		200		{object}	model.User
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Failure		403		{object}	handler.errorResponse
//	@Failure		404		{object}	handler.errorResponse
//	@Router			/users/{id} [patch]
func (h *UserHandler) Update(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	var req updateUserRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	u, err := h.service.UpdateUser(c.Request().Context(), id, req.Name, req.Email, req.Role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, u)
}

// Delete godoc
//
//	@Summary		Deactivate a user
//	@Description	Deactivates a user account. Requires the user:delete permission.
//	@Tags			users
//	@Security		BearerAuth
//	@Param			id	path	string	true	"User ID"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Failure		403	{object}	handler.errorResponse
//	@Failure		404	{object}	handler.errorResponse
//	@Router			/users/{id} [delete]
func (h *UserHandler) Delete(c *echo.Context) error {
	id, err := paramObjectID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	if err := h.service.DeactivateUser(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
