package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stemsi/exstem-backend/internal/response"
)

// RateLimiter implements a simple per-IP token bucket rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int           // Tokens per interval
	interval time.Duration // Refill interval
}

type visitor struct {
	tokens   int
	lastSeen time.Time
}

// NewRateLimiter creates a RateLimiter (e.g., 10 requests per minute).
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		interval: interval,
	}

	// Cleanup stale visitors every minute.
	go func() {
		for range time.Tick(time.Minute) {
			rl.cleanup()
		}
	}()

	return rl
}

// Middleware returns a Gin middleware that rate-limits requests by IP.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		rl.mu.Lock()
		v, exists := rl.visitors[ip]
		if !exists {
			v = &visitor{tokens: rl.rate, lastSeen: time.Now()}
			rl.visitors[ip] = v
		}

		// Refill tokens based on elapsed time.
		elapsed := time.Since(v.lastSeen)
		refill := int(elapsed/rl.interval) * rl.rate
		if refill > 0 {
			v.tokens += refill
			if v.tokens > rl.rate {
				v.tokens = rl.rate
			}
			v.lastSeen = time.Now()
		}

		if v.tokens <= 0 {
			rl.mu.Unlock()
			response.AbortFail(c, http.StatusTooManyRequests, response.ErrRateLimitExceeded)
			return
		}

		v.tokens--
		rl.mu.Unlock()
		c.Next()
	}
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, v := range rl.visitors {
		if time.Since(v.lastSeen) > 3*time.Minute {
			delete(rl.visitors, ip)
		}
	}
}
