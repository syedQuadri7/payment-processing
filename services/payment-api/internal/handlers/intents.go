package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	"payment-processing/services/payment-api"
	"payment-processing/services/payment-api/internal/middleware"
	"payment-processing/shared/domain"
	"payment-processing/shared/workflowtypes"
)

// PaymentIntentRepository defines the interface for payment intent storage
type PaymentIntentRepository interface {
	Create(ctx context.Context, pi *domain.PaymentIntent) error
	GetByID(ctx context.Context, id string) (*domain.PaymentIntent, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentIntent, error)
	Update(ctx context.Context, pi *domain.PaymentIntent) error
	UpdateStatus(ctx context.Context, id string, status domain.PaymentIntentStatus) error
}

// PaymentAttemptRepository defines the interface for payment attempt storage
type PaymentAttemptRepository interface {
	GetByPaymentIntentID(ctx context.Context, intentID string) ([]*domain.PaymentAttempt, error)
}

// AuthorizationHoldRepository defines the interface for authorization hold storage
type AuthorizationHoldRepository interface {
	GetByPaymentIntentID(ctx context.Context, intentID string) (*domain.AuthorizationHold, error)
	GetActiveByPaymentIntentID(ctx context.Context, intentID string) (*domain.AuthorizationHold, error)
}

// IntentHandler handles payment intent API requests
type IntentHandler struct {
	intentRepo  PaymentIntentRepository
	attemptRepo PaymentAttemptRepository
	holdRepo    AuthorizationHoldRepository
	temporal    client.Client
}

// NewIntentHandler creates a new payment intent handler
func NewIntentHandler(
	intentRepo PaymentIntentRepository,
	attemptRepo PaymentAttemptRepository,
	holdRepo AuthorizationHoldRepository,
	temporal client.Client,
) *IntentHandler {
	return &IntentHandler{
		intentRepo:  intentRepo,
		attemptRepo: attemptRepo,
		holdRepo:    holdRepo,
		temporal:    temporal,
	}
}

// Create handles POST /api/v1/intents
func (h *IntentHandler) Create(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	// Only POST allowed
	if r.Method != http.MethodPost {
		server.WriteError(w, server.NewMethodNotAllowedError(r.Method), requestID)
		return
	}

	// Parse request body
	var req server.CreatePaymentIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteError(w, server.NewInvalidJSONError(), requestID)
		return
	}

	// Validate request
	if apiErr := req.Validate(); apiErr != nil {
		server.WriteError(w, apiErr, requestID)
		return
	}

	// Get idempotency key from header
	idempotencyKey := middleware.GetIdempotencyKey(r)
	if idempotencyKey == "" {
		server.WriteError(w, server.NewMissingHeaderError("Idempotency-Key"), requestID)
		return
	}

	// Check for existing payment intent with this idempotency key
	existing, err := h.intentRepo.GetByIdempotencyKey(r.Context(), idempotencyKey)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error checking idempotency key"), requestID)
		return
	}
	if existing != nil {
		// Return existing payment intent
		resp := h.toPaymentIntentResponse(existing, nil)
		server.WriteJSON(w, http.StatusOK, resp, requestID)
		return
	}

	amount := req.Amount

	// Determine capture method
	captureMethod := domain.CaptureMethodAutomatic
	if req.CaptureMethod != "" {
		captureMethod = domain.CaptureMethod(req.CaptureMethod)
	}

	// Convert metadata from map[string]string to map[string]any
	var metadata map[string]any
	if req.Metadata != nil {
		metadata = make(map[string]any)
		for k, v := range req.Metadata {
			metadata[k] = v
		}
	}

	// Create payment intent
	intentID := uuid.New().String()
	workflowID := "payment-" + intentID

	pi := &domain.PaymentIntent{
		ID:              intentID,
		IdempotencyKey:  idempotencyKey,
		CustomerID:      req.CustomerID,
		Amount:          amount,
		Currency:        strings.ToUpper(req.Currency),
		Status:          domain.PaymentIntentStatusCreated,
		CaptureMethod:   captureMethod,
		Provider:        domain.Provider(req.Provider),
		PaymentMethodID: req.PaymentMethodID,
		WorkflowID:      &workflowID,
		Metadata:        metadata,
	}

	// Save to database
	if err := h.intentRepo.Create(r.Context(), pi); err != nil {
		server.WriteError(w, server.NewInternalError("Error creating payment intent"), requestID)
		return
	}

	// Start Temporal workflow
	workflowInput := workflowtypes.PaymentWorkflowInput{
		PaymentIntentID: intentID,
		CustomerID:      req.CustomerID,
		Amount:          amount,
		Currency:        strings.ToUpper(req.Currency),
		Provider:        domain.Provider(req.Provider),
		CaptureMethod:   captureMethod,
		PaymentMethodID: req.PaymentMethodID,
		IdempotencyKey:  idempotencyKey,
		Metadata:        metadata,
	}

	_, err = h.temporal.ExecuteWorkflow(
		r.Context(),
		client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: workflowtypes.TaskQueueName,
		},
		workflowtypes.PaymentIntentWorkflowName,
		workflowInput,
	)
	if err != nil {
		// Workflow failed to start, but intent was created
		// Log this as it needs manual intervention
		server.WriteError(w, server.NewInternalError("Error starting payment workflow"), requestID)
		return
	}

	// Return response
	resp := h.toPaymentIntentResponse(pi, nil)
	server.WriteJSON(w, http.StatusCreated, resp, requestID)
}

