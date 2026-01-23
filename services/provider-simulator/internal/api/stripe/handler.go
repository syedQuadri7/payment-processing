package stripe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"provider-simulator/internal/behavior"
	"provider-simulator/internal/state"
)

// generateID creates a random ID string.
func generateID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Handler handles Stripe API simulation requests.
type Handler struct {
	store         state.Store
	logger        *slog.Logger
	webhookTarget string
	webhookSecret string
	onWebhook     func(paymentID, eventType string) // Callback for webhook triggering
}

// NewHandler creates a new Stripe API handler.
func NewHandler(store state.Store, logger *slog.Logger, webhookTarget, webhookSecret string) *Handler {
	return &Handler{
		store:         store,
		logger:        logger,
		webhookTarget: webhookTarget,
		webhookSecret: webhookSecret,
	}
}

// SetWebhookCallback sets the callback for webhook delivery.
func (h *Handler) SetWebhookCallback(cb func(paymentID, eventType string)) {
	h.onWebhook = cb
}

// RegisterRoutes registers all Stripe routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Payment Intents
	mux.HandleFunc("POST /stripe/v1/payment_intents", h.CreatePaymentIntent)
	mux.HandleFunc("GET /stripe/v1/payment_intents/{id}", h.GetPaymentIntent)
	mux.HandleFunc("POST /stripe/v1/payment_intents/{id}", h.UpdatePaymentIntent)
	mux.HandleFunc("POST /stripe/v1/payment_intents/{id}/confirm", h.ConfirmPaymentIntent)
	mux.HandleFunc("POST /stripe/v1/payment_intents/{id}/capture", h.CapturePaymentIntent)
	mux.HandleFunc("POST /stripe/v1/payment_intents/{id}/cancel", h.CancelPaymentIntent)

	// Refunds
	mux.HandleFunc("POST /stripe/v1/refunds", h.CreateRefund)
	mux.HandleFunc("GET /stripe/v1/refunds/{id}", h.GetRefund)
}

// CreatePaymentIntent handles POST /stripe/v1/payment_intents.
func (h *Handler) CreatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "", "Invalid request body")
		return
	}

	if req.Amount <= 0 {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "invalid_amount", "Amount must be positive")
		return
	}

	if req.Currency == "" {
		req.Currency = "usd"
	}

	captureMethod := "automatic"
	if req.CaptureMethod == "manual" {
		captureMethod = "manual"
	}

	// Generate IDs
	piID := "pi_" + generateID()
	clientSecret := piID + "_secret_" + generateID()

	// Determine initial status
	initialStatus := state.StatusRequiresPayment
	if req.PaymentMethod != "" || req.PaymentMethodData != nil {
		initialStatus = state.StatusRequiresConfirm
	}

	// Extract card number for behavior triggers
	var cardNumber string
	if req.PaymentMethodData != nil && req.PaymentMethodData.Card != nil {
		cardNumber = req.PaymentMethodData.Card.Number
	}

	// Create payment state
	payment := &state.PaymentState{
		ID:          piID,
		Provider:    "stripe",
		ProviderID:  piID,
		Status:      initialStatus,
		Amount:      req.Amount,
		Currency:    strings.ToLower(req.Currency),
		Description: req.Description,
		Metadata:    req.Metadata,
		CardNumber:  cardNumber,
		ProviderData: map[string]any{
			"capture_method":  captureMethod,
			"client_secret":   clientSecret,
			"payment_method":  req.PaymentMethod,
		},
	}

	if err := h.store.CreatePayment(r.Context(), payment); err != nil {
		h.logger.Error("failed to create payment", "error", err)
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	// Auto-confirm if requested
	if req.Confirm {
		payment, err := h.processConfirmation(r.Context(), payment)
		if err != nil {
			// Error already handled
			return
		}
		h.writeJSON(w, http.StatusOK, h.toPaymentIntent(payment))
		return
	}

	h.logger.Info("payment intent created",
		"id", piID,
		"amount", req.Amount,
		"currency", req.Currency,
		"capture_method", captureMethod)

	h.writeJSON(w, http.StatusOK, h.toPaymentIntent(payment))
}

