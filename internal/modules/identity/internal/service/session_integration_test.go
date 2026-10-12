package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"uuid"

	"app/internal/modules/identity/api"
	"app/internal/modules/identity/internal/cache"
	"app/internal/platform/firebase"
)

// newIntegrationService butuh Postgres + Redis nyata (skema sudah dimigrasi).
// Skip kalau env tidak diisi, supaya `make test` tetap hijau tanpa dependency.
func newIntegrationService(t *testing.T) (*Service, *pgxpool.Pool) {
	t.Helper()
	dbURL := os.Getenv("TEST_DB_URL")
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if dbURL == "" || redisAddr == "" {
		t.Skip("TEST_DB_URL / TEST_REDIS_ADDR not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres unavailable: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		pool.Close()
		_ = rdb.Close()
		t.Skipf("redis unavailable: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_ = rdb.Close()
	})
	return New(pool, cache.NewUserCache(rdb), nil), pool
}

func newTestUser(t *testing.T, pool *pgxpool.Pool, authID *string) (email, uid string, userID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	email = fmt.Sprintf("itest+%d@example.com", time.Now().UnixNano())
	userID = uuid.NewV7()
	if authID == nil {
		uid = uuid.NewV7().String()
	} else {
		uid = *authID
	}

	var auth any
	if authID != nil {
		auth = *authID
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO identity.users (id, auth_id, email, name, status) VALUES ($1, $2, $3, 'ITest', 'ACTIVE')`,
		userID, auth, email); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM identity.users WHERE id = $1`, userID)
	})
	return email, uid, userID
}

// User lama (auth_id masih NULL) harus ditautkan lewat email, bukan error.
// Mengunci bug "LinkAuthIDIfNull memakai UUID acak".
func TestEnsureLinksExistingUserByEmail(t *testing.T) {
	svc, pool := newIntegrationService(t)
	ctx := context.Background()
	email, uid, userID := newTestUser(t, pool, nil)

	got, err := svc.Ensure(ctx, firebase.Identity{UID: uid, Email: email, EmailVerified: true})
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got.ID != userID {
		t.Fatalf("linked to wrong user: got %s, want %s", got.ID, userID)
	}

	var authID string
	if err := pool.QueryRow(ctx, `SELECT auth_id FROM identity.users WHERE id = $1`, userID).Scan(&authID); err != nil {
		t.Fatal(err)
	}
	if authID != uid {
		t.Fatalf("auth_id = %q, want %q", authID, uid)
	}
	if !got.HasRole(api.RoleUser) {
		t.Fatalf("roles = %v, want USER", got.Roles)
	}
}

// User lama TIDAK boleh di-grant ulang role USER tiap /session, supaya
// pencabutan role oleh admin tidak dibatalkan.
func TestEnsureDoesNotRegrantRoleForExistingUser(t *testing.T) {
	svc, pool := newIntegrationService(t)
	ctx := context.Background()
	auth := uuid.NewV7().String()
	email, uid, _ := newTestUser(t, pool, &auth)

	got, err := svc.Ensure(ctx, firebase.Identity{UID: uid, Email: email})
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got.HasRole(api.RoleUser) {
		t.Fatalf("existing user should not be re-granted USER, roles = %v", got.Roles)
	}
}
