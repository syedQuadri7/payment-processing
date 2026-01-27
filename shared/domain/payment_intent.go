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

// CanAttachPaymentMethod returns true if a payment method can be attached
func (pi *PaymentIntent) CanAttachPaymentMethod() bool {
	return pi.Status == PaymentIntentStatusCreated ||
		pi.Status == PaymentIntentStatusRequiresMethod
}

// CanAuthorize returns true if the intent can be submitted for authorization
func (pi *PaymentIntent) CanAuthorize() bool {
	return (pi.Status == PaymentIntentStatusCreated ||
		pi.Status == PaymentIntentStatusRequiresAuth) &&
		pi.PaymentMethodID != nil
}

// CanCapture returns true if the intent can be captured
func (pi *PaymentIntent) CanCapture() bool {
	return pi.Status == PaymentIntentStatusAuthorized
}

// CanVoid returns true if the intent can be voided
func (pi *PaymentIntent) CanVoid() bool {
	return pi.Status == PaymentIntentStatusAuthorized
}

// CanCancel returns true if the intent can be cancelled
func (pi *PaymentIntent) CanCancel() bool {
	return pi.Status == PaymentIntentStatusCreated ||
		pi.Status == PaymentIntentStatusRequiresMethod ||
		pi.Status == PaymentIntentStatusRequiresAuth ||
		pi.Status == PaymentIntentStatusAuthorized
}

// CanRetry returns true if the intent can be retried after a soft decline
func (pi *PaymentIntent) CanRetry() bool {
	return pi.Status == PaymentIntentStatusRecovering ||
		pi.Status == PaymentIntentStatusFailed
}

// RequiresPaymentMethod returns true if a payment method must be attached
func (pi *PaymentIntent) RequiresPaymentMethod() bool {
	return pi.PaymentMethodID == nil &&
		(pi.Status == PaymentIntentStatusCreated ||
			pi.Status == PaymentIntentStatusRequiresMethod)
}

// IsAutoCapture returns true if the payment should be captured automatically
func (pi *PaymentIntent) IsAutoCapture() bool {
	return pi.CaptureMethod == CaptureMethodAutomatic
}

// IsManualCapture returns true if the payment requires manual capture
func (pi *PaymentIntent) IsManualCapture() bool {
	return pi.CaptureMethod == CaptureMethodManual
}

// HasWorkflow returns true if a workflow has been started for this intent
func (pi *PaymentIntent) HasWorkflow() bool {
	return pi.WorkflowID != nil
}

// Validate performs basic validation on the payment intent
func (pi *PaymentIntent) Validate() error {
	if pi.Amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAmount
	}
	if pi.Currency == "" || len(pi.Currency) != 3 {
		return ErrInvalidCurrency
	}
	if pi.CustomerID == "" {
		return ErrMissingCustomer
	}
	if pi.IdempotencyKey == "" {
		return ErrMissingIdempotencyKey
	}
	return nil
}

// TransitionTo attempts to transition to a new status, returning error if invalid
func (pi *PaymentIntent) TransitionTo(newStatus PaymentIntentStatus) error {
	if !pi.CanTransitionTo(newStatus) {
		return &InvalidStateTransitionError{
			From: pi.Status,
			To:   newStatus,
		}
	}
	pi.Status = newStatus
	pi.UpdatedAt = time.Now()
	return nil
}

// PaymentIntent validation errors
var (
	ErrInvalidAmount        = &ValidationError{Field: "amount", Message: "must be greater than zero"}
	ErrInvalidCurrency      = &ValidationError{Field: "currency", Message: "must be a 3-letter ISO code"}
	ErrMissingCustomer      = &ValidationError{Field: "customer_id", Message: "is required"}
	ErrMissingIdempotencyKey = &ValidationError{Field: "idempotency_key", Message: "is required"}
)

// ValidationError represents a field validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + " " + e.Message
}

// InvalidStateTransitionError represents an invalid state transition
type InvalidStateTransitionError struct {
	From PaymentIntentStatus
	To   PaymentIntentStatus
}

func (e *InvalidStateTransitionError) Error() string {
	return "invalid state transition from " + string(e.From) + " to " + string(e.To)
}
