package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPaymentIntent_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name      string
		from      PaymentIntentStatus
		to        PaymentIntentStatus
		canChange bool
	}{
		// From CREATED
		{name: "created to requires_method", from: PaymentIntentStatusCreated, to: PaymentIntentStatusRequiresMethod, canChange: true},
		{name: "created to requires_auth", from: PaymentIntentStatusCreated, to: PaymentIntentStatusRequiresAuth, canChange: true},
		{name: "created to authorized", from: PaymentIntentStatusCreated, to: PaymentIntentStatusAuthorized, canChange: true},
		{name: "created to cancelled", from: PaymentIntentStatusCreated, to: PaymentIntentStatusCancelled, canChange: true},
		{name: "created to captured (invalid)", from: PaymentIntentStatusCreated, to: PaymentIntentStatusCaptured, canChange: false},

		// From REQUIRES_METHOD
		{name: "requires_method to requires_auth", from: PaymentIntentStatusRequiresMethod, to: PaymentIntentStatusRequiresAuth, canChange: true},
		{name: "requires_method to cancelled", from: PaymentIntentStatusRequiresMethod, to: PaymentIntentStatusCancelled, canChange: true},

		// From REQUIRES_AUTH
		{name: "requires_auth to authorized", from: PaymentIntentStatusRequiresAuth, to: PaymentIntentStatusAuthorized, canChange: true},
		{name: "requires_auth to failed", from: PaymentIntentStatusRequiresAuth, to: PaymentIntentStatusFailed, canChange: true},
		{name: "requires_auth to cancelled", from: PaymentIntentStatusRequiresAuth, to: PaymentIntentStatusCancelled, canChange: true},

		// From AUTHORIZED
		{name: "authorized to captured", from: PaymentIntentStatusAuthorized, to: PaymentIntentStatusCaptured, canChange: true},
		{name: "authorized to voided", from: PaymentIntentStatusAuthorized, to: PaymentIntentStatusVoided, canChange: true},
		{name: "authorized to failed", from: PaymentIntentStatusAuthorized, to: PaymentIntentStatusFailed, canChange: true},

		// From RECOVERING
		{name: "recovering to authorized", from: PaymentIntentStatusRecovering, to: PaymentIntentStatusAuthorized, canChange: true},
		{name: "recovering to failed", from: PaymentIntentStatusRecovering, to: PaymentIntentStatusFailed, canChange: true},
		{name: "recovering to captured (invalid)", from: PaymentIntentStatusRecovering, to: PaymentIntentStatusCaptured, canChange: false},

		// Terminal states cannot transition
		{name: "captured cannot change", from: PaymentIntentStatusCaptured, to: PaymentIntentStatusVoided, canChange: false},
		{name: "cancelled cannot change", from: PaymentIntentStatusCancelled, to: PaymentIntentStatusRequiresAuth, canChange: false},
		{name: "voided cannot change", from: PaymentIntentStatusVoided, to: PaymentIntentStatusCaptured, canChange: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pi := &PaymentIntent{Status: tt.from}
			if got := pi.CanTransitionTo(tt.to); got != tt.canChange {
				t.Errorf("CanTransitionTo(%s) from %s = %v, want %v", tt.to, tt.from, got, tt.canChange)
			}
		})
	}
}

func TestPaymentIntent_IsTerminal(t *testing.T) {
	tests := []struct {
		status     PaymentIntentStatus
		isTerminal bool
	}{
		{PaymentIntentStatusCreated, false},
		{PaymentIntentStatusRequiresMethod, false},
		{PaymentIntentStatusRequiresAuth, false},
		{PaymentIntentStatusAuthorized, false},
		{PaymentIntentStatusRecovering, false},
		{PaymentIntentStatusCaptured, true},
		{PaymentIntentStatusFailed, true},
		{PaymentIntentStatusCancelled, true},
		{PaymentIntentStatusVoided, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			pi := &PaymentIntent{Status: tt.status}
			if got := pi.IsTerminal(); got != tt.isTerminal {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.isTerminal)
			}
		})
	}
}

