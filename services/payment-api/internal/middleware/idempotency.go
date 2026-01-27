package middleware

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"payment-processing/services/payment-api"
)

// IdempotencyConfig holds configuration for idempotency handling
type IdempotencyConfig struct {
	// TTL is how long to cache idempotent responses (default: 24 hours per FD-003)
	TTL time.Duration
	// RequireMethods are HTTP methods that require idempotency keys
	RequireMethods []string
	// SkipPaths are paths that don't require idempotency keys
	SkipPaths []string
}

// DefaultIdempotencyConfig returns sensible defaults per FD-003
func DefaultIdempotencyConfig() *IdempotencyConfig {
	return &IdempotencyConfig{
		TTL:            24 * time.Hour,
		RequireMethods: []string{http.MethodPost, http.MethodPut},
		SkipPaths:      []string{"/health", "/webhooks/", "/metrics"},
	}
}

// cachedResponse stores a cached idempotent response
type cachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	ExpiresAt  time.Time
	InProgress bool // True if request is currently being processed
}

// IdempotencyStore manages idempotency key storage
type IdempotencyStore struct {
	config *IdempotencyConfig
	mu     sync.RWMutex
	cache  map[string]*cachedResponse // key format: "merchantID:idempotencyKey"
}

// NewIdempotencyStore creates a new idempotency store
func NewIdempotencyStore(config *IdempotencyConfig) *IdempotencyStore {
	if config == nil {
		config = DefaultIdempotencyConfig()
	}
	return &IdempotencyStore{
		config: config,
		cache:  make(map[string]*cachedResponse),
	}
}

// responseRecorder captures the response for caching
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Idempotency creates idempotency middleware
func Idempotency(store *IdempotencyStore) func(http.Handler) http.Handler {
	if store == nil {
		store = NewIdempotencyStore(nil)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this method requires idempotency
			requiresIdempotency := false
			for _, m := range store.config.RequireMethods {
				if r.Method == m {
					requiresIdempotency = true
					break
				}
			}

			if !requiresIdempotency {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path should skip idempotency
			for _, path := range store.config.SkipPaths {
				if len(r.URL.Path) >= len(path) && r.URL.Path[:len(path)] == path {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Get idempotency key from header
			idempotencyKey := r.Header.Get("Idempotency-Key")
			if idempotencyKey == "" {
				requestID := r.Header.Get("X-Request-Id")
				if requestID == "" {
					requestID = "unknown"
				}
				server.WriteError(w, server.NewMissingHeaderError("Idempotency-Key"), requestID)
				return
			}

			// Get merchant ID for scoping
			merchantID := GetMerchantID(r.Context())
			if merchantID == "" {
				merchantID = "unknown"
			}

			cacheKey := merchantID + ":" + idempotencyKey

			// Check cache
			store.mu.Lock()
			cached, exists := store.cache[cacheKey]

			if exists {
				if cached.InProgress {
					// Another request with the same key is in progress
					store.mu.Unlock()
					requestID := r.Header.Get("X-Request-Id")
					if requestID == "" {
						requestID = "unknown"
					}
					server.WriteError(w, server.NewConflictError("idempotency_conflict", "A request with this idempotency key is already in progress"), requestID)
					return
				}

				if time.Now().Before(cached.ExpiresAt) {
					// Return cached response
					store.mu.Unlock()

					// Copy headers
					for k, v := range cached.Headers {
						for _, val := range v {
							w.Header().Add(k, val)
						}
					}
					w.Header().Set("X-Idempotency-Key", idempotencyKey)
					w.Header().Set("X-Idempotency-Cached", "true")
					w.WriteHeader(cached.StatusCode)
					w.Write(cached.Body)
					return
				}

				// Expired, remove and continue
				delete(store.cache, cacheKey)
			}

			// Mark as in progress
			store.cache[cacheKey] = &cachedResponse{InProgress: true}
			store.mu.Unlock()

			// Create response recorder to capture response
			recorder := newResponseRecorder(w)

			// Add idempotency key header to response
			recorder.Header().Set("X-Idempotency-Key", idempotencyKey)

			// Process request
			next.ServeHTTP(recorder, r)

			// Cache the response
			store.mu.Lock()
			store.cache[cacheKey] = &cachedResponse{
				StatusCode: recorder.statusCode,
				Headers:    recorder.Header().Clone(),
				Body:       recorder.body.Bytes(),
				ExpiresAt:  time.Now().Add(store.config.TTL),
				InProgress: false,
			}
			store.mu.Unlock()
		})
	}
}

// Cleanup removes expired entries from the idempotency store
func (s *IdempotencyStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.cache {
		if !entry.InProgress && now.After(entry.ExpiresAt) {
			delete(s.cache, key)
		}
	}
}

// StartCleanupRoutine starts a background goroutine that periodically cleans up
func (s *IdempotencyStore) StartCleanupRoutine(interval time.Duration) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.Cleanup()
			case <-stop:
				return
			}
		}
	}()
	return stop
}

// GetIdempotencyKey extracts the idempotency key from the request
func GetIdempotencyKey(r *http.Request) string {
	return r.Header.Get("Idempotency-Key")
}
