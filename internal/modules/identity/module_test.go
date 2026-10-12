package identity

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/platform/firebase"
	"app/internal/platform/server"
)

type fakeVerifier struct{}

func (fakeVerifier) Verify(context.Context, string) (firebase.Identity, error) {
	return firebase.Identity{UID: "uid-1", Email: "user@example.com"}, nil
}

// Wiring nyata tanpa DB/Redis (keduanya hanya disimpan, tidak diping di sini).
func TestMeWiring(t *testing.T) {
	srv := server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), server.Options{})

	m := New(Dependencies{Logger: slog.Default(), Verifier: fakeVerifier{}})
	m.RegisterRoutes(srv)

	// Tanpa token: RequireAuth menolak.
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/identities/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa token: status = %d, want 401 (%s)", rec.Code, rec.Body.String())
	}

	// Dengan token valid: lolos ke handler dan identitas terbawa lewat context.
	req := httptest.NewRequest(http.MethodGet, "/identities/me", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("dengan token: status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	var got struct {
		Data firebase.Identity `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.UID != "uid-1" {
		t.Fatalf("data.UID = %q, want %q", got.Data.UID, "uid-1")
	}
}
