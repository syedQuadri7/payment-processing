package outbox

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"payment-processing/pkg/domain"
)

// LoggingHandler is a simple handler that logs events
// Useful for development and debugging
func LoggingHandler() EventHandler {
	return func(ctx context.Context, event *domain.OutboxEvent) error {
		log.Printf("Event: id=%s type=%s aggregate=%s/%s payload=%s",
			event.ID,
			event.EventType,
			event.AggregateType,
			event.AggregateID,
			string(event.Payload),
		)
		return nil
	}
}

// IdempotentHandler wraps a handler to ensure events are only processed once
// Uses in-memory tracking (for demonstration - production should use database)
type IdempotentHandler struct {
	inner     EventHandler
	processed map[string]bool
	mu        sync.RWMutex
}

// NewIdempotentHandler creates a new idempotent handler wrapper
func NewIdempotentHandler(inner EventHandler) *IdempotentHandler {
	return &IdempotentHandler{
		inner:     inner,
		processed: make(map[string]bool),
	}
}

// Handle processes an event idempotently
func (h *IdempotentHandler) Handle(ctx context.Context, event *domain.OutboxEvent) error {
	// Check if already processed
	h.mu.RLock()
	if h.processed[event.ID] {
		h.mu.RUnlock()
		log.Printf("Event %s already processed, skipping", event.ID)
		return nil
	}
	h.mu.RUnlock()

	// Process the event
	if err := h.inner(ctx, event); err != nil {
		return err
	}

	// Mark as processed
	h.mu.Lock()
	h.processed[event.ID] = h.processed[event.ID] || true
	h.mu.Unlock()

	return nil
}

// Handler returns the EventHandler function
func (h *IdempotentHandler) Handler() EventHandler {
	return h.Handle
}

// ProcessedCount returns the number of events processed
func (h *IdempotentHandler) ProcessedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.processed)
}

// PaymentEventHandler processes payment-specific events
type PaymentEventHandler struct {
	// OnPaymentAuthorized is called when a payment is authorized
	OnPaymentAuthorized func(ctx context.Context, payload *domain.PaymentEventPayload) error

	// OnPaymentCaptured is called when a payment is captured
	OnPaymentCaptured func(ctx context.Context, payload *domain.PaymentEventPayload) error

	// OnPaymentVoided is called when a payment is voided
	OnPaymentVoided func(ctx context.Context, payload *domain.PaymentEventPayload) error

	// OnPaymentRefunded is called when a payment is refunded
	OnPaymentRefunded func(ctx context.Context, payload *domain.PaymentEventPayload) error

	// OnPaymentFailed is called when a payment fails
	OnPaymentFailed func(ctx context.Context, payload *domain.PaymentEventPayload) error
}

// Handle routes events to the appropriate handler
func (h *PaymentEventHandler) Handle(ctx context.Context, event *domain.OutboxEvent) error {
	// Only handle payment events
	if event.AggregateType != domain.AggregateTypePaymentIntent {
		return nil
	}

	// Parse the payload
	var payload domain.PaymentEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		log.Printf("Failed to parse payment event payload: %v", err)
		return nil // Don't fail on parse errors - log and continue
	}

	// Route to appropriate handler
	switch event.EventType {
	case domain.OutboxEventPaymentAuthorized:
		if h.OnPaymentAuthorized != nil {
			return h.OnPaymentAuthorized(ctx, &payload)
		}
	case domain.OutboxEventPaymentCaptured:
		if h.OnPaymentCaptured != nil {
			return h.OnPaymentCaptured(ctx, &payload)
		}
	case domain.OutboxEventPaymentVoided:
		if h.OnPaymentVoided != nil {
			return h.OnPaymentVoided(ctx, &payload)
		}
	case domain.OutboxEventPaymentRefunded:
		if h.OnPaymentRefunded != nil {
			return h.OnPaymentRefunded(ctx, &payload)
		}
	case domain.OutboxEventPaymentAuthorizationFailed, domain.OutboxEventPaymentCaptureFailed:
		if h.OnPaymentFailed != nil {
			return h.OnPaymentFailed(ctx, &payload)
		}
	}

	return nil
}

// Handler returns the EventHandler function
func (h *PaymentEventHandler) Handler() EventHandler {
	return h.Handle
}

// ChainHandlers combines multiple handlers into a single handler
// Each handler is called in order; if one fails, the chain stops
func ChainHandlers(handlers ...EventHandler) EventHandler {
	return func(ctx context.Context, event *domain.OutboxEvent) error {
		for _, h := range handlers {
			if err := h(ctx, event); err != nil {
				return err
			}
		}
		return nil
	}
}
