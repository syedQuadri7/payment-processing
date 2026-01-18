package signer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

// StripeSigner creates Stripe webhook signatures.
type StripeSigner struct{}

// NewStripeSigner creates a new Stripe signer.
func NewStripeSigner() *StripeSigner {
	return &StripeSigner{}
}

// Provider returns the provider name.
func (s *StripeSigner) Provider() string {
	return "stripe"
}

// Sign generates Stripe-Signature header.
// Format: t={timestamp},v1={signature}
// Signature: HMAC-SHA256 of "{timestamp}.{payload}"
func (s *StripeSigner) Sign(payload []byte, secret string, opts SignOpts) (http.Header, error) {
	headers := make(http.Header)

	if opts.SkipSignature {
		return headers, nil
	}

	timestamp := time.Now().Unix()
	if opts.TimestampOffset != 0 {
		timestamp += int64(opts.TimestampOffset.Seconds())
	}

	signingKey := secret
	if opts.InvalidKey {
		signingKey = "whsec_invalid_key_for_testing"
	}

	// Create signed payload: {timestamp}.{payload}
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))

	// Compute HMAC-SHA256
	h := hmac.New(sha256.New, []byte(signingKey))
	h.Write([]byte(signedPayload))
	signature := hex.EncodeToString(h.Sum(nil))

	// Format: t={timestamp},v1={signature}
	headerValue := fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
	headers.Set("Stripe-Signature", headerValue)

	return headers, nil
}

// VerifySignature is a helper to verify signatures (for testing).
func VerifyStripeSignature(payload []byte, header string, secret string, tolerance time.Duration) error {
	// Parse header
	var timestamp int64
	var signatures []string

	parts := splitHeader(header)
	for _, part := range parts {
		if len(part) < 3 {
			continue
		}
		key := part[:1]
		value := part[2:]
		switch key {
		case "t":
			fmt.Sscanf(value, "%d", &timestamp)
		case "v1":
			signatures = append(signatures, value)
		}
	}

	if timestamp == 0 {
		return fmt.Errorf("missing timestamp in header")
	}
	if len(signatures) == 0 {
		return fmt.Errorf("no signatures found in header")
	}

	// Check timestamp tolerance
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	if tolerance > 0 && time.Duration(diff)*time.Second > tolerance {
		return fmt.Errorf("timestamp outside tolerance")
	}

	// Verify signature
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(payload))
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signedPayload))
	expected := hex.EncodeToString(h.Sum(nil))

	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}

	return fmt.Errorf("signature verification failed")
}

func splitHeader(header string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(header); i++ {
		if i == len(header) || header[i] == ',' {
			if start < i {
				parts = append(parts, header[start:i])
			}
			start = i + 1
		}
	}
	return parts
}
