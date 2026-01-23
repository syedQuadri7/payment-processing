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

// CreatePaymentAttemptInput contains data for creating a new attempt record
type CreatePaymentAttemptInput struct {
	ID              string               `json:"id"`
	PaymentIntentID string               `json:"payment_intent_id"`
	AttemptNumber   int                  `json:"attempt_number"`
	Provider        domain.Provider      `json:"provider"`
	IdempotencyKey  string               `json:"idempotency_key"`
}

// CreatePaymentAttemptResult contains the created attempt ID
type CreatePaymentAttemptResult struct {
	AttemptID string `json:"attempt_id"`
}

// CreatePaymentAttempt creates a new payment attempt record in PENDING status
func (a *Activities) CreatePaymentAttempt(ctx context.Context, input CreatePaymentAttemptInput) (*CreatePaymentAttemptResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Creating payment attempt",
		"payment_intent_id", input.PaymentIntentID,
		"attempt_number", input.AttemptNumber,
	)

	if a.PaymentAttemptRepo != nil {
		attempt := &domain.PaymentAttempt{
			ID:              input.ID,
			PaymentIntentID: input.PaymentIntentID,
			AttemptNumber:   input.AttemptNumber,
			Status:          domain.AttemptStatusPending,
			Provider:        input.Provider,
			IdempotencyKey:  input.IdempotencyKey,
		}

		if err := a.PaymentAttemptRepo.Create(ctx, attempt); err != nil {
			return nil, err
		}
	}

	return &CreatePaymentAttemptResult{
		AttemptID: input.ID,
	}, nil
}

// CompletePaymentAttemptInput contains data for completing an attempt
type CompletePaymentAttemptInput struct {
	AttemptID           string                      `json:"attempt_id"`
	Status              domain.AttemptStatus        `json:"status"`
	ProviderResponseCode *string                    `json:"provider_response_code,omitempty"`
	CanonicalDeclineCode *domain.CanonicalDeclineCode `json:"canonical_decline_code,omitempty"`
	DeclineType         *domain.DeclineType         `json:"decline_type,omitempty"`
	ProcessorTxnID      *string                     `json:"processor_txn_id,omitempty"`
}

// CompletePaymentAttempt marks an attempt as completed with result details
func (a *Activities) CompletePaymentAttempt(ctx context.Context, input CompletePaymentAttemptInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Completing payment attempt",
		"attempt_id", input.AttemptID,
		"status", input.Status,
	)

	if a.PaymentAttemptRepo != nil {
		var declineCode *string
		if input.CanonicalDeclineCode != nil {
			code := string(*input.CanonicalDeclineCode)
			declineCode = &code
		}

		if err := a.PaymentAttemptRepo.MarkCompleted(
			ctx,
			input.AttemptID,
			input.Status,
			input.ProviderResponseCode,
			declineCode,
			input.DeclineType,
		); err != nil {
			return err
		}
	}

	return nil
}

// RecordPaymentAttemptInput contains data for recording an attempt (legacy - use Create/Complete instead)
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

// RecordPaymentAttempt records a payment attempt in the database (legacy - use Create/Complete instead)
func (a *Activities) RecordPaymentAttempt(ctx context.Context, input RecordPaymentAttemptInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Recording payment attempt",
		"payment_intent_id", input.PaymentIntentID,
		"attempt_number", input.AttemptNumber,
		"status", input.Status,
	)

	// This is a legacy activity - new code should use CreatePaymentAttempt + CompletePaymentAttempt
	return nil
}

// MarshalEventPayload marshals a canonical event to JSON for outbox
func MarshalEventPayload(event *domain.CanonicalEvent) ([]byte, error) {
	return json.Marshal(event)
}

// ClassifyDeclineInput contains data for decline classification lookup
type ClassifyDeclineInput struct {
	Provider     domain.Provider `json:"provider"`
	ProviderCode string          `json:"provider_code"`
}

// ClassifyDeclineResult contains the classified decline information
type ClassifyDeclineResult struct {
	Found           bool                       `json:"found"`
	CanonicalCode   domain.CanonicalDeclineCode `json:"canonical_code,omitempty"`
	DeclineType     domain.DeclineType         `json:"decline_type,omitempty"`
	RetryEligible   bool                       `json:"retry_eligible"`
	Description     string                     `json:"description,omitempty"`
	SuggestedAction string                     `json:"suggested_action,omitempty"`
}

// ClassifyDecline looks up a provider-specific decline code and returns its canonical classification
func (a *Activities) ClassifyDecline(ctx context.Context, input ClassifyDeclineInput) (*ClassifyDeclineResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Classifying decline code",
		"provider", input.Provider,
		"provider_code", input.ProviderCode,
	)

	// Try database lookup first if repository is available
	if a.DeclineCodeRepo != nil {
		mapping, err := a.DeclineCodeRepo.GetByProviderCode(ctx, input.Provider, input.ProviderCode)
		if err != nil {
			logger.Warn("Failed to lookup decline code from database, falling back to in-memory",
				"error", err,
			)
		} else if mapping != nil {
			description := ""
			if mapping.Description != nil {
				description = *mapping.Description
			}
			suggestedAction := ""
			if mapping.SuggestedAction != nil {
				suggestedAction = *mapping.SuggestedAction
			}
			return &ClassifyDeclineResult{
				Found:           true,
				CanonicalCode:   mapping.ToCanonicalDeclineCode(),
				DeclineType:     mapping.DeclineType,
				RetryEligible:   mapping.RetryEligible,
				Description:     description,
				SuggestedAction: suggestedAction,
			}, nil
		}
	}

	// Fall back to in-memory lookup from domain.CanonicalDeclineCodes
	// This handles cases where the provider code matches the canonical code
	canonicalCode := domain.CanonicalDeclineCode(input.ProviderCode)
	if info, ok := domain.GetDeclineInfo(canonicalCode); ok {
		return &ClassifyDeclineResult{
			Found:           true,
			CanonicalCode:   info.Code,
			DeclineType:     info.Type,
			RetryEligible:   info.RetryEligible,
			Description:     info.Description,
			SuggestedAction: info.SuggestedAction,
		}, nil
	}

	// Unknown decline code - default to soft decline (retry eligible)
	// This is a safe default as it allows retry attempts
	logger.Warn("Unknown decline code, defaulting to soft decline",
		"provider", input.Provider,
		"provider_code", input.ProviderCode,
	)
	return &ClassifyDeclineResult{
		Found:         false,
		DeclineType:   domain.DeclineTypeSoft,
		RetryEligible: true,
		Description:   "Unknown decline code",
	}, nil
}
