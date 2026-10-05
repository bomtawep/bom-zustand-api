// internal/handler/auth_handler.go
package handler

import (
	"context"
	"errors"
	"net/http"

	"bom-zustand-api/internal/auth"
	"bom-zustand-api/internal/model"

	"github.com/labstack/echo/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type authServicer interface {
	Login(ctx context.Context, email, password, userAgent string) (string, string, error)
	Refresh(ctx context.Context, refreshToken, userAgent string) (string, string, error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutAll(ctx context.Context, userID primitive.ObjectID) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID primitive.ObjectID, oldPassword, newPassword string) error
	Me(ctx context.Context, userID primitive.ObjectID) (*model.User, error)
}

type AuthHandler struct {
	service authServicer
}

func NewAuthHandler(svc authServicer) *AuthHandler {
	return &AuthHandler{service: svc}
}

func contextUserID(c *echo.Context) (primitive.ObjectID, error) {
	raw, ok := c.Get("userID").(string)
	if !ok {
		return primitive.NilObjectID, errors.New("missing user context")
	}
	return primitive.ObjectIDFromHex(raw)
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// Login godoc
//
//	@Summary		Log in
//	@Description	Exchanges email/password credentials for an access and refresh token pair.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		loginRequest	true	"Credentials"
//	@Success		200		{object}	handler.tokenResponse
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Router			/auth/login [post]
func (h *AuthHandler) Login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	access, refresh, err := h.service.Login(c.Request().Context(), req.Email, req.Password, c.Request().UserAgent())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh})
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" validate:"required"`
}

// Refresh godoc
//
//	@Summary		Refresh tokens
//	@Description	Exchanges a valid refresh token for a new access and refresh token pair.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		refreshRequest	true	"Refresh token"
//	@Success		200		{object}	handler.tokenResponse
//	@Failure		400		{object}	handler.errorResponse
//	@Failure		401		{object}	handler.errorResponse
//	@Router			/auth/refresh [post]
func (h *AuthHandler) Refresh(c *echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	access, refresh, err := h.service.Refresh(c.Request().Context(), req.RefreshToken, c.Request().UserAgent())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh})
}

// Logout godoc
//
//	@Summary		Log out
//	@Description	Revokes the given refresh token.
//	@Tags			auth
//	@Accept			json
//	@Param			body	body	refreshRequest	true	"Refresh token"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Router			/auth/logout [post]
func (h *AuthHandler) Logout(c *echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.Logout(c.Request().Context(), req.RefreshToken); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// LogoutAll godoc
//
//	@Summary		Log out of all sessions
//	@Description	Revokes every refresh token issued to the authenticated user.
//	@Tags			auth
//	@Security		BearerAuth
//	@Success		204
//	@Failure		401	{object}	handler.errorResponse
//	@Router			/auth/logout-all [post]
func (h *AuthHandler) LogoutAll(c *echo.Context) error {
	userID, err := contextUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user context")
	}
	if err := h.service.LogoutAll(c.Request().Context(), userID); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type forgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ForgotPassword godoc
//
//	@Summary		Request a password reset
//	@Description	Sends a password reset link to the given email if an account exists for it. Always responds 200 to avoid leaking account existence.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		forgotPasswordRequest	true	"Account email"
//	@Success		200		{object}	handler.messageResponse
//	@Failure		400		{object}	handler.errorResponse
//	@Router			/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *echo.Context) error {
	var req forgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.ForgotPassword(c.Request().Context(), req.Email); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, messageResponse{Message: "if that email exists, a reset link has been sent"})
}

type resetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"newPassword" validate:"required,min=8"`
}

// ResetPassword godoc
//
//	@Summary		Reset password with a token
//	@Description	Sets a new password using the token emailed by the forgot-password flow.
//	@Tags			auth
//	@Accept			json
//	@Param			body	body	resetPasswordRequest	true	"Reset token and new password"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Router			/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *echo.Context) error {
	var req resetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.ResetPassword(c.Request().Context(), req.Token, req.NewPassword); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword" validate:"required"`
	NewPassword string `json:"newPassword" validate:"required,min=8"`
}

// ChangePassword godoc
//
//	@Summary		Change password
//	@Description	Changes the authenticated user's password given their current password.
//	@Tags			auth
//	@Accept			json
//	@Security		BearerAuth
//	@Param			body	body	changePasswordRequest	true	"Old and new password"
//	@Success		204
//	@Failure		400	{object}	handler.errorResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Router			/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *echo.Context) error {
	userID, err := contextUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user context")
	}
	var req changePasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.ChangePassword(c.Request().Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

type meResponse struct {
	*model.User
	Permissions []auth.Permission `json:"permissions"`
}

// Me godoc
//
//	@Summary		Get current user
//	@Description	Returns the authenticated user along with their effective permissions.
//	@Tags			auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	handler.meResponse
//	@Failure		401	{object}	handler.errorResponse
//	@Router			/auth/me [get]
func (h *AuthHandler) Me(c *echo.Context) error {
	userID, err := contextUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid user context")
	}
	u, err := h.service.Me(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, meResponse{
		User:        u,
		Permissions: auth.PermissionsForRole(auth.Role(u.Role)),
	})
}
