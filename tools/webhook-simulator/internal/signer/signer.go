package signer

import (
	"fmt"
	"net/http"
	"time"
)

// SignOpts controls signature behavior for testing scenarios.
type SignOpts struct {
	// SkipSignature omits the signature header entirely.
	SkipSignature bool

	// InvalidKey uses an incorrect signing key.
	InvalidKey bool

	// TimestampOffset shifts the timestamp by this duration (for replay testing).
	TimestampOffset time.Duration
}

// Signer creates provider-specific webhook signatures.
type Signer interface {
	// Sign generates signature headers for the given payload.
	Sign(payload []byte, secret string, opts SignOpts) (http.Header, error)

	// Provider returns the provider name (stripe, adyen, paypal).
	Provider() string
}

// Registry holds all available signers.
type Registry struct {
	signers map[string]Signer
}

// NewRegistry creates a new signer registry with all providers.
func NewRegistry() *Registry {
	r := &Registry{
		signers: make(map[string]Signer),
	}

	// Register all providers
	r.Register(NewStripeSigner())
	r.Register(NewAdyenSigner())
	r.Register(NewPayPalSigner())

	return r
}

// Register adds a signer to the registry.
func (r *Registry) Register(s Signer) {
	r.signers[s.Provider()] = s
}

// Get returns a signer for the given provider.
func (r *Registry) Get(provider string) (Signer, error) {
	s, ok := r.signers[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
	return s, nil
}
