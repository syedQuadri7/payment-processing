package adapter

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"testing"

	"payment-processing/pkg/domain"
)

func TestAdyenAdapter_Provider(t *testing.T) {
	adapter := NewAdyenAdapter("test_key")
	if adapter.Provider() != domain.ProviderAdyen {
		t.Errorf("Provider() = %v, want %v", adapter.Provider(), domain.ProviderAdyen)
	}
}

func TestAdyenAdapter_VerifySignature(t *testing.T) {
	// Base64-encoded test key
	testKey := base64.StdEncoding.EncodeToString([]byte("test_hmac_key"))
	adapter := NewAdyenAdapter(testKey)

	tests := []struct {
		name    string
		headers func(body []byte) http.Header
		body    []byte
		wantErr bool
		errType error
	}{
		{
			name: "valid signature",
			headers: func(body []byte) http.Header {
				h := http.Header{}
				sig := computeAdyenSignature(body, "test_hmac_key")
				h.Set("HmacSignature", sig)
				return h
			},
			body:    []byte(`{"notificationItems":[]}`),
			wantErr: false,
		},
		{
			name: "missing signature header",
			headers: func(body []byte) http.Header {
				return http.Header{}
			},
			body:    []byte(`{"notificationItems":[]}`),
			wantErr: true,
			errType: ErrMissingSignature,
		},
		{
			name: "invalid signature",
			headers: func(body []byte) http.Header {
				h := http.Header{}
				h.Set("HmacSignature", "invalid_signature")
				return h
			},
			body:    []byte(`{"notificationItems":[]}`),
			wantErr: true,
			errType: ErrInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.VerifySignature(context.Background(), tt.headers(tt.body), tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifySignature() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && err != tt.errType {
				t.Errorf("VerifySignature() error = %v, want %v", err, tt.errType)
			}
		})
	}
}

func TestAdyenAdapter_ParseWebhook(t *testing.T) {
	adapter := NewAdyenAdapter("test_key")

	tests := []struct {
		name      string
		body      []byte
		wantType  domain.CanonicalEventType
		wantID    string
		wantCount int
	}{
		{
			name: "authorization success",
			body: []byte(`{
				"live": "false",
				"notificationItems": [{
					"NotificationRequestItem": {
						"eventCode": "AUTHORISATION",
						"success": "true",
						"pspReference": "psp_123",
						"merchantReference": "int_456",
						"amount": {"currency": "USD", "value": 10000},
						"eventDate": "2021-01-01T00:00:00Z"
					}
				}]
			}`),
			wantType:  domain.EventAuthorizationSucceeded,
			wantID:    "psp_123",
			wantCount: 1,
		},
		{
			name: "authorization failed",
			body: []byte(`{
				"live": "false",
				"notificationItems": [{
					"NotificationRequestItem": {
						"eventCode": "AUTHORISATION",
						"success": "false",
						"pspReference": "psp_789",
						"merchantReference": "int_111",
						"amount": {"currency": "EUR", "value": 5000},
						"reason": "Refused:51"
					}
				}]
			}`),
			wantType:  domain.EventAuthorizationFailed,
			wantID:    "psp_789",
			wantCount: 1,
		},
		{
			name: "capture success",
			body: []byte(`{
				"live": "true",
				"notificationItems": [{
					"NotificationRequestItem": {
						"eventCode": "CAPTURE",
						"success": "true",
						"pspReference": "psp_cap",
						"originalReference": "psp_auth",
						"merchantReference": "int_cap",
						"amount": {"currency": "USD", "value": 10000}
					}
				}]
			}`),
			wantType:  domain.EventCaptureSucceeded,
			wantID:    "psp_cap",
			wantCount: 1,
		},
		{
			name: "multiple notifications",
			body: []byte(`{
				"live": "false",
				"notificationItems": [
					{
						"NotificationRequestItem": {
							"eventCode": "AUTHORISATION",
							"success": "true",
							"pspReference": "psp_1",
							"merchantReference": "int_1",
							"amount": {"currency": "USD", "value": 1000}
						}
					},
					{
						"NotificationRequestItem": {
							"eventCode": "CAPTURE",
							"success": "true",
							"pspReference": "psp_2",
							"merchantReference": "int_1",
							"amount": {"currency": "USD", "value": 1000}
						}
					}
				]
			}`),
			wantCount: 2,
		},
		{
			name: "empty notifications",
			body: []byte(`{
				"live": "false",
				"notificationItems": []
			}`),
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := adapter.ParseWebhook(context.Background(), http.Header{}, tt.body)
			if err != nil {
				t.Fatalf("ParseWebhook() error = %v", err)
			}

			if len(events) != tt.wantCount {
				t.Fatalf("ParseWebhook() returned %d events, want %d", len(events), tt.wantCount)
			}

			if tt.wantCount > 0 && tt.wantType != "" {
				if events[0].Type != tt.wantType {
					t.Errorf("ParseWebhook() Type = %v, want %v", events[0].Type, tt.wantType)
				}
				if events[0].ID != tt.wantID {
					t.Errorf("ParseWebhook() ID = %v, want %v", events[0].ID, tt.wantID)
				}
				if events[0].Provider != domain.ProviderAdyen {
					t.Errorf("ParseWebhook() Provider = %v, want %v", events[0].Provider, domain.ProviderAdyen)
				}
			}
		})
	}
}

