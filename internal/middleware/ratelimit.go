package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// InMemoryRateLimiter limits requests per key (e.g. IP or user ID).
type InMemoryRateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewInMemoryRateLimiter(limit int, window time.Duration) *InMemoryRateLimiter {
	r := &InMemoryRateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	go r.cleanup()
	return r
}

func (r *InMemoryRateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-r.window)
	times := r.requests[key]
	// drop expired
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= r.limit {
		return false
	}
	valid = append(valid, now)
	r.requests[key] = valid
	return true
}

func (r *InMemoryRateLimiter) cleanup() {
	tick := time.NewTicker(time.Minute)
	for range tick.C {
		r.mu.Lock()
		cutoff := time.Now().Add(-r.window)
		for k, times := range r.requests {
			var valid []time.Time
			for _, t := range times {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(r.requests, k)
			} else {
				r.requests[k] = valid
			}
		}
		r.mu.Unlock()
	}
}

// RateLimit returns a middleware that limits by client IP.
func RateLimit(limiter *InMemoryRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
