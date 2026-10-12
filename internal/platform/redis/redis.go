// Package redis provides the Redis client (go-redis/v9).
package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"app/internal/platform/config"
)

// New opens a Redis client and verifies connectivity. The caller owns the client
// and must Close it on shutdown.
func New(ctx context.Context, cfg config.Redis) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return client, nil
}
