package database

import (
"context"
"log"
"time"

"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
"github.com/redis/go-redis/v9"
)

var RDB *redis.Client
var RedisEnabled bool

// ConnectRedis connects to Redis for caching. If REDIS_URL is not set,
// caching is disabled and the app falls back to hitting Postgres directly
// on every request - this is intentional so local/dev setups without Redis
// still work.
func ConnectRedis(cfg *config.Config) {
if cfg.RedisURL == "" {
log.Println("REDIS_URL not set - caching disabled, all reads will hit Postgres directly")
RedisEnabled = false
return
}

opt, err := redis.ParseURL(cfg.RedisURL)
if err != nil {
log.Printf("Failed to parse REDIS_URL: %v - caching disabled", err)
RedisEnabled = false
return
}

RDB = redis.NewClient(opt)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := RDB.Ping(ctx).Err(); err != nil {
_ = RDB.Close() // BUG-91: do not leak the client pool when Ping fails
log.Printf("Failed to connect to Redis: %v - caching disabled", err)
RedisEnabled = false
return
}

RedisEnabled = true
log.Println("Redis connected successfully - caching enabled")
}
