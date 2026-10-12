package middleware

import (
	"app/internal/platform/apperr"
	"app/internal/platform/firebase"
	"app/internal/platform/logging"
	"context"
	"github.com/labstack/echo/v5"
	"strings"
)

type identityKey struct{}

// RequireAuth memverifikasi Bearer ID token lalu menaruh hasilnya di request
// context. Ia membuktikan SIAPA caller-nya, bukan apa yang boleh ia lakukan.
func RequireAuth(v firebase.Verifier) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw := c.Request().Header.Get(echo.HeaderAuthorization)
			token, ok := strings.CutPrefix(raw, "Bearer ")
			if token = strings.TrimSpace(token); !ok || token == "" {
				return apperr.Unauthorized("missing_token", "missing bearer token")
			}

			ctx := c.Request().Context()
			id, err := v.Verify(ctx, token)
			if err != nil {
				// Expired/signature/audience masuk log (ber-request_id), bukan ke client.
				logging.FromContext(ctx).Warn("firebase token verification failed", "error", err)
				return apperr.Unauthorized("invalid_token", "invalid or expired token")
			}

			ctx = context.WithValue(ctx, identityKey{}, id)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// Identity mengambil caller terverifikasi. ok=false kalau RequireAuth belum jalan.
func Identity(ctx context.Context) (firebase.Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(firebase.Identity)
	return id, ok
}
