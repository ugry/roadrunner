// Amazon ElastiCache for Redis client (managed cache/sessions).
// Enabled when REDIS_URL is set (e.g. rediss://:<auth>@<endpoint>:6379).
// No-ops safely when unconfigured so the prototype still runs locally.
package main

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func initRedis() {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return
	}
	rdb = redis.NewClient(opt)
}

func cacheGet(ctx context.Context, key string) ([]byte, bool) {
	if rdb == nil {
		return nil, false
	}
	b, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, false
	}
	return b, true
}

func cacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) {
	if rdb == nil {
		return
	}
	rdb.Set(ctx, key, val, ttl)
}

// redisHealthy reports true when Redis is unconfigured (not a failure) or reachable.
func redisHealthy(ctx context.Context) bool {
	if rdb == nil {
		return true
	}
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return rdb.Ping(c).Err() == nil
}
