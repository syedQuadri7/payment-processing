package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const (
	// ContextKeyRequestID is the context key for the request ID
	ContextKeyRequestID ContextKey = "request_id"
)

// RequestID middleware adds a unique request ID to each request
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client provided a request ID
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add to response header
		w.Header().Set("X-Request-Id", requestID)

		// Add to context
		ctx := context.WithValue(r.Context(), ContextKeyRequestID, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the request context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return id
	}
	return ""
}

// GetRequestIDFromRequest extracts the request ID from the HTTP request
func GetRequestIDFromRequest(r *http.Request) string {
	// First try context
	if id := GetRequestID(r.Context()); id != "" {
		return id
	}
	// Fall back to header
	return r.Header.Get("X-Request-Id")
}
