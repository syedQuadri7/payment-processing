package adapter

import (
	"context"
	"net/http"

	"payment-processing/internal/domain"
)

// WebhookAdapter defines the interface for processing webhooks from payment providers
// Each provider implements this interface to normalize webhooks into canonical events
type WebhookAdapter interface {
	// Provider returns the provider this adapter handles
	Provider() domain.Provider

	// VerifySignature verifies the webhook signature/authenticity
	// Returns an error if the signature is invalid
	VerifySignature(ctx context.Context, headers http.Header, body []byte) error

	// ParseWebhook parses the raw webhook payload into canonical events
	// May return multiple events for providers that batch notifications (e.g., Adyen)
	ParseWebhook(ctx context.Context, headers http.Header, body []byte) ([]*domain.CanonicalEvent, error)

	// FormatResponse returns the provider-specific success response
	// Some providers require specific response formats (e.g., Adyen requires "[accepted]")
	FormatResponse() (int, string)
}

// AdapterConfig holds configuration for all webhook adapters
type AdapterConfig struct {
	// Stripe configuration
	StripeWebhookSecret string

	// Adyen configuration
	AdyenHMACKey string

	// PayPal configuration
	PayPalClientID     string
	PayPalClientSecret string
	PayPalWebhookID    string
	PayPalAPIURL       string // api-m.sandbox.paypal.com or api-m.paypal.com
}

// AdapterRegistry holds all configured webhook adapters
type AdapterRegistry struct {
	adapters map[domain.Provider]WebhookAdapter
}

// NewAdapterRegistry creates a new adapter registry with all configured adapters
func NewAdapterRegistry(cfg AdapterConfig) *AdapterRegistry {
	registry := &AdapterRegistry{
		adapters: make(map[domain.Provider]WebhookAdapter),
	}

	// Register Stripe adapter if configured
	if cfg.StripeWebhookSecret != "" {
		registry.adapters[domain.ProviderStripe] = NewStripeAdapter(cfg.StripeWebhookSecret)
	}

	// Register Adyen adapter if configured
	if cfg.AdyenHMACKey != "" {
		registry.adapters[domain.ProviderAdyen] = NewAdyenAdapter(cfg.AdyenHMACKey)
	}

	// Register PayPal adapter if configured
	if cfg.PayPalClientID != "" && cfg.PayPalClientSecret != "" && cfg.PayPalWebhookID != "" {
		registry.adapters[domain.ProviderPayPal] = NewPayPalAdapter(
			cfg.PayPalClientID,
			cfg.PayPalClientSecret,
			cfg.PayPalWebhookID,
			cfg.PayPalAPIURL,
		)
	}

	return registry
}

// GetAdapter returns the adapter for the given provider
func (r *AdapterRegistry) GetAdapter(provider domain.Provider) (WebhookAdapter, bool) {
	adapter, ok := r.adapters[provider]
	return adapter, ok
}

// Providers returns a list of all configured providers
func (r *AdapterRegistry) Providers() []domain.Provider {
	providers := make([]domain.Provider, 0, len(r.adapters))
	for provider := range r.adapters {
		providers = append(providers, provider)
	}
	return providers
}

// Common errors
var (
	ErrInvalidSignature   = &AdapterError{Code: "INVALID_SIGNATURE", Message: "webhook signature verification failed"}
	ErrMissingSignature   = &AdapterError{Code: "MISSING_SIGNATURE", Message: "webhook signature header missing"}
	ErrInvalidPayload     = &AdapterError{Code: "INVALID_PAYLOAD", Message: "failed to parse webhook payload"}
	ErrUnknownEventType   = &AdapterError{Code: "UNKNOWN_EVENT", Message: "unknown webhook event type"}
	ErrTimestampExpired   = &AdapterError{Code: "TIMESTAMP_EXPIRED", Message: "webhook timestamp expired (replay attack protection)"}
	ErrVerificationFailed = &AdapterError{Code: "VERIFICATION_FAILED", Message: "webhook verification API call failed"}
)

// AdapterError represents an adapter-specific error
type AdapterError struct {
	Code    string
	Message string
	Err     error
}

func (e *AdapterError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AdapterError) Unwrap() error {
	return e.Err
}
