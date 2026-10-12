package http

import (
	"app/internal/platform/server/middleware"
	"github.com/labstack/echo/v5"
	"net/http"
)

func (h *handler) me(c *echo.Context) error {
	identity, ok := middleware.Identity(c.Request().Context())
	if !ok {
		return c.JSON(http.StatusUnauthorized, "unauthorized")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "ok",
		"data":    identity,
	})
}
