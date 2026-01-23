package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"payment-processing/pkg/logging"
)

const (
	// CorrelationIDHeader is the HTTP header for correlation ID
	CorrelationIDHeader = "X-Correlation-ID"
)

type correlationIDKeyType string

const correlationIDKey correlationIDKeyType = "correlationID"

// CorrelationID middleware extracts or generates a correlation ID for request tracing
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing correlation ID in header
		correlationID := r.Header.Get(CorrelationIDHeader)
		if correlationID == "" {
			// Generate new correlation ID if not provided
			correlationID = uuid.New().String()
		}

		// Add to response header for client reference
		w.Header().Set(CorrelationIDHeader, correlationID)

		// Add to context using both our local key and the logging package key
		ctx := context.WithValue(r.Context(), correlationIDKey, correlationID)
		ctx = logging.WithCorrelationID(ctx, correlationID)

		// Also add request ID to logging context if available
		if requestID := GetRequestID(ctx); requestID != "" {
			ctx = logging.WithRequestID(ctx, requestID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetCorrelationID retrieves the correlation ID from the context
func GetCorrelationID(ctx context.Context) string {
	if v := ctx.Value(correlationIDKey); v != nil {
		return v.(string)
	}
	// Fallback to logging package's context
	return logging.GetCorrelationID(ctx)
}

// WithCorrelationID creates a context with a correlation ID
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	ctx = context.WithValue(ctx, correlationIDKey, correlationID)
	return logging.WithCorrelationID(ctx, correlationID)
}