func TestPaymentIntent_CanAttachPaymentMethod(t *testing.T) {
	tests := []struct {
		status   PaymentIntentStatus
		expected bool
	}{
		{PaymentIntentStatusCreated, true},
		{PaymentIntentStatusRequiresMethod, true},
		{PaymentIntentStatusRequiresAuth, false},
		{PaymentIntentStatusAuthorized, false},
		{PaymentIntentStatusCaptured, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			pi := &PaymentIntent{Status: tt.status}
			if got := pi.CanAttachPaymentMethod(); got != tt.expected {
				t.Errorf("CanAttachPaymentMethod() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentIntent_CanAuthorize(t *testing.T) {
	methodID := "pm-123"

	tests := []struct {
		name            string
		status          PaymentIntentStatus
		paymentMethodID *string
		expected        bool
	}{
		{"created with method", PaymentIntentStatusCreated, &methodID, true},
		{"requires_auth with method", PaymentIntentStatusRequiresAuth, &methodID, true},
		{"created without method", PaymentIntentStatusCreated, nil, false},
		{"authorized cannot authorize again", PaymentIntentStatusAuthorized, &methodID, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pi := &PaymentIntent{Status: tt.status, PaymentMethodID: tt.paymentMethodID}
			if got := pi.CanAuthorize(); got != tt.expected {
				t.Errorf("CanAuthorize() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentIntent_CanCapture(t *testing.T) {
	tests := []struct {
		status   PaymentIntentStatus
		expected bool
	}{
		{PaymentIntentStatusAuthorized, true},
		{PaymentIntentStatusCreated, false},
		{PaymentIntentStatusCaptured, false},
		{PaymentIntentStatusVoided, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			pi := &PaymentIntent{Status: tt.status}
			if got := pi.CanCapture(); got != tt.expected {
				t.Errorf("CanCapture() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentIntent_CanCancel(t *testing.T) {
	tests := []struct {
		status   PaymentIntentStatus
		expected bool
	}{
		{PaymentIntentStatusCreated, true},
		{PaymentIntentStatusRequiresMethod, true},
		{PaymentIntentStatusRequiresAuth, true},
		{PaymentIntentStatusAuthorized, true},
		{PaymentIntentStatusCaptured, false},
		{PaymentIntentStatusFailed, false},
		{PaymentIntentStatusCancelled, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			pi := &PaymentIntent{Status: tt.status}
			if got := pi.CanCancel(); got != tt.expected {
				t.Errorf("CanCancel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentIntent_Validate(t *testing.T) {
	tests := []struct {
		name        string
		intent      PaymentIntent
		expectError bool
	}{
		{
			name: "valid intent",
			intent: PaymentIntent{
				Amount:         decimal.NewFromInt(100),
				Currency:       "USD",
				CustomerID:     "cust-123",
				IdempotencyKey: "key-123",
			},
			expectError: false,
		},
		{
			name: "zero amount",
			intent: PaymentIntent{
				Amount:         decimal.Zero,
				Currency:       "USD",
				CustomerID:     "cust-123",
				IdempotencyKey: "key-123",
			},
			expectError: true,
		},
		{
			name: "negative amount",
			intent: PaymentIntent{
				Amount:         decimal.NewFromInt(-100),
				Currency:       "USD",
				CustomerID:     "cust-123",
				IdempotencyKey: "key-123",
			},
			expectError: true,
		},
		{
			name: "invalid currency",
			intent: PaymentIntent{
				Amount:         decimal.NewFromInt(100),
				Currency:       "US", // Should be 3 letters
				CustomerID:     "cust-123",
				IdempotencyKey: "key-123",
			},
			expectError: true,
		},
		{
			name: "missing customer",
			intent: PaymentIntent{
				Amount:         decimal.NewFromInt(100),
				Currency:       "USD",
				CustomerID:     "",
				IdempotencyKey: "key-123",
			},
			expectError: true,
		},
		{
			name: "missing idempotency key",
			intent: PaymentIntent{
				Amount:         decimal.NewFromInt(100),
				Currency:       "USD",
				CustomerID:     "cust-123",
				IdempotencyKey: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.intent.Validate()
			if (err != nil) != tt.expectError {
				t.Errorf("Validate() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestPaymentIntent_TransitionTo(t *testing.T) {
	tests := []struct {
		name        string
		from        PaymentIntentStatus
		to          PaymentIntentStatus
		expectError bool
	}{
		{"valid transition", PaymentIntentStatusCreated, PaymentIntentStatusRequiresAuth, false},
		{"invalid transition", PaymentIntentStatusCreated, PaymentIntentStatusCaptured, true},
		{"terminal cannot transition", PaymentIntentStatusCaptured, PaymentIntentStatusVoided, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pi := &PaymentIntent{Status: tt.from}
			err := pi.TransitionTo(tt.to)
			if (err != nil) != tt.expectError {
				t.Errorf("TransitionTo() error = %v, expectError %v", err, tt.expectError)
			}
			if err == nil && pi.Status != tt.to {
				t.Errorf("TransitionTo() status = %v, want %v", pi.Status, tt.to)
			}
		})
	}
}
