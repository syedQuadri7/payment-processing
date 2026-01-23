package workflow

import (
	"time"

	"github.com/shopspring/decimal"

	"payment-processing/pkg/domain"
)

// PaymentState tracks the current state of a payment workflow
// This is used for queries and internal state management
type PaymentState struct {
	PaymentIntentID string                     `json:"payment_intent_id"`
	Status          domain.PaymentIntentStatus `json:"status"`
	Provider        domain.Provider            `json:"provider"`
	Amount          decimal.Decimal            `json:"amount"`
	Currency        string                     `json:"currency"`
	CaptureMethod   domain.CaptureMethod       `json:"capture_method"`

	// Provider references
	ProviderPaymentID *string    `json:"provider_payment_id,omitempty"`
	AuthorizationCode *string    `json:"authorization_code,omitempty"`
	AuthExpiresAt     *time.Time `json:"auth_expires_at,omitempty"`

	// Capture tracking
	CapturedAmount *decimal.Decimal `json:"captured_amount,omitempty"`
	CapturedAt     *time.Time       `json:"captured_at,omitempty"`

	// Decline/Error info
	LastDeclineCode  *string             `json:"last_decline_code,omitempty"`
	LastDeclineType  *domain.DeclineType `json:"last_decline_type,omitempty"`
	LastErrorMessage *string             `json:"last_error_message,omitempty"`

	// Attempt tracking
	CurrentAttempt int             `json:"current_attempt"`
	Attempts       []AttemptRecord `json:"attempts"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AttemptRecord tracks individual payment attempts
type AttemptRecord struct {
	AttemptNumber int                 `json:"attempt_number"`
	Status        domain.AttemptStatus `json:"status"`
	DeclineCode   *string             `json:"decline_code,omitempty"`
	DeclineType   *domain.DeclineType `json:"decline_type,omitempty"`
	ErrorMessage  *string             `json:"error_message,omitempty"`
	StartedAt     time.Time           `json:"started_at"`
	CompletedAt   *time.Time          `json:"completed_at,omitempty"`
}

// CanTransitionTo validates state transitions
func (s *PaymentState) CanTransitionTo(newStatus domain.PaymentIntentStatus) bool {
	validTransitions := map[domain.PaymentIntentStatus][]domain.PaymentIntentStatus{
		domain.PaymentIntentStatusCreated: {
			domain.PaymentIntentStatusRequiresMethod,
			domain.PaymentIntentStatusRequiresAuth,
			domain.PaymentIntentStatusAuthorized,
			domain.PaymentIntentStatusFailed,
			domain.PaymentIntentStatusCancelled,
		},
		domain.PaymentIntentStatusRequiresMethod: {
			domain.PaymentIntentStatusRequiresAuth,
			domain.PaymentIntentStatusAuthorized,
			domain.PaymentIntentStatusFailed,
			domain.PaymentIntentStatusCancelled,
		},
		domain.PaymentIntentStatusRequiresAuth: {
			domain.PaymentIntentStatusAuthorized,
			domain.PaymentIntentStatusFailed,
			domain.PaymentIntentStatusCancelled,
		},
		domain.PaymentIntentStatusAuthorized: {
			domain.PaymentIntentStatusCaptured,
			domain.PaymentIntentStatusVoided,
			domain.PaymentIntentStatusFailed,
			domain.PaymentIntentStatusCancelled,
		},
		domain.PaymentIntentStatusCaptured: {
			domain.PaymentIntentStatusRecovering,
		},
		domain.PaymentIntentStatusFailed: {
			domain.PaymentIntentStatusRecovering,
		},
		domain.PaymentIntentStatusRecovering: {
			domain.PaymentIntentStatusAuthorized,
			domain.PaymentIntentStatusFailed,
		},
		domain.PaymentIntentStatusCancelled: {},
		domain.PaymentIntentStatusVoided:    {},
	}

	allowed, ok := validTransitions[s.Status]
	if !ok {
		return false
	}

	for _, status := range allowed {
		if status == newStatus {
			return true
		}
	}
	return false
}

// IsTerminal returns true if the payment is in a final state
func (s *PaymentState) IsTerminal() bool {
	return s.Status == domain.PaymentIntentStatusCaptured ||
		s.Status == domain.PaymentIntentStatusFailed ||
		s.Status == domain.PaymentIntentStatusCancelled ||
		s.Status == domain.PaymentIntentStatusVoided
}

// CanRetry returns true if another retry attempt is allowed
func (s *PaymentState) CanRetry() bool {
	if s.LastDeclineType == nil {
		return false
	}
	// Only soft and temporary declines are retryable
	if *s.LastDeclineType != domain.DeclineTypeSoft && *s.LastDeclineType != domain.DeclineTypeTemporary {
		return false
	}
	// Check if we have retries remaining
	return s.CurrentAttempt < len(RetryIntervals)+1
}

// NextRetryDelay returns the delay before the next retry attempt
func (s *PaymentState) NextRetryDelay() time.Duration {
	if s.CurrentAttempt <= 0 || s.CurrentAttempt > len(RetryIntervals) {
		return 0
	}
	return RetryIntervals[s.CurrentAttempt-1]
}
