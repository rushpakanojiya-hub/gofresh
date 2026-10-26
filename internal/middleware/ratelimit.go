package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter is a simple in-memory fixed-window limiter keyed by an
// arbitrary string (we use client IP). Good enough for a single-instance
// deployment; if this app is ever scaled across multiple server instances,
// swap this for a shared store (e.g. Redis) so limits apply globally.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	lastSweep time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// BUG-81: periodically drop idle keys so the map cannot grow forever.
	if now.Sub(rl.lastSweep) > rl.window {
		for k, ts := range rl.requests {
			if len(ts) == 0 || !ts[len(ts)-1].After(cutoff) {
				delete(rl.requests, k)
			}
		}
		rl.lastSweep = now
	}

	kept := rl.requests[key][:0]
	for _, t := range rl.requests[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.requests[key] = kept
		return false
	}
	rl.requests[key] = append(kept, now)
	return true
}

// RateLimit returns middleware allowing at most `limit` requests per client
// IP within `window`. Meant for sensitive, abuse-prone endpoints — most
// importantly OTP send/verify, which are otherwise open to spam and brute
// force.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	rl := newRateLimiter(limit, window)
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests, please try again later"})
			c.Abort()
			return
		}
		c.Next()
	}
}
