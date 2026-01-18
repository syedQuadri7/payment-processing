package signer

import (
	"strings"
	"testing"
	"time"
)

func TestStripeSigner_Sign(t *testing.T) {
	signer := NewStripeSigner()
	payload := []byte(`{"test": "payload"}`)
	secret := "whsec_test_secret"

	t.Run("normal signature", func(t *testing.T) {
		headers, err := signer.Sign(payload, secret, SignOpts{})
		if err != nil {
			t.Errorf("Sign() error = %v", err)
		}

		sig := headers.Get("Stripe-Signature")
		if sig == "" {
			t.Error("Expected Stripe-Signature header")
		}

		if !strings.HasPrefix(sig, "t=") {
			t.Error("Expected signature to start with t=")
		}

		if !strings.Contains(sig, ",v1=") {
			t.Error("Expected signature to contain ,v1=")
		}
	})

	t.Run("skip signature", func(t *testing.T) {
		headers, err := signer.Sign(payload, secret, SignOpts{SkipSignature: true})
		if err != nil {
			t.Errorf("Sign() error = %v", err)
		}

		sig := headers.Get("Stripe-Signature")
		if sig != "" {
			t.Error("Expected no Stripe-Signature header when skipping")
		}
	})

	t.Run("invalid key produces different signature", func(t *testing.T) {
		normalHeaders, _ := signer.Sign(payload, secret, SignOpts{})
		invalidHeaders, _ := signer.Sign(payload, secret, SignOpts{InvalidKey: true})

		normalSig := normalHeaders.Get("Stripe-Signature")
		invalidSig := invalidHeaders.Get("Stripe-Signature")

		// Extract v1 part
		normalV1 := extractV1(normalSig)
		invalidV1 := extractV1(invalidSig)

		if normalV1 == invalidV1 {
			t.Error("Expected different signatures with invalid key")
		}
	})

	t.Run("timestamp offset", func(t *testing.T) {
		headers, err := signer.Sign(payload, secret, SignOpts{
			TimestampOffset: -5 * time.Minute,
		})
		if err != nil {
			t.Errorf("Sign() error = %v", err)
		}

		sig := headers.Get("Stripe-Signature")
		if sig == "" {
			t.Error("Expected Stripe-Signature header")
		}
	})
}

func TestAdyenSigner_Sign(t *testing.T) {
	signer := NewAdyenSigner()
	payload := []byte(`{"test": "payload"}`)
	secret := "test_hmac_key"

	t.Run("normal signature", func(t *testing.T) {
		headers, err := signer.Sign(payload, secret, SignOpts{})
		if err != nil {
			t.Errorf("Sign() error = %v", err)
		}

		sig := headers.Get("X-Adyen-Hmac-Signature")
		if sig == "" {
			t.Error("Expected X-Adyen-Hmac-Signature header")
		}
	})

	t.Run("skip signature", func(t *testing.T) {
		headers, err := signer.Sign(payload, secret, SignOpts{SkipSignature: true})
		if err != nil {
			t.Errorf("Sign() error = %v", err)
		}

		sig := headers.Get("X-Adyen-Hmac-Signature")
		if sig != "" {
			t.Error("Expected no X-Adyen-Hmac-Signature header when skipping")
		}
	})
}

func TestPayPalSigner_Sign(t *testing.T) {
	signer := NewPayPalSigner()
	payload := []byte(`{"test": "payload"}`)
	secret := "test_webhook_id"

	t.Run("normal headers", func(t *testing.T) {
		headers, err := signer.Sign(payload, secret, SignOpts{})
		if err != nil {
			t.Errorf("Sign() error = %v", err)
		}

		requiredHeaders := []string{
			"PAYPAL-TRANSMISSION-ID",
			"PAYPAL-TRANSMISSION-TIME",
			"PAYPAL-AUTH-ALGO",
			"PAYPAL-CERT-URL",
			"PAYPAL-TRANSMISSION-SIG",
		}

		for _, h := range requiredHeaders {
			if headers.Get(h) == "" {
				t.Errorf("Expected %s header", h)
			}
		}
	})

	t.Run("skip signature", func(t *testing.T) {
		headers, err := signer.Sign(payload, secret, SignOpts{SkipSignature: true})
		if err != nil {
			t.Errorf("Sign() error = %v", err)
		}

		if headers.Get("PAYPAL-TRANSMISSION-ID") != "" {
			t.Error("Expected no PayPal headers when skipping")
		}
	})
}

func TestSignerRegistry(t *testing.T) {
	registry := NewRegistry()

	providers := []string{"stripe", "adyen", "paypal"}
	for _, p := range providers {
		t.Run(p, func(t *testing.T) {
			signer, err := registry.Get(p)
			if err != nil {
				t.Errorf("Get(%s) error = %v", p, err)
			}
			if signer.Provider() != p {
				t.Errorf("Provider() = %v, want %v", signer.Provider(), p)
			}
		})
	}

	t.Run("unknown provider", func(t *testing.T) {
		_, err := registry.Get("unknown")
		if err == nil {
			t.Error("Expected error for unknown provider")
		}
	})
}

func extractV1(sig string) string {
	parts := strings.Split(sig, ",")
	for _, p := range parts {
		if strings.HasPrefix(p, "v1=") {
			return strings.TrimPrefix(p, "v1=")
		}
	}
	return ""
}
