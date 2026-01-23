package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"payment-processing/server"
)

// RateLimitConfig holds configuration for the rate limiter
type RateLimitConfig struct {
	// ReadLimit is the maximum read requests per minute
	ReadLimit int
	// WriteLimit is the maximum write requests per minute
	WriteLimit int
	// WindowDuration is the time window for rate limiting
	WindowDuration time.Duration
}

// DefaultRateLimitConfig returns sensible defaults per FD-005
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		ReadLimit:      300, // 300 read requests per minute
		WriteLimit:     100, // 100 write requests per minute
		WindowDuration: time.Minute,
	}
}

// rateLimitEntry tracks rate limit state for a single key
type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

// RateLimiter implements a simple in-memory sliding window rate limiter
type RateLimiter struct {
	config   *RateLimitConfig
	mu       sync.RWMutex
	counters map[string]*rateLimitEntry // key format: "merchantID:read" or "merchantID:write"
}

// NewRateLimiter creates a new rate limiter with the given config
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}
	return &RateLimiter{
		config:   config,
		counters: make(map[string]*rateLimitEntry),
	}
}

// Allow checks if a request should be allowed and updates counters
// Returns (allowed, remaining, resetTime)
func (rl *RateLimiter) Allow(merchantID string, isWrite bool) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	keyType := "read"
	limit := rl.config.ReadLimit
	if isWrite {
		keyType = "write"
		limit = rl.config.WriteLimit
	}

	key := merchantID + ":" + keyType

	entry, exists := rl.counters[key]
	if !exists || now.After(entry.windowEnd) {
		// Start new window
		entry = &rateLimitEntry{
			count:     0,
			windowEnd: now.Add(rl.config.WindowDuration),
		}
		rl.counters[key] = entry
	}

	remaining := limit - entry.count - 1
	if remaining < 0 {
		remaining = 0
	}

	if entry.count >= limit {
		return false, 0, entry.windowEnd
	}

	entry.count++
	return true, remaining, entry.windowEnd
}

// RateLimit creates rate limiting middleware
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	if limiter == nil {
		limiter = NewRateLimiter(nil)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get merchant ID from context (set by auth middleware)
			merchantID := GetMerchantID(r.Context())
			if merchantID == "" {
				// If no merchant ID (unauthenticated), use IP as fallback
				merchantID = "ip:" + r.RemoteAddr
			}

			// Determine if this is a write operation
			isWrite := r.Method == http.MethodPost ||
				r.Method == http.MethodPut ||
				r.Method == http.MethodPatch ||
				r.Method == http.MethodDelete

			allowed, remaining, resetTime := limiter.Allow(merchantID, isWrite)

			// Set rate limit headers
			limit := limiter.config.ReadLimit
			if isWrite {
				limit = limiter.config.WriteLimit
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

			if !allowed {
				requestID := r.Header.Get("X-Request-Id")
				if requestID == "" {
					requestID = "unknown"
				}
				w.Header().Set("Retry-After", strconv.FormatInt(int64(time.Until(resetTime).Seconds()), 10))
				server.WriteError(w, server.NewRateLimitError(), requestID)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Cleanup removes expired entries from the rate limiter
// Should be called periodically to prevent memory growth
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.counters {
		if now.After(entry.windowEnd) {
			delete(rl.counters, key)
		}
	}
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up expired entries
func (rl *RateLimiter) StartCleanupRoutine(interval time.Duration) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.Cleanup()
			case <-stop:
				return
			}
		}
	}()
	return stop
}
