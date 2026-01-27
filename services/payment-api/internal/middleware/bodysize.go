package middleware

import (
	"net/http"
)

const (
	// DefaultAPIBodyLimit is the max body size for API requests (64KB)
	DefaultAPIBodyLimit int64 = 64 * 1024
	// DefaultWebhookBodyLimit is the max body size for webhook requests (1MB)
	DefaultWebhookBodyLimit int64 = 1024 * 1024
)

// BodySizeLimit returns middleware that limits request body size.
// When the limit is exceeded, http.MaxBytesReader returns an error
// on the next Read call.
func BodySizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// APIBodyLimit returns middleware with the default API body limit (64KB)
func APIBodyLimit() func(http.Handler) http.Handler {
	return BodySizeLimit(DefaultAPIBodyLimit)
}

// WebhookBodyLimit returns middleware with the default webhook body limit (1MB)
func WebhookBodyLimit() func(http.Handler) http.Handler {
	return BodySizeLimit(DefaultWebhookBodyLimit)
}
