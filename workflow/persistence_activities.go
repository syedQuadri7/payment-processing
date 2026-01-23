package workflow

import (
	"context"
	"encoding/json"

	"go.temporal.io/sdk/activity"

	"payment-processing/internal/domain"
)

// PersistPaymentStateInput contains data for persistence
type PersistPaymentStateInput struct {
	PaymentIntentID   string                     `json:"payment_intent_id"`
	Status            domain.PaymentIntentStatus `json:"status"`
	ProviderPaymentID *string                    `json:"provider_payment_id,omitempty"`
	Attempt           *AttemptRecord             `json:"attempt,omitempty"`
}

// PersistPaymentState saves the payment state to the database
func (a *Activities) PersistPaymentState(ctx context.Context, input PersistPaymentStateInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Persisting payment state",
		"payment_intent_id", input.PaymentIntentID,
		"status", input.Status,
	)

	// TODO: Use repository to persist state
	// This should be idempotent using optimistic locking
	return nil
}

// WriteOutboxEventInput contains data for outbox writes
type WriteOutboxEventInput struct {
	EventType      string         `json:"event_type"`
	AggregateType  string         `json:"aggregate_type"`
	AggregateID    string         `json:"aggregate_id"`
	Payload        map[string]any `json:"payload"`
	IdempotencyKey string         `json:"idempotency_key"`
}

// WriteOutboxEvent writes an event to the outbox for CDC
func (a *Activities) WriteOutboxEvent(ctx context.Context, input WriteOutboxEventInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Writing outbox event",
		"event_type", input.EventType,
		"aggregate_type", input.AggregateType,
		"aggregate_id", input.AggregateID,
	)

	// TODO: Write to outbox table
	return nil
}

// WriteAuditLogInput contains data for audit log writes
type WriteAuditLogInput struct {
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Action     string         `json:"action"`
	Changes    map[string]any `json:"changes,omitempty"`
	ActorID    *string        `json:"actor_id,omitempty"`
	ActorType  string         `json:"actor_type"`
}

// WriteAuditLog writes an entry to the audit log
func (a *Activities) WriteAuditLog(ctx context.Context, input WriteAuditLogInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Writing audit log",
		"entity_type", input.EntityType,
		"entity_id", input.EntityID,
		"action", input.Action,
	)

	// TODO: Write to audit log table
	return nil
}

// GetPaymentIntentInput contains data for fetching payment intent
type GetPaymentIntentInput struct {
	PaymentIntentID string `json:"payment_intent_id"`
}

// GetPaymentIntentResult contains the fetched payment intent
type GetPaymentIntentResult struct {
	PaymentIntent domain.PaymentIntent `json:"payment_intent"`
	Exists        bool                 `json:"exists"`
}

// GetPaymentIntent fetches the current payment intent from the database
func (a *Activities) GetPaymentIntent(ctx context.Context, input GetPaymentIntentInput) (*GetPaymentIntentResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Getting payment intent", "payment_intent_id", input.PaymentIntentID)

	// TODO: Fetch from repository
	return &GetPaymentIntentResult{
		Exists: false,
	}, nil
}

// RecordPaymentAttemptInput contains data for recording an attempt
type RecordPaymentAttemptInput struct {
	PaymentIntentID string               `json:"payment_intent_id"`
	AttemptNumber   int                  `json:"attempt_number"`
	Status          domain.AttemptStatus `json:"status"`
	Provider        domain.Provider      `json:"provider"`
	DeclineCode     *string              `json:"decline_code,omitempty"`
	DeclineType     *domain.DeclineType  `json:"decline_type,omitempty"`
	ProcessorTxnID  *string              `json:"processor_txn_id,omitempty"`
	ErrorMessage    *string              `json:"error_message,omitempty"`
	IdempotencyKey  string               `json:"idempotency_key"`
}

// RecordPaymentAttempt records a payment attempt in the database
func (a *Activities) RecordPaymentAttempt(ctx context.Context, input RecordPaymentAttemptInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Recording payment attempt",
		"payment_intent_id", input.PaymentIntentID,
		"attempt_number", input.AttemptNumber,
		"status", input.Status,
	)

	// TODO: Write to payment_attempts table
	return nil
}

// MarshalEventPayload marshals a canonical event to JSON for outbox
func MarshalEventPayload(event *domain.CanonicalEvent) ([]byte, error) {
	return json.Marshal(event)
}
