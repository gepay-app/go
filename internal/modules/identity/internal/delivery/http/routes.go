// Package http adalah delivery layer modul identity: handler HTTP dan
// pendaftaran route. Ia memanggil business logic (../service), bukan sebaliknya.
package http

import (
	"app/internal/platform/firebase"
	"app/internal/platform/server/middleware"
	"github.com/labstack/echo/v5"

	"app/internal/modules/identity/internal/service"
)

type handler struct{ svc *service.Service }

func newHandler(svc *service.Service) *handler { return &handler{svc: svc} }

// RegisterRoutes memasang seluruh endpoint modul identity pada router.
func RegisterRoutes(e *echo.Echo, verifier firebase.Verifier, svc *service.Service) {
	h := newHandler(svc)
	requireAuth := middleware.RequireAuth(verifier)
	g := e.Group("/identities", requireAuth)
	g.GET("/me", h.me)
}
