package http

import (
	"app/internal/platform/apperr"
	"app/internal/platform/server/middleware"
	"github.com/labstack/echo/v5"
	"net/http"
)

func (h *handler) session(c *echo.Context) error {
	id, ok := middleware.Identity(c.Request().Context())
	if !ok {
		return apperr.Unauthorized("unauthorized", "unauthenticated")
	}
	u, err := h.svc.Ensure(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": u})
}

func (h *handler) me(c *echo.Context) error {
	id, ok := middleware.Identity(c.Request().Context())
	if !ok {
		return apperr.Unauthorized("unauthorized", "unauthenticated")
	}
	u, err := h.svc.Resolve(c.Request().Context(), id.UID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"data": u})
}