// GetPaymentIntent handles GET /stripe/v1/payment_intents/{id}.
func (h *Handler) GetPaymentIntent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeStripeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "No such payment_intent: "+id)
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	h.writeJSON(w, http.StatusOK, h.toPaymentIntent(payment))
}

// UpdatePaymentIntent handles POST /stripe/v1/payment_intents/{id}.
func (h *Handler) UpdatePaymentIntent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeStripeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "No such payment_intent: "+id)
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	var req CreatePaymentIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "", "Invalid request body")
		return
	}

	if req.Description != "" {
		payment.Description = req.Description
	}
	if req.Metadata != nil {
		payment.Metadata = req.Metadata
	}

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	h.writeJSON(w, http.StatusOK, h.toPaymentIntent(payment))
}

// ConfirmPaymentIntent handles POST /stripe/v1/payment_intents/{id}/confirm.
func (h *Handler) ConfirmPaymentIntent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeStripeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "No such payment_intent: "+id)
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	// Check valid state for confirmation
	if payment.Status != state.StatusRequiresPayment && payment.Status != state.StatusRequiresConfirm {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "payment_intent_unexpected_state",
			"This PaymentIntent's status is "+string(payment.Status))
		return
	}

	var req ConfirmPaymentIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "", "Invalid request body")
		return
	}

	// Update card number if provided
	if req.PaymentMethodData != nil && req.PaymentMethodData.Card != nil {
		payment.CardNumber = req.PaymentMethodData.Card.Number
	}

	payment, err = h.processConfirmation(r.Context(), payment)
	if err != nil {
		// Write error response
		if payment != nil && payment.Status == state.StatusFailed {
			h.writeStripeError(w, http.StatusPaymentRequired, "card_error",
				behavior.GetStripeDeclineCode(payment.DeclineOnNext),
				getDeclineMessage(payment.DeclineOnNext))
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	h.writeJSON(w, http.StatusOK, h.toPaymentIntent(payment))
}

// processConfirmation handles the payment confirmation logic.
func (h *Handler) processConfirmation(ctx context.Context, payment *state.PaymentState) (*state.PaymentState, error) {
	// Check for behavior triggers from card number and metadata
	effectiveBehavior := behavior.GetEffectiveBehavior(payment.CardNumber, payment.Metadata)

	// Apply delay if configured (metadata or card or admin-set)
	delay := effectiveBehavior.DelayMs
	if payment.DelayMs > 0 {
		delay = payment.DelayMs
	}
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	// Check for timeout behavior
	if effectiveBehavior.ShouldTimeout || payment.TimeoutOnNext {
		time.Sleep(30 * time.Second)
		return nil, errors.New("timeout")
	}

	// Determine decline code (admin-set takes precedence over card/metadata)
	declineCode := effectiveBehavior.DeclineCode
	if payment.DeclineOnNext != "" {
		declineCode = payment.DeclineOnNext
	}

	// Process payment
	if declineCode != "" {
		// Payment failed
		payment.Status = state.StatusFailed
		payment.DeclineOnNext = declineCode
		if err := h.store.UpdatePayment(ctx, payment); err != nil {
			return nil, err
		}

		// Trigger webhook
		if h.onWebhook != nil {
			h.onWebhook(payment.ID, "payment_intent.payment_failed")
		}

		return payment, errors.New("payment declined")
	}

	// Payment succeeded (or requires capture for manual capture method)
	captureMethod, _ := payment.ProviderData["capture_method"].(string)
	if captureMethod == "manual" {
		payment.Status = state.StatusRequiresCapture
		payment.CapturedAmount = 0
	} else {
		payment.Status = state.StatusCaptured
		payment.CapturedAmount = payment.Amount
	}

	// Clear one-time behavior triggers
	payment.DeclineOnNext = ""
	payment.DelayMs = 0
	payment.TimeoutOnNext = false

	if err := h.store.UpdatePayment(ctx, payment); err != nil {
		return nil, err
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "payment_intent.succeeded")
	}

	h.logger.Info("payment confirmed",
		"id", payment.ID,
		"status", payment.Status,
		"capture_method", captureMethod)

	return payment, nil
}

