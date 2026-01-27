package adapter

import (
	"context"
	"net/http"
	"testing"

	"payment-processing/shared/domain"
)

func TestPayPalAdapter_Provider(t *testing.T) {
	adapter := NewPayPalAdapter("client_id", "client_secret", "webhook_id", "")
	if adapter.Provider() != domain.ProviderPayPal {
		t.Errorf("Provider() = %v, want %v", adapter.Provider(), domain.ProviderPayPal)
	}
}

func TestPayPalAdapter_ParseWebhook(t *testing.T) {
	adapter := NewPayPalAdapter("client_id", "client_secret", "webhook_id", "")

	tests := []struct {
		name      string
		body      []byte
		wantType  domain.CanonicalEventType
		wantID    string
		wantNil   bool
	}{
		{
			name: "authorization created",
			body: []byte(`{
				"id": "WH-123",
				"event_type": "PAYMENT.AUTHORIZATION.CREATED",
				"create_time": "2021-01-01T00:00:00Z",
				"resource": {
					"id": "auth_456",
					"invoice_id": "int_789",
					"amount": {"currency_code": "USD", "value": "100.00"},
					"expiration_time": "2021-01-04T00:00:00Z"
				}
			}`),
			wantType: domain.EventAuthorizationSucceeded,
			wantID:   "WH-123",
		},
		{
			name: "capture completed",
			body: []byte(`{
				"id": "WH-456",
				"event_type": "PAYMENT.CAPTURE.COMPLETED",
				"create_time": "2021-01-01T00:00:00Z",
				"resource": {
					"id": "cap_789",
					"invoice_id": "int_111",
					"amount": {"currency_code": "EUR", "value": "50.00"}
				}
			}`),
			wantType: domain.EventCaptureSucceeded,
			wantID:   "WH-456",
		},
		{
			name: "capture denied",
			body: []byte(`{
				"id": "WH-789",
				"event_type": "PAYMENT.CAPTURE.DENIED",
				"create_time": "2021-01-01T00:00:00Z",
				"resource": {
					"id": "cap_fail",
					"invoice_id": "int_222",
					"amount": {"currency_code": "USD", "value": "75.00"},
					"status_details": {"reason": "INSUFFICIENT_FUNDS"}
				}
			}`),
			wantType: domain.EventCaptureFailed,
			wantID:   "WH-789",
		},
		{
			name: "dispute created",
			body: []byte(`{
				"id": "WH-dispute",
				"event_type": "CUSTOMER.DISPUTE.CREATED",
				"create_time": "2021-01-01T00:00:00Z",
				"resource": {
					"id": "disp_123",
					"reason": "MERCHANDISE_OR_SERVICE_NOT_RECEIVED",
					"dispute_amount": {"currency_code": "USD", "value": "100.00"}
				}
			}`),
			wantType: domain.EventDisputeOpened,
			wantID:   "WH-dispute",
		},
		{
			name: "unknown event type",
			body: []byte(`{
				"id": "WH-unknown",
				"event_type": "UNKNOWN.EVENT",
				"create_time": "2021-01-01T00:00:00Z",
				"resource": {"id": "unknown"}
			}`),
			wantNil: true,
		},
		{
			name: "void succeeded",
			body: []byte(`{
				"id": "WH-void",
				"event_type": "PAYMENT.AUTHORIZATION.VOIDED",
				"create_time": "2021-01-01T00:00:00Z",
				"resource": {
					"id": "void_123",
					"invoice_id": "int_void",
					"amount": {"currency_code": "USD", "value": "100.00"}
				}
			}`),
			wantType: domain.EventVoidSucceeded,
			wantID:   "WH-void",
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
			if events[0].Provider != domain.ProviderPayPal {
				t.Errorf("ParseWebhook() Provider = %v, want %v", events[0].Provider, domain.ProviderPayPal)
			}
		})
	}
}

func TestPayPalAdapter_FormatResponse(t *testing.T) {
	adapter := NewPayPalAdapter("client_id", "client_secret", "webhook_id", "")
	code, body := adapter.FormatResponse()

	if code != http.StatusOK {
		t.Errorf("FormatResponse() code = %v, want %v", code, http.StatusOK)
	}
	if body != "" {
		t.Errorf("FormatResponse() body = %v, want empty", body)
	}
}

func TestPayPalAdapter_VerifySignature_MissingHeaders(t *testing.T) {
	adapter := NewPayPalAdapter("client_id", "client_secret", "webhook_id", "")

	tests := []struct {
		name    string
		headers http.Header
		wantErr error
	}{
		{
			name:    "all headers missing",
			headers: http.Header{},
			wantErr: ErrMissingSignature,
		},
		{
			name: "transmission ID missing",
			headers: func() http.Header {
				h := http.Header{}
				h.Set("PAYPAL-TRANSMISSION-TIME", "2021-01-01T00:00:00Z")
				h.Set("PAYPAL-TRANSMISSION-SIG", "sig")
				return h
			}(),
			wantErr: ErrMissingSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.VerifySignature(context.Background(), tt.headers, []byte(`{}`))
			if err != tt.wantErr {
				t.Errorf("VerifySignature() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMapPayPalDeclineType(t *testing.T) {
	tests := []struct {
		reason   string
		wantType domain.DeclineType
	}{
		{"INSUFFICIENT_FUNDS", domain.DeclineTypeSoft},
		{"INSTRUMENT_DECLINED", domain.DeclineTypeSoft},
		{"DO_NOT_HONOR", domain.DeclineTypeSoft},
		{"CREDIT_CARD_EXPIRED", domain.DeclineTypeHard},
		{"INVALID_ACCOUNT", domain.DeclineTypeHard},
		{"TRANSACTION_REFUSED", domain.DeclineTypeFraud},
		{"PAYER_ACCOUNT_LOCKED", domain.DeclineTypeFraud},
		{"UNKNOWN_REASON", domain.DeclineTypeSoft}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			got := mapPayPalDeclineType(tt.reason)
			if got != tt.wantType {
				t.Errorf("mapPayPalDeclineType(%s) = %v, want %v", tt.reason, got, tt.wantType)
			}
		})
	}
}

func TestPayPalAdapter_DefaultAPIURL(t *testing.T) {
	adapter := NewPayPalAdapter("client_id", "client_secret", "webhook_id", "")
	if adapter.apiURL != paypalDefaultAPIURL {
		t.Errorf("Default apiURL = %v, want %v", adapter.apiURL, paypalDefaultAPIURL)
	}

	customAdapter := NewPayPalAdapter("client_id", "client_secret", "webhook_id", "api-m.paypal.com")
	if customAdapter.apiURL != "api-m.paypal.com" {
		t.Errorf("Custom apiURL = %v, want api-m.paypal.com", customAdapter.apiURL)
	}
}