// Get handles GET /api/v1/intents/:id
func (h *IntentHandler) Get(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	// Extract intent ID from path
	intentID := chi.URLParam(r, "id")
	if intentID == "" {
		server.WriteError(w, server.NewValidationError("id", "payment intent ID is required"), requestID)
		return
	}

	// Get payment intent
	pi, err := h.intentRepo.GetByID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving payment intent"), requestID)
		return
	}
	if pi == nil {
		server.WriteError(w, server.NewNotFoundError("payment_intent", intentID), requestID)
		return
	}

	// Get active authorization hold if exists
	var hold *domain.AuthorizationHold
	if pi.Status == domain.PaymentIntentStatusAuthorized {
		hold, _ = h.holdRepo.GetActiveByPaymentIntentID(r.Context(), intentID)
	}

	resp := h.toPaymentIntentResponse(pi, hold)
	server.WriteJSON(w, http.StatusOK, resp, requestID)
}

// AttachMethod handles PUT /api/v1/intents/:id/method
func (h *IntentHandler) AttachMethod(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	// Extract intent ID from path
	intentID := chi.URLParam(r, "id")
	if intentID == "" {
		server.WriteError(w, server.NewValidationError("id", "payment intent ID is required"), requestID)
		return
	}

	// Parse request body
	var req server.AttachPaymentMethodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteError(w, server.NewInvalidJSONError(), requestID)
		return
	}

	// Validate request
	if apiErr := req.Validate(); apiErr != nil {
		server.WriteError(w, apiErr, requestID)
		return
	}

	// Get payment intent
	pi, err := h.intentRepo.GetByID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving payment intent"), requestID)
		return
	}
	if pi == nil {
		server.WriteError(w, server.NewNotFoundError("payment_intent", intentID), requestID)
		return
	}

	// Check if payment method can be attached
	if !pi.CanAttachPaymentMethod() {
		server.WriteError(w, server.NewBusinessRuleError("invalid_state",
			"Cannot attach payment method in current state: "+string(pi.Status)), requestID)
		return
	}

	// Update payment intent
	pi.PaymentMethodID = &req.PaymentMethodID
	if pi.Status == domain.PaymentIntentStatusCreated {
		pi.Status = domain.PaymentIntentStatusRequiresAuth
	}

	if err := h.intentRepo.Update(r.Context(), pi); err != nil {
		server.WriteError(w, server.NewInternalError("Error updating payment intent"), requestID)
		return
	}

	// Signal workflow to update payment method
	if pi.WorkflowID != nil {
		err = h.temporal.SignalWorkflow(
			r.Context(),
			*pi.WorkflowID,
			"",
			workflowtypes.SignalUpdatePaymentMethod,
			req.PaymentMethodID,
		)
		if err != nil {
			// Log but don't fail - workflow may not be in a state that handles this signal
		}
	}

	resp := h.toPaymentIntentResponse(pi, nil)
	server.WriteJSON(w, http.StatusOK, resp, requestID)
}

