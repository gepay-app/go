// Package http adalah delivery layer modul identity: handler HTTP dan
// pendaftaran route. Ia memanggil business logic (../service), bukan sebaliknya.
package http

import (
	nethttp "net/http"

	"github.com/labstack/echo/v5"

	"app/internal/modules/identity/internal/service"
)

type handler struct{ svc *service.Service }

func newHandler(svc *service.Service) *handler { return &handler{svc: svc} }

func (h *handler) greet(c *echo.Context) error {
	greeting, err := h.svc.Greet(c.Request().Context(), c.QueryParam("name"))
	if err != nil {
		return err // dirender ErrorHandler jadi envelope JSON
	}
	return c.JSON(nethttp.StatusOK, greeting)
}

// RegisterRoutes memasang seluruh endpoint modul identity pada router.
func RegisterRoutes(e *echo.Echo, svc *service.Service) {
	h := newHandler(svc)
	g := e.Group("/identity")
	g.GET("/hello", h.greet)
}
