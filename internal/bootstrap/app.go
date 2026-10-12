// Package bootstrap is the composition root: it loads config, builds the logger,
// connects dependencies, starts the HTTP server and releases resources on
// shutdown. Keeping it out of main makes the wiring testable and main trivial.
package bootstrap

import (
	"app/internal/platform/firebase"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"app/internal/modules/identity"
	"app/internal/platform/config"
	"app/internal/platform/logging"
	"app/internal/platform/postgres"
	"app/internal/platform/redis"
	"app/internal/platform/server"
)

const connectTimeout = 10 * time.Second

// Run wires and runs the application until it is shut down, returning the first
// fatal error.
func Run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := logging.New(cfg)
	slog.SetDefault(logger)

	return serve(cfg, logger)
}

func serve(cfg *config.App, logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	pool, err := postgres.New(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()
	logger.Info("postgres connected")

	rdb, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	defer func() { _ = rdb.Close() }()
	logger.Info("redis connected")

	srv := server.New(logger, server.Options{
		CORSOrigins: cfg.CORSOrigins,
		Dependencies: []server.Dependency{
			{Name: "postgres", Check: pool.Ping},
			{Name: "redis", Check: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }},
		},
	})

	// inject verifier di setiap module yang butuh
	verifier, err := firebase.New(ctx, &cfg.Firebase)
	if err != nil {
		return fmt.Errorf("firebase: %w", err)
	}
	logger.Info("firebase auth ready")

	// Wiring modul. Tambah/ganti di sini saat kamu membangun modul asli.
	identityModule := identity.New(identity.Dependencies{
		Logger:   logger,
		DB:       pool,
		Redis:    rdb,
		Verifier: verifier,
	})
	identityModule.RegisterRoutes(srv)

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("http server listening", "addr", addr, "env", cfg.Environment)

	return server.Run(srv, addr)
}

func loadConfig() (*config.App, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return config.Load()
}