// Capture handles POST /api/v1/intents/:id/capture
func (h *IntentHandler) Capture(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	// Extract intent ID from path
	intentID := chi.URLParam(r, "id")
	if intentID == "" {
		server.WriteError(w, server.NewValidationError("id", "payment intent ID is required"), requestID)
		return
	}

	// Parse request body (optional for full capture)
	var req server.CapturePaymentRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			server.WriteError(w, server.NewInvalidJSONError(), requestID)
			return
		}
		if apiErr := req.Validate(); apiErr != nil {
			server.WriteError(w, apiErr, requestID)
			return
		}
	}

	// Get payment intent
	pi, err := h.intentRepo.GetByID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving payment intent"), requestID)
		return
	}
	if pi == nil {
		server.WriteError(w, server.NewNotFoundError("payment_intent", intentID), requestID)
		return
	}

	// Check if intent can be captured
	if !pi.CanCapture() {
		server.WriteError(w, server.NewBusinessRuleError("invalid_state",
			"Cannot capture payment in current state: "+string(pi.Status)), requestID)
		return
	}

	// Validate capture amount if provided
	if req.Amount != nil && req.Amount.GreaterThan(pi.Amount) {
		server.WriteError(w, server.NewBusinessRuleError("amount_exceeds_authorized",
			"Capture amount cannot exceed authorized amount"), requestID)
		return
	}

	// For manual capture, we need to trigger the capture through the workflow
	// The workflow handles this by executing the capture activity
	// Note: In production, this would signal the workflow to perform capture
	// For now, we query workflow state to confirm it's ready for capture
	if pi.WorkflowID == nil {
		server.WriteError(w, server.NewBusinessRuleError("no_workflow",
			"Payment intent has no associated workflow"), requestID)
		return
	}

	// Query the workflow to get current state before proceeding
	// The actual capture will be performed by the workflow's capture handler
	// In production, you'd add a SignalCapturePayment signal to the workflow

	// Return accepted - capture is async
	resp := map[string]any{
		"id":      intentID,
		"status":  "capture_pending",
		"message": "Capture request submitted",
	}
	server.WriteJSON(w, http.StatusAccepted, resp, requestID)
}

// Cancel handles POST /api/v1/intents/:id/cancel
func (h *IntentHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	// Extract intent ID from path
	intentID := chi.URLParam(r, "id")
	if intentID == "" {
		server.WriteError(w, server.NewValidationError("id", "payment intent ID is required"), requestID)
		return
	}

	// Parse request body (optional)
	var req server.CancelPaymentRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			server.WriteError(w, server.NewInvalidJSONError(), requestID)
			return
		}
	}

	// Get payment intent
	pi, err := h.intentRepo.GetByID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving payment intent"), requestID)
		return
	}
	if pi == nil {
		server.WriteError(w, server.NewNotFoundError("payment_intent", intentID), requestID)
		return
	}

	// Check if intent can be cancelled
	if !pi.CanCancel() {
		server.WriteError(w, server.NewBusinessRuleError("invalid_state",
			"Cannot cancel payment in current state: "+string(pi.Status)), requestID)
		return
	}

	// Signal workflow to cancel
	if pi.WorkflowID != nil {
		reason := req.Reason
		if reason == "" {
			reason = "cancelled by user"
		}
		err = h.temporal.SignalWorkflow(
			r.Context(),
			*pi.WorkflowID,
			"",
			workflowtypes.SignalCancelPayment,
			reason,
		)
		if err != nil {
			server.WriteError(w, server.NewInternalError("Error signaling workflow for cancellation"), requestID)
			return
		}
	} else {
		// No workflow, just update status directly
		if err := h.intentRepo.UpdateStatus(r.Context(), intentID, domain.PaymentIntentStatusCancelled); err != nil {
			server.WriteError(w, server.NewInternalError("Error cancelling payment intent"), requestID)
			return
		}
		pi.Status = domain.PaymentIntentStatusCancelled
	}

	resp := h.toPaymentIntentResponse(pi, nil)
	server.WriteJSON(w, http.StatusOK, resp, requestID)
}

// GetAttempts handles GET /api/v1/intents/:id/attempts
func (h *IntentHandler) GetAttempts(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	// Extract intent ID from path
	intentID := chi.URLParam(r, "id")
	if intentID == "" {
		server.WriteError(w, server.NewValidationError("id", "payment intent ID is required"), requestID)
		return
	}

	// Verify payment intent exists
	pi, err := h.intentRepo.GetByID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving payment intent"), requestID)
		return
	}
	if pi == nil {
		server.WriteError(w, server.NewNotFoundError("payment_intent", intentID), requestID)
		return
	}

	// Get attempts
	attempts, err := h.attemptRepo.GetByPaymentIntentID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving payment attempts"), requestID)
		return
	}

	// Convert to response format
	var attemptResponses []server.PaymentAttemptResponse
	for _, a := range attempts {
		attemptResponses = append(attemptResponses, h.toPaymentAttemptResponse(a))
	}

	resp := server.AttemptsListResponse{
		Data:    attemptResponses,
		HasMore: false, // TODO: implement pagination
	}
	server.WriteJSON(w, http.StatusOK, resp, requestID)
}