// CapturePaymentIntent handles POST /stripe/v1/payment_intents/{id}/capture.
func (h *Handler) CapturePaymentIntent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeStripeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "No such payment_intent: "+id)
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	if payment.Status != state.StatusRequiresCapture && payment.Status != state.StatusAuthorized {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "payment_intent_unexpected_state",
			"This PaymentIntent's status is "+string(payment.Status))
		return
	}

	var req CapturePaymentIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "", "Invalid request body")
		return
	}

	captureAmount := payment.Amount
	if req.AmountToCapture > 0 {
		if req.AmountToCapture > payment.Amount {
			h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "amount_too_large",
				"Amount to capture exceeds authorized amount")
			return
		}
		captureAmount = req.AmountToCapture
	}

	payment.Status = state.StatusCaptured
	payment.CapturedAmount = captureAmount

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "charge.captured")
	}

	h.logger.Info("payment captured", "id", payment.ID, "amount", captureAmount)
	h.writeJSON(w, http.StatusOK, h.toPaymentIntent(payment))
}

// CancelPaymentIntent handles POST /stripe/v1/payment_intents/{id}/cancel.
func (h *Handler) CancelPaymentIntent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeStripeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "No such payment_intent: "+id)
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	// Can't cancel succeeded or already canceled
	if payment.Status == state.StatusCaptured || payment.Status == state.StatusCanceled {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "payment_intent_unexpected_state",
			"This PaymentIntent's status is "+string(payment.Status))
		return
	}

	var req CancelPaymentIntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "", "Invalid request body")
		return
	}

	payment.Status = state.StatusCanceled
	now := time.Now().Unix()
	payment.ProviderData["canceled_at"] = now
	if req.CancellationReason != "" {
		payment.ProviderData["cancellation_reason"] = req.CancellationReason
	}

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	h.logger.Info("payment canceled", "id", payment.ID)
	h.writeJSON(w, http.StatusOK, h.toPaymentIntent(payment))
}

// CreateRefund handles POST /stripe/v1/refunds.
func (h *Handler) CreateRefund(w http.ResponseWriter, r *http.Request) {
	var req CreateRefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "", "Invalid request body")
		return
	}

	// Find the payment
	paymentID := req.PaymentIntent
	if paymentID == "" && req.Charge != "" {
		// For charges, we'd need a charge->payment mapping
		// For now, treat charge ID as payment ID
		paymentID = req.Charge
	}

	if paymentID == "" {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "parameter_missing",
			"Must provide payment_intent or charge")
		return
	}

	payment, err := h.store.GetPayment(r.Context(), paymentID)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeStripeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing",
				"No such payment_intent: "+paymentID)
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	if payment.Status != state.StatusCaptured {
		h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "charge_not_refundable",
			"This payment cannot be refunded")
		return
	}

	refundAmount := payment.CapturedAmount - payment.RefundedAmount
	if req.Amount > 0 {
		if req.Amount > refundAmount {
			h.writeStripeError(w, http.StatusBadRequest, "invalid_request_error", "amount_too_large",
				"Refund amount exceeds available balance")
			return
		}
		refundAmount = req.Amount
	}

	// Create refund record
	refundID := "re_" + generateID()
	refund := &state.RefundRecord{
		ID:         refundID,
		PaymentID:  payment.ID,
		ProviderID: refundID,
		Amount:     refundAmount,
		Status:     state.RefundSucceeded,
		Reason:     req.Reason,
	}

	if err := h.store.CreateRefund(r.Context(), refund); err != nil {
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	// Update payment
	payment.RefundedAmount += refundAmount
	if payment.RefundedAmount >= payment.CapturedAmount {
		payment.Status = state.StatusRefunded
	} else {
		payment.Status = state.StatusPartiallyRefunded
	}

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "charge.refunded")
	}

	h.logger.Info("refund created", "id", refundID, "payment_id", payment.ID, "amount", refundAmount)
	h.writeJSON(w, http.StatusOK, h.toRefund(refund, payment.Currency))
}

// GetRefund handles GET /stripe/v1/refunds/{id}.
func (h *Handler) GetRefund(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	refund, err := h.store.GetRefund(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeStripeError(w, http.StatusNotFound, "invalid_request_error", "resource_missing", "No such refund: "+id)
			return
		}
		h.writeStripeError(w, http.StatusInternalServerError, "api_error", "", "Internal error")
		return
	}

	// Get payment for currency
	payment, _ := h.store.GetPayment(r.Context(), refund.PaymentID)
	currency := "usd"
	if payment != nil {
		currency = payment.Currency
	}

	h.writeJSON(w, http.StatusOK, h.toRefund(refund, currency))
}