func TestAdyenAdapter_FormatResponse(t *testing.T) {
	adapter := NewAdyenAdapter("test_key")
	code, body := adapter.FormatResponse()

	if code != http.StatusOK {
		t.Errorf("FormatResponse() code = %v, want %v", code, http.StatusOK)
	}
	if body != "[accepted]" {
		t.Errorf("FormatResponse() body = %v, want [accepted]", body)
	}
}

func TestAdyenAdapter_MapEventType(t *testing.T) {
	adapter := NewAdyenAdapter("test_key")

	tests := []struct {
		eventCode string
		success   string
		wantType  domain.CanonicalEventType
	}{
		{"AUTHORISATION", "true", domain.EventAuthorizationSucceeded},
		{"AUTHORISATION", "false", domain.EventAuthorizationFailed},
		{"CAPTURE", "true", domain.EventCaptureSucceeded},
		{"CAPTURE", "false", domain.EventCaptureFailed},
		{"CAPTURE_FAILED", "false", domain.EventCaptureFailed},
		{"CANCELLATION", "true", domain.EventVoidSucceeded},
		{"REFUND", "true", domain.EventRefundSucceeded},
		{"REFUND", "false", domain.EventRefundFailed},
		{"CHARGEBACK", "true", domain.EventDisputeOpened},
		{"CHARGEBACK_REVERSED", "true", domain.EventDisputeClosed},
		{"UNKNOWN", "true", ""},
	}

	for _, tt := range tests {
		t.Run(tt.eventCode+"_"+tt.success, func(t *testing.T) {
			got := adapter.mapEventType(tt.eventCode, tt.success)
			if got != tt.wantType {
				t.Errorf("mapEventType(%s, %s) = %v, want %v", tt.eventCode, tt.success, got, tt.wantType)
			}
		})
	}
}

func TestMapAdyenDeclineType(t *testing.T) {
	tests := []struct {
		reason   string
		wantType domain.DeclineType
	}{
		{"Refused:51", domain.DeclineTypeSoft},
		{"Refused:05", domain.DeclineTypeSoft},
		{"Refused:33", domain.DeclineTypeHard},
		{"Refused:14", domain.DeclineTypeHard},
		{"Refused:59", domain.DeclineTypeFraud},
		{"Refused:41", domain.DeclineTypeFraud},
		{"FRAUD", domain.DeclineTypeFraud},
		{"Unknown reason", domain.DeclineTypeSoft}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := mapAdyenDeclineType(tt.reason)
			if got != tt.wantType {
				t.Errorf("mapAdyenDeclineType(%s) = %v, want %v", tt.reason, got, tt.wantType)
			}
		})
	}
}

// computeAdyenSignature computes a valid Adyen HMAC signature for testing
func computeAdyenSignature(body []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(body)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
