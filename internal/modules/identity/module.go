// Package identity adalah modul identity. File ini satu-satunya tempat wiring
// modul: mendeklarasikan dependensinya, menyusun implementasi, dan menyerahkan
// pendaftaran route ke delivery layer.
//
//	./api                                -> kontrak yang boleh dipakai modul lain
//	./internal/service                   -> business logic
//	./internal/repository                -> akses data (belum diisi)
//	./internal/delivery/http             -> handler + route (transport)
//
// Seluruh ./internal terkunci oleh aturan "internal" di Go.
package identity

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"

	"app/internal/modules/identity/api"
	httpdelivery "app/internal/modules/identity/internal/delivery/http"
	"app/internal/modules/identity/internal/service"
)

// Dependencies adalah apa yang modul ini butuhkan dari luar. bootstrap yang mengisi.
type Dependencies struct {
	Logger *slog.Logger
	DB     *pgxpool.Pool
	Redis  *redis.Client
	// Modul lain lewat kontraknya, contoh: Users someOtherModuleapi.UserReader
}

// Module memegang dependensi dan service modul.
type Module struct {
	deps Dependencies
	svc  *service.Service
}

func New(deps Dependencies) *Module {
	return &Module{
		deps: deps,
		svc:  service.New(deps.DB, deps.Redis),
	}
}

// RegisterRoutes mendelegasikan pemasangan endpoint ke delivery layer.
func (m *Module) RegisterRoutes(e *echo.Echo) {
	httpdelivery.RegisterRoutes(e, m.svc)

	m.deps.Logger.Info("identity routes registered")
}

// Greeter mengembalikan kontrak modul ini untuk dipakai modul lain (disuntikkan
// lewat bootstrap). Modul lain tidak pernah melihat ./internal.
func (m *Module) Greeter() api.Greeter { return m.svc }
