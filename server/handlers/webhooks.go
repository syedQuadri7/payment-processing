package handlers

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"payment-processing/internal/adapter"
	"payment-processing/pkg/domain"
	"payment-processing/server"
	"payment-processing/server/middleware"
	"payment-processing/workflow"
)

// ProcessedEventRepository defines the interface for tracking processed webhook events
type ProcessedEventRepository interface {
	Exists(ctx context.Context, provider domain.Provider, eventID string) (bool, error)
	Create(ctx context.Context, event *domain.ProcessedEvent) error
}

// WebhookHandler handles webhook requests from payment providers
type WebhookHandler struct {
	registry       *adapter.AdapterRegistry
	intentRepo     PaymentIntentRepository
	processedRepo  ProcessedEventRepository
	temporal       client.Client
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(
	registry *adapter.AdapterRegistry,
	intentRepo PaymentIntentRepository,
	processedRepo ProcessedEventRepository,
	temporal client.Client,
) *WebhookHandler {
	return &WebhookHandler{
		registry:      registry,
		intentRepo:    intentRepo,
		processedRepo: processedRepo,
		temporal:      temporal,
	}
}

// Stripe handles POST /webhooks/stripe
func (h *WebhookHandler) Stripe(w http.ResponseWriter, r *http.Request) {
	h.handleWebhook(w, r, domain.ProviderStripe)
}

// Adyen handles POST /webhooks/adyen
func (h *WebhookHandler) Adyen(w http.ResponseWriter, r *http.Request) {
	h.handleWebhook(w, r, domain.ProviderAdyen)
}

// PayPal handles POST /webhooks/paypal
func (h *WebhookHandler) PayPal(w http.ResponseWriter, r *http.Request) {
	h.handleWebhook(w, r, domain.ProviderPayPal)
}

// handleWebhook processes a webhook for any provider
func (h *WebhookHandler) handleWebhook(w http.ResponseWriter, r *http.Request, provider domain.Provider) {
	requestID := middleware.GetRequestID(r.Context())

	// Only POST allowed
	if r.Method != http.MethodPost {
		server.WriteError(w, server.NewMethodNotAllowedError(r.Method), requestID)
		return
	}

	// Get adapter for this provider
	webhookAdapter, ok := h.registry.GetAdapter(provider)
	if !ok {
		// Provider not configured - return success to avoid retries
		// Log this as it may indicate misconfiguration
		resp := server.WebhookResponse{
			Received: true,
			Message:  "provider not configured",
		}
		server.WriteJSON(w, http.StatusOK, resp, requestID)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		server.WriteError(w, server.NewValidationError("body", "failed to read request body"), requestID)
		return
	}

	// Verify signature
	if err := webhookAdapter.VerifySignature(r.Context(), r.Header, body); err != nil {
		// Log signature verification failure for security monitoring
		// Return 401 to indicate authentication failure
		server.WriteError(w, server.NewAuthenticationError("webhook signature verification failed"), requestID)
		return
	}

	// Parse webhook into canonical events
	events, err := webhookAdapter.ParseWebhook(r.Context(), r.Header, body)
	if err != nil {
		// Log parsing failure
		// Still return success to provider to prevent retries for malformed payloads
		statusCode, responseBody := webhookAdapter.FormatResponse()
		if responseBody != "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(statusCode)
			w.Write([]byte(responseBody))
		} else {
			resp := server.WebhookResponse{
				Received: true,
				Message:  "parse error",
			}
			server.WriteJSON(w, statusCode, resp, requestID)
		}
		return
	}

	// No events to process (unknown event type)
	if len(events) == 0 {
		statusCode, responseBody := webhookAdapter.FormatResponse()
		if responseBody != "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(statusCode)
			w.Write([]byte(responseBody))
		} else {
			resp := server.WebhookResponse{
				Received: true,
				Message:  "event type not handled",
			}
			server.WriteJSON(w, statusCode, resp, requestID)
		}
		return
	}

	// Process each canonical event
	for _, event := range events {
		if err := h.processEvent(r.Context(), event); err != nil {
			// Log error but continue processing other events
			// Individual event failures shouldn't fail the whole webhook
			continue
		}
	}

	// Return provider-specific success response
	statusCode, responseBody := webhookAdapter.FormatResponse()
	if responseBody != "" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(statusCode)
		w.Write([]byte(responseBody))
	} else {
		resp := server.WebhookResponse{
			Received: true,
		}
		server.WriteJSON(w, statusCode, resp, requestID)
	}
}

// processEvent processes a single canonical event
func (h *WebhookHandler) processEvent(ctx context.Context, event *domain.CanonicalEvent) error {
	// Check for duplicate event (idempotency)
	exists, err := h.processedRepo.Exists(ctx, event.Provider, event.ID)
	if err != nil {
		// Log error but continue - better to process duplicate than miss event
	}
	if exists {
		// Already processed, skip
		return nil
	}

	// Find the payment intent to signal
	var workflowID string

	if event.PaymentIntentID != nil {
		// Payment intent ID provided in event metadata
		pi, err := h.intentRepo.GetByID(ctx, *event.PaymentIntentID)
		if err != nil {
			return err
		}
		if pi != nil && pi.WorkflowID != nil {
			workflowID = *pi.WorkflowID
		}
	}

	// If no payment intent ID in metadata, we can't signal a workflow
	// This might happen for dispute events or other events that don't have our internal ID
	if workflowID == "" {
		// Log that we couldn't find the workflow to signal
		// Still mark as processed to avoid retries
	}

	// Signal the workflow with the canonical event
	if workflowID != "" {
		err = h.temporal.SignalWorkflow(
			ctx,
			workflowID,
			"",
			workflow.SignalWebhookEvent,
			*event,
		)
		if err != nil {
			// Log workflow signal failure
			// Don't return error - we still want to mark as processed
		}
	}

	// Mark event as processed
	processedEvent := &domain.ProcessedEvent{
		ID:          uuid.New().String(),
		Provider:    event.Provider,
		EventID:     event.ID,
		EventType:   string(event.Type),
		ProcessedAt: time.Now(),
	}

	if err := h.processedRepo.Create(ctx, processedEvent); err != nil {
		// Log error but don't fail - duplicate key errors are expected
		// if the event was processed concurrently
	}

	return nil
}