// toPaymentIntent converts internal state to Stripe PaymentIntent response.
func (h *Handler) toPaymentIntent(p *state.PaymentState) *PaymentIntent {
	pi := &PaymentIntent{
		ID:                 p.ID,
		Object:             "payment_intent",
		Amount:             p.Amount,
		AmountCapturable:   0,
		AmountReceived:     p.CapturedAmount,
		CaptureMethod:      getStringFromMap(p.ProviderData, "capture_method", "automatic"),
		ClientSecret:       getStringFromMap(p.ProviderData, "client_secret", ""),
		ConfirmationMethod: "automatic",
		Created:            p.CreatedAt.Unix(),
		Currency:           p.Currency,
		Description:        p.Description,
		Livemode:           false,
		Metadata:           p.Metadata,
		PaymentMethodTypes: []string{"card"},
		Status:             ToStripeStatus(string(p.Status)),
		PaymentMethod:      getStringFromMap(p.ProviderData, "payment_method", ""),
	}

	if p.Status == state.StatusRequiresCapture {
		pi.AmountCapturable = p.Amount
	}

	if p.Status == state.StatusFailed && p.DeclineOnNext != "" {
		stripeCode := behavior.GetStripeDeclineCode(p.DeclineOnNext)
		pi.LastPaymentError = &PaymentError{
			Code:        stripeCode,
			DeclineCode: stripeCode,
			Message:     getDeclineMessage(p.DeclineOnNext),
			Type:        "card_error",
		}
	}

	if canceledAt, ok := p.ProviderData["canceled_at"].(int64); ok {
		pi.CanceledAt = &canceledAt
	}
	if reason, ok := p.ProviderData["cancellation_reason"].(string); ok {
		pi.CancellationReason = reason
	}

	return pi
}

// toRefund converts internal state to Stripe Refund response.
func (h *Handler) toRefund(r *state.RefundRecord, currency string) *Refund {
	return &Refund{
		ID:       r.ID,
		Object:   "refund",
		Amount:   r.Amount,
		Charge:   "ch_" + r.PaymentID[3:], // Convert pi_ to ch_
		Created:  r.CreatedAt.Unix(),
		Currency: currency,
		Reason:   r.Reason,
		Status:   string(r.Status),
	}
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeStripeError writes a Stripe-formatted error response.
func (h *Handler) writeStripeError(w http.ResponseWriter, status int, errType, code, message string) {
	h.writeJSON(w, status, StripeError{
		Error: StripeErrorBody{
			Type:    errType,
			Code:    code,
			Message: message,
		},
	})
}

// getStringFromMap safely gets a string from a map.
func getStringFromMap(m map[string]any, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}

// getDeclineMessage returns a human-readable message for a decline code.
func getDeclineMessage(code string) string {
	messages := map[string]string{
		"INSUFFICIENT_FUNDS":  "Your card has insufficient funds.",
		"CARD_EXPIRED":        "Your card has expired.",
		"INVALID_CVC":         "Your card's security code is incorrect.",
		"GENERIC_DECLINE":     "Your card was declined.",
		"FRAUD_SUSPICION":     "Your card was declined.",
		"STOLEN_CARD":         "Your card was declined.",
		"LOST_CARD":           "Your card was declined.",
		"OVER_LIMIT":          "Your card is over its credit limit.",
		"DO_NOT_HONOR":        "Your card was declined.",
		"INVALID_CARD_NUMBER": "Your card number is incorrect.",
		"INVALID_EXPIRY":      "Your card's expiration date is invalid.",
		"INVALID_ACCOUNT":     "Your card was declined.",
		"ACCOUNT_CLOSED":      "Your card was declined.",
		"ACCOUNT_RESTRICTED":  "Your card was declined.",
		"NOT_PERMITTED":       "This transaction is not permitted.",
		"TRY_AGAIN_LATER":     "Please try again later.",
		"PROCESSOR_ERROR":     "An error occurred processing your card.",
		"PICKUP_CARD":         "Your card was declined.",
	}
	if msg, ok := messages[code]; ok {
		return msg
	}
	return "Your card was declined."
}
