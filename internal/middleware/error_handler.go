package middleware

import (
	"errors"
	"net/http"

	"bom-zustand-api/internal/apperr"

	"github.com/labstack/echo/v5"
)

func ErrorHandler(c *echo.Context, err error) {
	status := http.StatusInternalServerError
	message := "internal server error"

	switch {
	case errors.Is(err, apperr.ErrInvalidCredentials),
		errors.Is(err, apperr.ErrTokenInvalid),
		errors.Is(err, apperr.ErrTokenExpired):
		status, message = http.StatusUnauthorized, err.Error()
	case errors.Is(err, apperr.ErrUserInactive),
		errors.Is(err, apperr.ErrPermissionDenied):
		status, message = http.StatusForbidden, err.Error()
	case errors.Is(err, apperr.ErrUserNotFound),
		errors.Is(err, apperr.ErrTemplateNotFound),
		errors.Is(err, apperr.ErrReportNotFound):
		status, message = http.StatusNotFound, err.Error()
	case errors.Is(err, apperr.ErrEmailAlreadyExists),
		errors.Is(err, apperr.ErrNameAlreadyExists):
		status, message = http.StatusConflict, err.Error()
	case errors.Is(err, apperr.ErrInvalidReportParams),
		errors.Is(err, apperr.ErrTemplateInvalid),
		errors.Is(err, apperr.ErrPipelineInvalid):
		status, message = http.StatusBadRequest, err.Error()
	default:
		var he *echo.HTTPError
		if errors.As(err, &he) {
			status = he.Code
			message = he.Message
		} else {
			c.Logger().Error("unhandled error", "error", err)
		}
	}

	if resp, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && resp.Committed {
		return
	}
	_ = c.JSON(status, map[string]string{"error": message})
}
