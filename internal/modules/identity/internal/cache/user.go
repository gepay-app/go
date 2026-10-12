package cache

import (
	"app/internal/modules/identity/api"
	"app/internal/platform/logging"
	"context"
	"encoding/json/v2"
	"github.com/redis/go-redis/v9"
	"time"
)

const userTTL = 10 * time.Minute

type UserCache struct {
	rdb *redis.Client
}

func NewUserCache(rdb *redis.Client) *UserCache { return &UserCache{rdb} }

func userKey(uid string) string { return "identity:user:" + uid }

func (c *UserCache) Get(ctx context.Context, uid string) (api.User, bool) {
	raw, err := c.rdb.Get(ctx, userKey(uid)).Bytes()
	if err != nil {
		return api.User{}, false
	}
	var u api.User
	if err := json.Unmarshal(raw, &u); err != nil {
		return api.User{}, false
	}
	return u, true
}

func (c *UserCache) Set(ctx context.Context, uid string, u api.User) {
	raw, err := json.Marshal(u)
	if err != nil {
		return
	}
	if err := c.rdb.Set(ctx, userKey(uid), raw, userTTL).Err(); err != nil {
		// Cache tidak boleh menggagalkan request; DB tetap sumber kebenaran.
		logging.FromContext(ctx).Warn("identity cache set failed", "error", err)
	}
}

// Invalidate dipanggil setiap mutasi user/role, supaya perubahan langsung terasa.
func (c *UserCache) Invalidate(ctx context.Context, uid string) {
	if err := c.rdb.Del(ctx, userKey(uid)).Err(); err != nil {
		logging.FromContext(ctx).Warn("identity cache del failed", "error", err)
	}
}
