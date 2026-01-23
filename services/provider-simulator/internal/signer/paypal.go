package signer

import (
	"fmt"
	"net/http"
)

// PayPalSigner creates PayPal webhook verification headers.
// Note: Real PayPal signature verification requires calling PayPal's API.
// This implementation provides mock headers for testing purposes.
type PayPalSigner struct{}

// NewPayPalSigner creates a new PayPal signer.
func NewPayPalSigner() *PayPalSigner {
	return &PayPalSigner{}
}

// Provider returns the provider name.
func (s *PayPalSigner) Provider() string {
	return "paypal"
}

// Sign generates PayPal webhook verification headers.
// PayPal uses several headers for webhook verification:
// - PAYPAL-TRANSMISSION-ID: Unique ID for the transmission
// - PAYPAL-TRANSMISSION-TIME: Timestamp of the transmission
// - PAYPAL-TRANSMISSION-SIG: The signature (requires PayPal API for real verification)
// - PAYPAL-CERT-URL: URL to the certificate used for signing
// - PAYPAL-AUTH-ALGO: Algorithm used for signing
func (s *PayPalSigner) Sign(payload []byte, secret string, opts SignOpts) (http.Header, error) {
	headers := make(http.Header)

	if opts.SkipSignature {
		return headers, nil
	}

	// Generate mock transmission ID
	transmissionID := fmt.Sprintf("%s", generateMockID())
	if opts.InvalidKey {
		transmissionID = "invalid-transmission-id"
	}

	// Set PayPal-specific headers
	headers.Set("PAYPAL-TRANSMISSION-ID", transmissionID)
	headers.Set("PAYPAL-TRANSMISSION-TIME", "2024-01-15T12:00:00Z")
	headers.Set("PAYPAL-AUTH-ALGO", "SHA256withRSA")
	headers.Set("PAYPAL-CERT-URL", "https://api.sandbox.paypal.com/v1/notifications/certs/CERT-360caa42-fca2a594-a5cafa77")

	// Mock signature - in production, this would be an actual RSA signature
	// The webhook_id is used during verification to generate the expected signature
	mockSig := "mock-signature-for-testing"
	if opts.InvalidKey {
		mockSig = "invalid-signature"
	}
	headers.Set("PAYPAL-TRANSMISSION-SIG", mockSig)

	return headers, nil
}

// generateMockID creates a mock transmission ID.
func generateMockID() string {
	return fmt.Sprintf("TRANS-%s", generateID())
}
