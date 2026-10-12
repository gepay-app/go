// Package logging builds the app logger and carries a request id and a
// request-scoped logger through context.Context, so any layer can log with the
// same correlation fields without threading a logger through every signature.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"app/internal/platform/config"
)

// New builds the app logger. Empty level/format auto-select from Environment
// (production => info/json, otherwise debug/text).
func New(cfg *config.App) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel, cfg.Environment)}

	var h slog.Handler = slog.NewTextHandler(os.Stdout, opts)
	if format(cfg.LogFormat, cfg.Environment) == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

func parseLevel(raw, env string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	if strings.EqualFold(env, "production") {
		return slog.LevelInfo
	}
	return slog.LevelDebug
}

func format(raw, env string) string {
	switch v := strings.ToLower(strings.TrimSpace(raw)); v {
	case "json", "text":
		return v
	}
	if strings.EqualFold(env, "production") {
		return "json"
	}
	return "text"
}

type loggerKey struct{}
type requestIDKey struct{}

// WithRequestID stores the request id in ctx. Set by the request-id middleware.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID returns the request id stored in ctx, or "" when absent.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// WithLogger stores a request-scoped logger in ctx. Set by the logger middleware.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, log)
}

// FromContext returns the request-scoped logger, falling back to slog.Default()
// outside a request (workers, jobs, tests).
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}
