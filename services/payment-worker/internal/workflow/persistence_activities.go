package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.temporal.io/sdk/activity"

	"payment-processing/shared/domain"
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

	if a.OutboxRepo == nil {
		logger.Warn("Outbox repository not configured, skipping outbox write")
		return nil
	}

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("marshaling outbox payload: %w", err)
	}

	// Use idempotency key as event ID to prevent duplicates
	event := &domain.OutboxEvent{
		ID:            input.IdempotencyKey,
		AggregateType: domain.AggregateType(input.AggregateType),
		AggregateID:   input.AggregateID,
		EventType:     domain.OutboxEventType(input.EventType),
		Payload:       payloadBytes,
	}

	if err := a.OutboxRepo.Create(ctx, event); err != nil {
		// Check for duplicate key error (idempotent)
		if isDuplicateKeyError(err) {
			logger.Info("Outbox event already exists (idempotent)",
				"event_id", input.IdempotencyKey,
			)
			return nil
		}
		return fmt.Errorf("creating outbox event: %w", err)
	}

	logger.Info("Outbox event created",
		"event_id", input.IdempotencyKey,
	)
	return nil
}

// isDuplicateKeyError checks if the error is a PostgreSQL duplicate key violation
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// Check for PostgreSQL duplicate key error code (23505)
	errMsg := err.Error()
	return strings.Contains(errMsg, "duplicate key") || strings.Contains(errMsg, "23505")
}

// WriteAuditLogInput contains data for audit log writes
type WriteAuditLogInput struct {
	EntityType    string         `json:"entity_type"`
	EntityID      string         `json:"entity_id"`
	Action        string         `json:"action"`
	OldValues     map[string]any `json:"old_values,omitempty"`
	NewValues     map[string]any `json:"new_values,omitempty"`
	ActorID       *string        `json:"actor_id,omitempty"`
	ActorType     string         `json:"actor_type"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Provider      string         `json:"provider,omitempty"`
	WorkflowID    string         `json:"workflow_id,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
}

// WriteAuditLog writes an entry to the audit log
func (a *Activities) WriteAuditLog(ctx context.Context, input WriteAuditLogInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Writing audit log",
		"entity_type", input.EntityType,
		"entity_id", input.EntityID,
		"action", input.Action,
	)

	if a.AuditLogRepo == nil {
		logger.Warn("Audit log repository not configured, skipping audit write")
		return nil
	}

	// Marshal old values
	var oldValuesJSON json.RawMessage
	if input.OldValues != nil {
		bytes, err := json.Marshal(input.OldValues)
		if err != nil {
			return fmt.Errorf("marshaling old values: %w", err)
		}
		oldValuesJSON = bytes
	}

	// Marshal new values
	var newValuesJSON json.RawMessage
	if input.NewValues != nil {
		bytes, err := json.Marshal(input.NewValues)
		if err != nil {
			return fmt.Errorf("marshaling new values: %w", err)
		}
		newValuesJSON = bytes
	}

	// Marshal metadata
	metadata := domain.AuditMetadata{
		CorrelationID: input.CorrelationID,
		Provider:      input.Provider,
		WorkflowID:    input.WorkflowID,
		RequestID:     input.RequestID,
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Generate unique ID for the audit entry
	activityInfo := activity.GetInfo(ctx)
	entryID := fmt.Sprintf("audit-%s-%s-%d",
		input.EntityID,
		input.Action,
		activityInfo.Attempt,
	)

	entry := &domain.AuditLogEntry{
		ID:         entryID,
		EntityType: domain.AuditEntityType(input.EntityType),
		EntityID:   input.EntityID,
		Action:     domain.AuditAction(input.Action),
		ActorType:  domain.AuditActorType(input.ActorType),
		ActorID:    input.ActorID,
		OldValues:  oldValuesJSON,
		NewValues:  newValuesJSON,
		Metadata:   metadataBytes,
	}

	if err := a.AuditLogRepo.Create(ctx, entry); err != nil {
		// Check for duplicate key error (idempotent)
		if isDuplicateKeyError(err) {
			logger.Info("Audit log entry already exists (idempotent)",
				"entry_id", entryID,
			)
			return nil
		}
		return fmt.Errorf("creating audit log entry: %w", err)
	}

	logger.Info("Audit log entry created",
		"entry_id", entryID,
	)
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
