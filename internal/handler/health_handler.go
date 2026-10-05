// internal/handler/health_handler.go
package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type healthResponse struct {
	Status string `json:"status" example:"ok"`
}

// Healthz godoc
//
//	@Summary		Health check
//	@Description	Returns ok if the service is up.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	handler.healthResponse
//	@Router			/healthz [get]
func Healthz(c *echo.Context) error {
	return c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}
