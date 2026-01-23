package domain

import (
	"testing"
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
