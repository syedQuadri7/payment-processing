package middleware

import (
	"context"
	"net/http"
	"strings"

	"payment-processing/services/payment-api"
)

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

const (
	// ContextKeyAPIKey is the context key for the authenticated API key
	ContextKeyAPIKey ContextKey = "api_key"
	// ContextKeyMerchantID is the context key for the merchant ID
	ContextKeyMerchantID ContextKey = "merchant_id"
	// ContextKeyKeyType is the context key for the key type (secret/publishable)
	ContextKeyKeyType ContextKey = "key_type"
	// ContextKeyTestMode is the context key for test mode flag
	ContextKeyTestMode ContextKey = "test_mode"
)

// APIKeyInfo contains information extracted from an API key
type APIKeyInfo struct {
	Key        string
	KeyType    string // "secret" or "publishable"
	TestMode   bool
	MerchantID string
}

// AuthConfig holds configuration for the authentication middleware
type AuthConfig struct {
	// SkipPaths are paths that don't require authentication (e.g., /health, /webhooks)
	SkipPaths []string
	// ValidateKey is a function to validate an API key and return merchant info
	// For now, we just validate the format; production would check database
	ValidateKey func(key string) (*APIKeyInfo, error)
}

// DefaultAuthConfig returns a default auth configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		SkipPaths: []string{"/health", "/health/live", "/health/ready", "/metrics", "/webhooks/"},
		ValidateKey: func(key string) (*APIKeyInfo, error) {
			// Simple validation based on key format
			// Production would validate against database
			return parseAPIKey(key)
		},
	}
}

// parseAPIKey parses and validates an API key format
// Key format: sk_live_xxx, sk_test_xxx, pk_live_xxx, pk_test_xxx
func parseAPIKey(key string) (*APIKeyInfo, error) {
	parts := strings.Split(key, "_")
	if len(parts) < 3 {
		return nil, nil // Invalid format
	}

	keyType := parts[0]
	if keyType != "sk" && keyType != "pk" {
		return nil, nil // Invalid key type prefix
	}

	mode := parts[1]
	if mode != "live" && mode != "test" {
		return nil, nil // Invalid mode
	}

	info := &APIKeyInfo{
		Key:      key,
		TestMode: mode == "test",
	}

	if keyType == "sk" {
		info.KeyType = "secret"
	} else {
		info.KeyType = "publishable"
	}

	// For now, use a placeholder merchant ID
	// Production would look up the key in the database
	suffix := parts[2]
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	info.MerchantID = "merchant_" + suffix

	return info, nil
}

// Auth creates authentication middleware
func Auth(config *AuthConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultAuthConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should skip authentication
			for _, path := range config.SkipPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				requestID := r.Header.Get("X-Request-Id")
				if requestID == "" {
					requestID = "unknown"
				}
				server.WriteError(w, server.NewAuthenticationError("Missing Authorization header"), requestID)
				return
			}

			// Parse Bearer token
			if !strings.HasPrefix(authHeader, "Bearer ") {
				requestID := r.Header.Get("X-Request-Id")
				if requestID == "" {
					requestID = "unknown"
				}
				server.WriteError(w, server.NewAuthenticationError("Invalid Authorization header format. Expected: Bearer <token>"), requestID)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				requestID := r.Header.Get("X-Request-Id")
				if requestID == "" {
					requestID = "unknown"
				}
				server.WriteError(w, server.NewAuthenticationError("Empty API key"), requestID)
				return
			}

			// Validate the API key
			keyInfo, err := config.ValidateKey(token)
			if err != nil {
				requestID := r.Header.Get("X-Request-Id")
				if requestID == "" {
					requestID = "unknown"
				}
				server.WriteError(w, server.NewInternalError("Error validating API key"), requestID)
				return
			}

			if keyInfo == nil {
				requestID := r.Header.Get("X-Request-Id")
				if requestID == "" {
					requestID = "unknown"
				}
				server.WriteError(w, server.NewAuthenticationError("Invalid API key"), requestID)
				return
			}

			// Add key info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, ContextKeyAPIKey, keyInfo.Key)
			ctx = context.WithValue(ctx, ContextKeyMerchantID, keyInfo.MerchantID)
			ctx = context.WithValue(ctx, ContextKeyKeyType, keyInfo.KeyType)
			ctx = context.WithValue(ctx, ContextKeyTestMode, keyInfo.TestMode)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSecretKey middleware ensures the request is made with a secret key (not publishable)
func RequireSecretKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keyType, ok := r.Context().Value(ContextKeyKeyType).(string)
		if !ok || keyType != "secret" {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = "unknown"
			}
			server.WriteError(w, server.NewAuthenticationError("This endpoint requires a secret API key (sk_*)"), requestID)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetMerchantID extracts the merchant ID from the request context
func GetMerchantID(ctx context.Context) string {
	if id, ok := ctx.Value(ContextKeyMerchantID).(string); ok {
		return id
	}
	return ""
}

// GetAPIKey extracts the API key from the request context
func GetAPIKey(ctx context.Context) string {
	if key, ok := ctx.Value(ContextKeyAPIKey).(string); ok {
		return key
	}
	return ""
}

// IsTestMode checks if the request is using a test API key
func IsTestMode(ctx context.Context) bool {
	if testMode, ok := ctx.Value(ContextKeyTestMode).(bool); ok {
		return testMode
	}
	return false
}
