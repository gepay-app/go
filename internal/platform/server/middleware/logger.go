package middleware

import (
	"errors"
	"log/slog"
	"time"

	"github.com/labstack/echo/v5"

	"app/internal/platform/apperr"
	"app/internal/platform/logging"
)

// ContextLogger stores a request-scoped logger carrying request_id, so any layer
// can log via logging.FromContext(ctx). Must run after RequestID.
func ContextLogger(base *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()
			log := base.With(slog.String("request_id", logging.RequestID(ctx)))
			c.SetRequest(c.Request().WithContext(logging.WithLogger(ctx, log)))
			return next(c)
		}
	}
}

// AccessLog writes one structured line per request, correlated by request_id.
// Place it outside Recover so a recovered panic is reported as a 500 here.
func AccessLog() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)

			status := statusOf(c, err)
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}

			attrs := []slog.Attr{
				slog.String("method", c.Request().Method),
				slog.String("path", c.Request().URL.Path),
				slog.String("route", c.Path()),
				slog.Int("status", status),
				slog.Duration("latency", time.Since(start)),
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				if status >= 500 {
					if cause := errors.Unwrap(err); cause != nil {
						attrs = append(attrs, slog.String("cause", cause.Error()))
					}
				}
			}
			logging.FromContext(c.Request().Context()).LogAttrs(c.Request().Context(), level, "request", attrs...)
			return err
		}
	}
}

// statusOf reports the status the client will receive. apperr kinds carry no
// HTTP status themselves, so map them here (the response is not yet committed).
func statusOf(c *echo.Context, err error) int {
	if res, _ := echo.UnwrapResponse(c.Response()); res == nil || !res.Committed {
		var appErr *apperr.Error
		if errors.As(err, &appErr) {
			return statusFor(appErr.Kind)
		}
	}
	_, status := echo.ResolveResponseStatus(c.Response(), err)
	return status
}
