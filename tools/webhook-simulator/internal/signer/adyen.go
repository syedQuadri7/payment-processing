package signer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
)

// AdyenSigner creates Adyen HMAC signatures.
type AdyenSigner struct{}

// NewAdyenSigner creates a new Adyen signer.
func NewAdyenSigner() *AdyenSigner {
	return &AdyenSigner{}
}

// Provider returns the provider name.
func (s *AdyenSigner) Provider() string {
	return "adyen"
}

// Sign generates Adyen HMAC signature header.
// Adyen uses HMAC-SHA256 with Base64 encoding.
func (s *AdyenSigner) Sign(payload []byte, secret string, opts SignOpts) (http.Header, error) {
	headers := make(http.Header)

	if opts.SkipSignature {
		return headers, nil
	}

	signingKey := secret
	if opts.InvalidKey {
		signingKey = "invalid_hmac_key_for_testing"
	}

	// Decode the secret if it's Base64 encoded (Adyen provides Base64 encoded keys)
	keyBytes, err := base64.StdEncoding.DecodeString(signingKey)
	if err != nil {
		// If not Base64, use as-is
		keyBytes = []byte(signingKey)
	}

	// Compute HMAC-SHA256
	h := hmac.New(sha256.New, keyBytes)
	h.Write(payload)
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	headers.Set("X-Adyen-Hmac-Signature", signature)

	return headers, nil
}

// VerifyAdyenSignature verifies an Adyen HMAC signature.
func VerifyAdyenSignature(payload []byte, signature string, secret string) error {
	// Decode the secret
	keyBytes, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		keyBytes = []byte(secret)
	}

	// Compute expected signature
	h := hmac.New(sha256.New, keyBytes)
	h.Write(payload)
	expected := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return ErrSignatureMismatch
	}

	return nil
}

// ErrSignatureMismatch is returned when signature verification fails.
var ErrSignatureMismatch = signatureError("signature mismatch")

type signatureError string

func (e signatureError) Error() string { return string(e) }
