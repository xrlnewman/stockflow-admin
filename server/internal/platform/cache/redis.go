package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLocker struct{ Client *redis.Client }

func NewRedisLocker(addr string, db int) *RedisLocker {
	return &RedisLocker{Client: redis.NewClient(&redis.Options{Addr: addr, DB: db})}
}

func (l *RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return l.Client.SetNX(ctx, key, "1", ttl).Result()
}
func (l *RedisLocker) Ping(ctx context.Context) error { return l.Client.Ping(ctx).Err() }
