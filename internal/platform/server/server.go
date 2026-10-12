package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	srvmw "app/internal/platform/server/middleware"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
	gracefulTimeout   = 10 * time.Second

	requestTimeout = 15 * time.Second
	bodyLimitBytes = 1 << 20 // 1 MiB
	readyTimeout   = 2 * time.Second
)

// Dependency is a named readiness check, e.g. Postgres or Redis.
type Dependency struct {
	Name  string
	Check func(ctx context.Context) error
}

// Options configures the HTTP server.
type Options struct {
	// CORSOrigins, when non-empty, enables CORS for those origins.
	CORSOrigins []string
	// Dependencies are checked by /readyz.
	Dependencies []Dependency
}

// New builds the Echo instance with logging, tracing, recovery and the baseline
// middleware. Middleware order, outer to inner:
// BodyLimit/CORS (pre) -> requestID -> contextLogger -> accessLog -> contextTimeout -> recover.
func New(base *slog.Logger, opts Options) *echo.Echo {
	if base == nil {
		base = slog.Default()
	}

	e := echo.NewWithConfig(echo.Config{
		Logger: base,
		Binder: jsonBinder{},
	})
	e.HTTPErrorHandler = srvmw.ErrorHandler()

	// Pre: run before routing, so oversized bodies are rejected early and CORS
	// preflight (OPTIONS) is answered without a matching route.
	pre := []echo.MiddlewareFunc{middleware.BodyLimit(bodyLimitBytes)}
	if len(opts.CORSOrigins) > 0 {
		pre = append(pre, middleware.CORS(opts.CORSOrigins...))
	}
	e.Pre(pre...)

	e.Use(
		srvmw.RequestID(),
		srvmw.ContextLogger(base),
		srvmw.AccessLog(),
		middleware.ContextTimeout(requestTimeout),
		srvmw.Recover(),
	)

	e.GET("/healthz", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})
	e.GET("/readyz", readiness(opts.Dependencies))

	return e
}

// readiness reports 200 only when every dependency is reachable, else 503.
func readiness(deps []Dependency) echo.HandlerFunc {
	return func(c *echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), readyTimeout)
		defer cancel()

		status := http.StatusOK
		overall := "ok"
		checks := make(map[string]string, len(deps))
		for _, d := range deps {
			if err := d.Check(ctx); err != nil {
				status, overall = http.StatusServiceUnavailable, "unavailable"
				checks[d.Name] = "down"
				continue
			}
			checks[d.Name] = "ok"
		}
		return c.JSON(status, map[string]any{"status": overall, "checks": checks})
	}
}

// Run serves HTTP on addr until SIGINT/SIGTERM, then drains in-flight requests
// within gracefulTimeout. Returns after the server has stopped.
func Run(e *echo.Echo, addr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return echo.StartConfig{
		Address:         addr,
		GracefulTimeout: gracefulTimeout,
		BeforeServeFunc: func(s *http.Server) error {
			s.ReadHeaderTimeout = readHeaderTimeout
			s.ReadTimeout = readTimeout
			s.WriteTimeout = writeTimeout
			s.IdleTimeout = idleTimeout
			return nil
		},
	}.Start(ctx, e)
}
