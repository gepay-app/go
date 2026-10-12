package identity

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/platform/server"
)

// Wiring nyata tanpa DB/Redis (keduanya hanya disimpan, tidak diping di sini).
func TestHelloWiring(t *testing.T) {
	srv := server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), server.Options{})

	m := New(Dependencies{Logger: slog.Default()})
	m.RegisterRoutes(srv)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/identity/hello?name=World", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var got struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Message != "hello, World" {
		t.Fatalf("message = %q, want %q", got.Message, "hello, World")
	}
}
