package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// PaymentIntent represents a payment transaction
type PaymentIntent struct {
	ID                string
	IdempotencyKey    string
	CustomerID        string
	Amount            decimal.Decimal
	Currency          string
	Status            PaymentIntentStatus
	CaptureMethod     CaptureMethod
	Provider          Provider
	ProviderPaymentID *string
	PaymentMethodID   *string
	WorkflowID        *string
	Metadata          map[string]any
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CanTransitionTo checks if a status transition is valid
func (pi *PaymentIntent) CanTransitionTo(newStatus PaymentIntentStatus) bool {
	validTransitions := map[PaymentIntentStatus][]PaymentIntentStatus{
		PaymentIntentStatusCreated: {
			PaymentIntentStatusRequiresMethod,
			PaymentIntentStatusRequiresAuth,
			PaymentIntentStatusAuthorized,
			PaymentIntentStatusFailed,
			PaymentIntentStatusCancelled,
		},
		PaymentIntentStatusRequiresMethod: {
			PaymentIntentStatusRequiresAuth,
			PaymentIntentStatusAuthorized,
			PaymentIntentStatusFailed,
			PaymentIntentStatusCancelled,
		},
		PaymentIntentStatusRequiresAuth: {
			PaymentIntentStatusAuthorized,
			PaymentIntentStatusFailed,
			PaymentIntentStatusCancelled,
		},
		PaymentIntentStatusAuthorized: {
			PaymentIntentStatusCaptured,
			PaymentIntentStatusVoided,
			PaymentIntentStatusFailed,
			PaymentIntentStatusCancelled,
		},
		PaymentIntentStatusCaptured: {
			PaymentIntentStatusRecovering,
		},
		PaymentIntentStatusFailed: {
			PaymentIntentStatusRecovering,
		},
		PaymentIntentStatusRecovering: {
			PaymentIntentStatusAuthorized,
			PaymentIntentStatusFailed,
		},
		PaymentIntentStatusCancelled: {},
		PaymentIntentStatusVoided:    {},
	}

	allowed, ok := validTransitions[pi.Status]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the payment intent is in a final state
func (pi *PaymentIntent) IsTerminal() bool {
	return pi.Status == PaymentIntentStatusCaptured ||
		pi.Status == PaymentIntentStatusFailed ||
		pi.Status == PaymentIntentStatusCancelled ||
		pi.Status == PaymentIntentStatusVoided
}
