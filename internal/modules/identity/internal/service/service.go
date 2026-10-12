// Package service berisi business logic modul identity.
package service

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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
