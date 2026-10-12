// Package service berisi business logic modul identity.
package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"app/internal/modules/identity/api"
	"app/internal/platform/logging"
)

// Service mengimplementasi api.Greeter. db dan rdb disimpan untuk dipakai nanti;
// skeleton ini belum menjalankan query apa pun.
type Service struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func New(db *pgxpool.Pool, rdb *redis.Client) *Service {
	return &Service{db: db, rdb: rdb}
}

func (s *Service) Greet(ctx context.Context, name string) (api.Greeting, error) {
	// Logger dari context sudah membawa request_id; jangan pakai logger base.
	logging.FromContext(ctx).Info("greet", "name", name)

	return api.Greeting{Message: "hello, " + name}, nil
}

// Pastikan Service memenuhi kontrak api.Greeter.
var _ api.Greeter = (*Service)(nil)