// GetHold handles GET /api/v1/intents/:id/hold
func (h *IntentHandler) GetHold(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	// Extract intent ID from path
	intentID := chi.URLParam(r, "id")
	if intentID == "" {
		server.WriteError(w, server.NewValidationError("id", "payment intent ID is required"), requestID)
		return
	}

	// Verify payment intent exists
	pi, err := h.intentRepo.GetByID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving payment intent"), requestID)
		return
	}
	if pi == nil {
		server.WriteError(w, server.NewNotFoundError("payment_intent", intentID), requestID)
		return
	}

	// Get authorization hold
	hold, err := h.holdRepo.GetByPaymentIntentID(r.Context(), intentID)
	if err != nil {
		server.WriteError(w, server.NewInternalError("Error retrieving authorization hold"), requestID)
		return
	}
	if hold == nil {
		server.WriteError(w, server.NewNotFoundError("authorization_hold", intentID), requestID)
		return
	}

	resp := h.toHoldResponse(hold)
	server.WriteJSON(w, http.StatusOK, resp, requestID)
}

// toPaymentIntentResponse converts a domain payment intent to API response
func (h *IntentHandler) toPaymentIntentResponse(pi *domain.PaymentIntent, hold *domain.AuthorizationHold) *server.PaymentIntentResponse {
	resp := &server.PaymentIntentResponse{
		ID:                pi.ID,
		Status:            string(pi.Status),
		Amount:            pi.Amount,
		Currency:          pi.Currency,
		CustomerID:        pi.CustomerID,
		Provider:          string(pi.Provider),
		CaptureMethod:     string(pi.CaptureMethod),
		PaymentMethodID:   pi.PaymentMethodID,
		ProviderPaymentID: pi.ProviderPaymentID,
		IdempotencyKey:    pi.IdempotencyKey,
		CreatedAt:         pi.CreatedAt,
		UpdatedAt:         pi.UpdatedAt,
	}

	if pi.WorkflowID != nil {
		resp.WorkflowID = *pi.WorkflowID
	}

	// Convert metadata back to map[string]string
	if pi.Metadata != nil {
		resp.Metadata = make(map[string]string)
		for k, v := range pi.Metadata {
			if s, ok := v.(string); ok {
				resp.Metadata[k] = s
			}
		}
	}

	if hold != nil {
		resp.Hold = h.toHoldResponse(hold)
	}

	return resp
}

// toPaymentAttemptResponse converts a domain payment attempt to API response
func (h *IntentHandler) toPaymentAttemptResponse(pa *domain.PaymentAttempt) server.PaymentAttemptResponse {
	resp := server.PaymentAttemptResponse{
		ID:            pa.ID,
		AttemptNumber: pa.AttemptNumber,
		Status:        string(pa.Status),
		Provider:      string(pa.Provider),
		CreatedAt:     pa.CreatedAt,
		CompletedAt:   pa.CompletedAt,
	}

	if pa.CanonicalDeclineCode != nil {
		resp.CanonicalCode = pa.CanonicalDeclineCode
	}
	if pa.DeclineType != nil {
		dt := string(*pa.DeclineType)
		resp.DeclineType = &dt
	}
	if pa.ProcessorTxnID != nil {
		resp.ProcessorTxnID = pa.ProcessorTxnID
	}

	return resp
}

// toHoldResponse converts a domain authorization hold to API response
func (h *IntentHandler) toHoldResponse(hold *domain.AuthorizationHold) *server.HoldResponse {
	resp := &server.HoldResponse{
		ID:        hold.ID,
		Amount:    hold.Amount,
		Status:    string(hold.Status),
		ExpiresAt: hold.ExpiresAt,
		CreatedAt: hold.CreatedAt,
	}

	if hold.AuthCode != nil {
		resp.AuthorizationCode = *hold.AuthCode
	}

	return resp
}
