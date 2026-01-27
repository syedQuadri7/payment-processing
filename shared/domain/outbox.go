package domain

import (
	"encoding/json"
	"time"
)

// OutboxEventType represents canonical event types
type OutboxEventType string

const (
	OutboxEventPaymentCreated           OutboxEventType = "payment.created"
	OutboxEventPaymentAuthorized        OutboxEventType = "payment.authorized"
	OutboxEventPaymentAuthorizationFailed OutboxEventType = "payment.authorization_failed"
	OutboxEventPaymentCaptured          OutboxEventType = "payment.captured"
	OutboxEventPaymentCaptureFailed     OutboxEventType = "payment.capture_failed"
	OutboxEventPaymentVoided            OutboxEventType = "payment.voided"
	OutboxEventPaymentRefunded          OutboxEventType = "payment.refunded"
	OutboxEventPaymentDisputeOpened     OutboxEventType = "payment.dispute_opened"
	OutboxEventAccountBalanceUpdated    OutboxEventType = "account.balance_updated"
)

// AggregateType represents the type of aggregate in the outbox
type AggregateType string

const (
	AggregateTypePaymentIntent AggregateType = "PaymentIntent"
	AggregateTypeAccount       AggregateType = "Account"
)

// OutboxEvent represents an event to be published via CDC
type OutboxEvent struct {
	ID            string
	AggregateType AggregateType
	AggregateID   string
	EventType     OutboxEventType
	Payload       json.RawMessage
	CreatedAt     time.Time
}

// PaymentEventPayload represents the canonical payload for payment events
type PaymentEventPayload struct {
	EventID       string  `json:"event_id"`
	EventType     string  `json:"event_type"`
	PaymentID     string  `json:"payment_id"`
	Amount        string  `json:"amount,omitempty"`
	Currency      string  `json:"currency,omitempty"`
	Provider      string  `json:"provider,omitempty"`
	Status        string  `json:"status,omitempty"`
	DeclineCode   string  `json:"decline_code,omitempty"`
	DeclineType   string  `json:"decline_type,omitempty"`
	CorrelationID string  `json:"correlation_id,omitempty"`
	Timestamp     string  `json:"timestamp"`
}

// AccountEventPayload represents the canonical payload for account events
type AccountEventPayload struct {
	EventID          string `json:"event_id"`
	EventType        string `json:"event_type"`
	AccountID        string `json:"account_id"`
	LedgerBalance    string `json:"ledger_balance"`
	PendingBalance   string `json:"pending_balance"`
	AvailableBalance string `json:"available_balance"`
	CorrelationID    string `json:"correlation_id,omitempty"`
	Timestamp        string `json:"timestamp"`
}
