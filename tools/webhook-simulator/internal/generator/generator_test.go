package generator

import (
	"encoding/json"
	"testing"
)

func TestStripeGenerator_Generate(t *testing.T) {
	gen := NewStripeGenerator()

	tests := []struct {
		name      string
		eventType string
		data      map[string]any
		wantType  string
		wantErr   bool
	}{
		{
			name:      "payment_intent.succeeded",
			eventType: "payment_intent.succeeded",
			data: map[string]any{
				"payment_id": "pi_test_123",
				"amount":     10000,
				"currency":   "usd",
			},
			wantType: "payment_intent.succeeded",
			wantErr:  false,
		},
		{
			name:      "payment_intent.payment_failed with decline",
			eventType: "payment_intent.payment_failed",
			data: map[string]any{
				"payment_id":   "pi_fail_123",
				"amount":       5000,
				"decline_code": "insufficient_funds",
			},
			wantType: "payment_intent.payment_failed",
			wantErr:  false,
		},
		{
			name:      "charge.captured",
			eventType: "charge.captured",
			data: map[string]any{
				"charge_id": "ch_test_123",
				"amount":    15000,
			},
			wantType: "charge.captured",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := gen.Generate(tt.eventType, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			var result map[string]any
			if err := json.Unmarshal(payload, &result); err != nil {
				t.Errorf("Failed to unmarshal payload: %v", err)
				return
			}

			if result["type"] != tt.wantType {
				t.Errorf("Generate() type = %v, want %v", result["type"], tt.wantType)
			}

			if result["object"] != "event" {
				t.Errorf("Generate() object = %v, want 'event'", result["object"])
			}
		})
	}
}

func TestAdyenGenerator_Generate(t *testing.T) {
	gen := NewAdyenGenerator()

	tests := []struct {
		name      string
		eventType string
		data      map[string]any
		wantErr   bool
	}{
		{
			name:      "AUTHORISATION",
			eventType: "AUTHORISATION",
			data: map[string]any{
				"psp_reference": "ADYEN_123",
				"amount":        10000,
			},
			wantErr: false,
		},
		{
			name:      "CAPTURE",
			eventType: "CAPTURE",
			data: map[string]any{
				"psp_reference":      "ADYEN_CAP_123",
				"original_reference": "ADYEN_123",
				"amount":             10000,
			},
			wantErr: false,
		},
		{
			name:      "CHARGEBACK",
			eventType: "CHARGEBACK",
			data: map[string]any{
				"psp_reference":      "ADYEN_CB_123",
				"original_reference": "ADYEN_123",
				"amount":             10000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := gen.Generate(tt.eventType, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			var result map[string]any
			if err := json.Unmarshal(payload, &result); err != nil {
				t.Errorf("Failed to unmarshal payload: %v", err)
				return
			}

			items, ok := result["notificationItems"].([]any)
			if !ok || len(items) == 0 {
				t.Error("Expected notificationItems array")
				return
			}

			item := items[0].(map[string]any)
			notifItem, ok := item["NotificationRequestItem"].(map[string]any)
			if !ok {
				t.Error("Expected NotificationRequestItem")
				return
			}

			if notifItem["eventCode"] != tt.eventType {
				t.Errorf("Generate() eventCode = %v, want %v", notifItem["eventCode"], tt.eventType)
			}
		})
	}
}

func TestPayPalGenerator_Generate(t *testing.T) {
	gen := NewPayPalGenerator()

	tests := []struct {
		name      string
		eventType string
		data      map[string]any
		wantErr   bool
	}{
		{
			name:      "PAYMENT.CAPTURE.COMPLETED",
			eventType: "PAYMENT.CAPTURE.COMPLETED",
			data: map[string]any{
				"capture_id": "CAP_123",
				"amount":     10000,
			},
			wantErr: false,
		},
		{
			name:      "PAYMENT.AUTHORIZATION.CREATED",
			eventType: "PAYMENT.AUTHORIZATION.CREATED",
			data: map[string]any{
				"authorization_id": "AUTH_123",
				"amount":           5000,
			},
			wantErr: false,
		},
		{
			name:      "CUSTOMER.DISPUTE.CREATED",
			eventType: "CUSTOMER.DISPUTE.CREATED",
			data: map[string]any{
				"dispute_id": "DISP_123",
				"amount":     10000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := gen.Generate(tt.eventType, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			var result map[string]any
			if err := json.Unmarshal(payload, &result); err != nil {
				t.Errorf("Failed to unmarshal payload: %v", err)
				return
			}

			if result["event_type"] != tt.eventType {
				t.Errorf("Generate() event_type = %v, want %v", result["event_type"], tt.eventType)
			}

			if result["resource"] == nil {
				t.Error("Expected resource object")
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	t.Run("get stripe generator", func(t *testing.T) {
		gen, err := registry.Get("stripe")
		if err != nil {
			t.Errorf("Get(stripe) error = %v", err)
		}
		if gen.Provider() != "stripe" {
			t.Errorf("Provider() = %v, want stripe", gen.Provider())
		}
	})

	t.Run("get adyen generator", func(t *testing.T) {
		gen, err := registry.Get("adyen")
		if err != nil {
			t.Errorf("Get(adyen) error = %v", err)
		}
		if gen.Provider() != "adyen" {
			t.Errorf("Provider() = %v, want adyen", gen.Provider())
		}
	})

	t.Run("get paypal generator", func(t *testing.T) {
		gen, err := registry.Get("paypal")
		if err != nil {
			t.Errorf("Get(paypal) error = %v", err)
		}
		if gen.Provider() != "paypal" {
			t.Errorf("Provider() = %v, want paypal", gen.Provider())
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		_, err := registry.Get("unknown")
		if err == nil {
			t.Error("Expected error for unknown provider")
		}
	})
}
