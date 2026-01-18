package domain

import (
	"time"
)

// PaymentAttempt represents a single attempt to process a payment
type PaymentAttempt struct {
	ID                   string
	PaymentIntentID      string
	AttemptNumber        int
	Status               AttemptStatus
	Provider             Provider
	ProviderResponseCode *string
	CanonicalDeclineCode *string
	DeclineType          *DeclineType
	ProcessorTxnID       *string
	IdempotencyKey       string
	CreatedAt            time.Time
	CompletedAt          *time.Time
}

// IsRetryable checks if the attempt failed with a retryable error
func (pa *PaymentAttempt) IsRetryable() bool {
	if pa.Status != AttemptStatusFailed || pa.DeclineType == nil {
		return false
	}
	return *pa.DeclineType == DeclineTypeSoft || *pa.DeclineType == DeclineTypeTemporary
}

// Duration returns how long the attempt took, if completed
func (pa *PaymentAttempt) Duration() *time.Duration {
	if pa.CompletedAt == nil {
		return nil
	}
	d := pa.CompletedAt.Sub(pa.CreatedAt)
	return &d
}
