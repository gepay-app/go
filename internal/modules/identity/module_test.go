package identity

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/platform/server"
)

// Wiring route + auth guard tanpa DB/Redis. Jalur yang menyentuh DB
// (Resolve/Ensure) diuji lewat integrasi (lihat identity.md §9).
func TestMeRequiresAuth(t *testing.T) {
	srv := server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), server.Options{})

	m := New(Dependencies{Logger: slog.Default()})
	m.RegisterRoutes(srv)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/identities/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}
}
