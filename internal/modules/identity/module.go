// Package identity adalah modul identity. File ini satu-satunya tempat wiring
// modul: mendeklarasikan dependensinya, menyusun implementasi, dan menyerahkan
// pendaftaran route ke delivery layer.
//
//	./api                                -> kontrak yang boleh dipakai modul lain
//	./internal/service                   -> business logic
//	./internal/repository                -> akses data (sqlc, generated)
//	./internal/cache                     -> cache per agregat
//	./internal/delivery/http             -> handler + route (transport)
//
// Seluruh ./internal terkunci oleh aturan "internal" di Go.
package identity

import (
	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/cache"
	"app/internal/platform/firebase"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"

	httpdelivery "app/internal/modules/identity/internal/delivery/http"
	"app/internal/modules/identity/internal/service"
)

// Dependencies adalah apa yang modul ini butuhkan dari luar. bootstrap yang mengisi.
type Dependencies struct {
	Logger   *slog.Logger
	DB       *pgxpool.Pool
	Redis    *redis.Client
	Verifier firebase.Verifier
	Admin    firebase.UserAdmin
}

// Module memegang dependensi dan service modul.
type Module struct {
	deps Dependencies
	svc  *service.Service
}

func New(deps Dependencies) *Module {
	userCache := cache.NewUserCache(deps.Redis)
	return &Module{
		deps: deps,
		svc:  service.New(deps.DB, userCache, deps.Admin),
	}
}

// RegisterRoutes mendelegasikan pemasangan endpoint ke delivery layer.
func (m *Module) RegisterRoutes(e *echo.Echo) {
	httpdelivery.RegisterRoutes(e, m.deps.Verifier, m.svc)

	m.deps.Logger.Info("identity routes registered")
}

// Users mengekspos kontrak api.UserApi untuk modul lain. Bootstrap yang
// menyuntikkannya, mis. otherModule.New(otherModule.Dependencies{Users: identityModule.Users()}).
func (m *Module) Users() api.UserApi { return m.svc }
