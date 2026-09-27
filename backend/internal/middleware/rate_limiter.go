package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages in-memory token buckets keyed by IP or User ID.
type RateLimiter struct {
	mu            sync.RWMutex
	clients       map[string]*clientLimiter
	rateLimit     rate.Limit
	burst         int
	retryAfterSec time.Duration
	disabled      bool
}

// SetDisabled programmatically enables or disables rate limit enforcement (useful in testing).
func (rl *RateLimiter) SetDisabled(disabled bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.disabled = disabled
}

// NewRateLimiter creates an in-memory token-bucket rate limiter with automatic stale key eviction.
func NewRateLimiter(r rate.Limit, b int, retryAfterSec time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:       make(map[string]*clientLimiter),
		rateLimit:     r,
		burst:         b,
		retryAfterSec: retryAfterSec,
	}

	// Periodically evict limiters inactive for more than 10 minutes
	go rl.cleanupRoutine(10 * time.Minute)

	return rl
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[key]
	if !exists {
		lim := rate.NewLimiter(rl.rateLimit, rl.burst)
		rl.clients[key] = &clientLimiter{
			limiter:  lim,
			lastSeen: time.Now(),
		}
		return lim
	}

	client.lastSeen = time.Now()
	return client.limiter
}

func (rl *RateLimiter) cleanupRoutine(cleanupInterval time.Duration) {
	ticker := time.NewTicker(cleanupInterval)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, client := range rl.clients {
			if now.Sub(client.lastSeen) > cleanupInterval {
				delete(rl.clients, key)
			}
		}
		rl.mu.Unlock()
	}
}

// LimitByIP enforces token-bucket rate limits per client IP.
func (rl *RateLimiter) LimitByIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		rl.mu.RLock()
		disabled := rl.disabled
		rl.mu.RUnlock()
		if disabled {
			c.Next()
			return
		}

		key := c.ClientIP()
		limiter := rl.getLimiter(key)

		if !limiter.Allow() {
			retryAfter := int(rl.retryAfterSec / time.Second)
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests. Please slow down.",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

// LimitByClientOrUser enforces rate limits per User ID if authenticated, falling back to IP.
func (rl *RateLimiter) LimitByClientOrUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		rl.mu.RLock()
		disabled := rl.disabled
		rl.mu.RUnlock()
		if disabled {
			c.Next()
			return
		}

		key := c.ClientIP()
		if userID, exists := GetUserID(c); exists {
			key = "usr:" + userID.String()
		}

		limiter := rl.getLimiter(key)
		if !limiter.Allow() {
			retryAfter := int(rl.retryAfterSec / time.Second)
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests. Please slow down.",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}

// Predefined Rate Limiter Configurations
// 1. Public Read Limiter: 60 requests/minute per IP (burst 10)
func NewPublicReadLimiter() *RateLimiter {
	return NewRateLimiter(rate.Every(1*time.Second), 10, 2*time.Second)
}

// 2. Action Limiter: 12 mutating requests/minute per User or IP (burst 3)
func NewActionLimiter() *RateLimiter {
	return NewRateLimiter(rate.Every(5*time.Second), 3, 5*time.Second)
}

// 3. Quote Limiter: High-frequency calculation quota for interactive trading terminals (120 requests/minute per IP, burst 20)
func NewQuoteLimiter() *RateLimiter {
	return NewRateLimiter(rate.Every(500*time.Millisecond), 20, 1*time.Second)
}
