package adapter

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
	"time"

	"payment-processing/internal/domain"
)

func TestStripeAdapter_Provider(t *testing.T) {
	adapter := NewStripeAdapter("test_secret")
	if adapter.Provider() != domain.ProviderStripe {
		t.Errorf("Provider() = %v, want %v", adapter.Provider(), domain.ProviderStripe)
	}
}

func TestStripeAdapter_VerifySignature(t *testing.T) {
	secret := "whsec_test_secret"
	adapter := NewStripeAdapter(secret)

	tests := []struct {
		name      string
		headers   func() http.Header
		body      []byte
		wantErr   bool
		errType   error
	}{
		{
			name: "valid signature",
			headers: func() http.Header {
				h := http.Header{}
				body := []byte(`{"id":"evt_123"}`)
				ts := time.Now().Unix()
				sig := computeTestSignature(ts, body, secret)
				h.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", ts, sig))
				return h
			},
			body:    []byte(`{"id":"evt_123"}`),
			wantErr: false,
		},
		{
			name: "missing signature header",
			headers: func() http.Header {
				return http.Header{}
			},
			body:    []byte(`{"id":"evt_123"}`),
			wantErr: true,
			errType: ErrMissingSignature,
		},
		{
			name: "invalid signature",
			headers: func() http.Header {
				h := http.Header{}
				ts := time.Now().Unix()
				h.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=invalid_sig", ts))
				return h
			},
			body:    []byte(`{"id":"evt_123"}`),
			wantErr: true,
			errType: ErrInvalidSignature,
		},
		{
			name: "expired timestamp",
			headers: func() http.Header {
				h := http.Header{}
				body := []byte(`{"id":"evt_123"}`)
				// 10 minutes ago (beyond 5 minute tolerance)
				ts := time.Now().Add(-10 * time.Minute).Unix()
				sig := computeTestSignature(ts, body, secret)
				h.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", ts, sig))
				return h
			},
			body:    []byte(`{"id":"evt_123"}`),
			wantErr: true,
			errType: ErrTimestampExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.VerifySignature(context.Background(), tt.headers(), tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifySignature() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && err != tt.errType {
				t.Errorf("VerifySignature() error = %v, want %v", err, tt.errType)
			}
		})
	}
}

func TestStripeAdapter_ParseWebhook(t *testing.T) {
	adapter := NewStripeAdapter("test_secret")

	tests := []struct {
		name       string
		body       []byte
		wantType   domain.CanonicalEventType
		wantID     string
		wantNil    bool
	}{
		{
			name: "payment_intent.succeeded",
			body: []byte(`{
				"id": "evt_123",
				"type": "payment_intent.succeeded",
				"created": 1609459200,
				"data": {
					"object": {
						"id": "pi_abc",
						"amount": 10000,
						"currency": "usd",
						"metadata": {"internal_id": "int_123"}
					}
				}
			}`),
			wantType: domain.EventAuthorizationSucceeded,
			wantID:   "evt_123",
		},
		{
			name: "payment_intent.payment_failed",
			body: []byte(`{
				"id": "evt_456",
				"type": "payment_intent.payment_failed",
				"created": 1609459200,
				"data": {
					"object": {
						"id": "pi_xyz",
						"amount": 5000,
						"currency": "usd",
						"last_payment_error": {
							"decline_code": "insufficient_funds",
							"message": "Your card has insufficient funds"
						}
					}
				}
			}`),
			wantType: domain.EventAuthorizationFailed,
			wantID:   "evt_456",
		},
		{
			name: "unknown event type",
			body: []byte(`{
				"id": "evt_789",
				"type": "unknown.event",
				"created": 1609459200,
				"data": {"object": {}}
			}`),
			wantNil: true,
		},
		{
			name: "charge.captured",
			body: []byte(`{
				"id": "evt_cap",
				"type": "charge.captured",
				"created": 1609459200,
				"data": {
					"object": {
						"id": "ch_123",
						"amount": 10000,
						"currency": "usd"
					}
				}
			}`),
			wantType: domain.EventCaptureSucceeded,
			wantID:   "evt_cap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := adapter.ParseWebhook(context.Background(), http.Header{}, tt.body)
			if err != nil {
				t.Fatalf("ParseWebhook() error = %v", err)
			}

			if tt.wantNil {
				if len(events) != 0 {
					t.Errorf("ParseWebhook() returned %d events, want 0", len(events))
				}
				return
			}

			if len(events) != 1 {
				t.Fatalf("ParseWebhook() returned %d events, want 1", len(events))
			}

			if events[0].Type != tt.wantType {
				t.Errorf("ParseWebhook() Type = %v, want %v", events[0].Type, tt.wantType)
			}
			if events[0].ID != tt.wantID {
				t.Errorf("ParseWebhook() ID = %v, want %v", events[0].ID, tt.wantID)
			}
			if events[0].Provider != domain.ProviderStripe {
				t.Errorf("ParseWebhook() Provider = %v, want %v", events[0].Provider, domain.ProviderStripe)
			}
		})
	}
}

func TestStripeAdapter_FormatResponse(t *testing.T) {
	adapter := NewStripeAdapter("test_secret")
	code, body := adapter.FormatResponse()

	if code != http.StatusOK {
		t.Errorf("FormatResponse() code = %v, want %v", code, http.StatusOK)
	}
	if body != "" {
		t.Errorf("FormatResponse() body = %v, want empty", body)
	}
}

func TestMapStripeDeclineType(t *testing.T) {
	tests := []struct {
		code     string
		wantType domain.DeclineType
	}{
		{"insufficient_funds", domain.DeclineTypeSoft},
		{"card_declined", domain.DeclineTypeSoft},
		{"expired_card", domain.DeclineTypeHard},
		{"incorrect_cvc", domain.DeclineTypeHard},
		{"fraudulent", domain.DeclineTypeFraud},
		{"stolen_card", domain.DeclineTypeFraud},
		{"unknown_code", domain.DeclineTypeSoft}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := mapStripeDeclineType(tt.code)
			if got != tt.wantType {
				t.Errorf("mapStripeDeclineType(%s) = %v, want %v", tt.code, got, tt.wantType)
			}
		})
	}
}

// computeTestSignature computes a valid Stripe signature for testing
func computeTestSignature(timestamp int64, body []byte, secret string) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signedPayload))
	return hex.EncodeToString(h.Sum(nil))
}
