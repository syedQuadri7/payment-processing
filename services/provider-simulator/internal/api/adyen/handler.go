package adyen

import (
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

// Handler handles Adyen API simulation requests.
type Handler struct {
	store         state.Store
	logger        *slog.Logger
	webhookTarget string
	webhookSecret string
	onWebhook     func(paymentID, eventType string)
}

// NewHandler creates a new Adyen API handler.
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

// RegisterRoutes registers all Adyen routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Checkout API v71
	mux.HandleFunc("POST /adyen/v71/payments", h.CreatePayment)
	mux.HandleFunc("POST /adyen/v71/payments/{id}/captures", h.CapturePayment)
	mux.HandleFunc("POST /adyen/v71/payments/{id}/cancels", h.CancelPayment)
	mux.HandleFunc("POST /adyen/v71/payments/{id}/refunds", h.RefundPayment)

	// Also support older API paths
	mux.HandleFunc("POST /adyen/v70/payments", h.CreatePayment)
	mux.HandleFunc("POST /adyen/v70/payments/{id}/captures", h.CapturePayment)
	mux.HandleFunc("POST /adyen/v70/payments/{id}/cancels", h.CancelPayment)
	mux.HandleFunc("POST /adyen/v70/payments/{id}/refunds", h.RefundPayment)
}

// CreatePayment handles POST /adyen/v71/payments.
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeAdyenError(w, http.StatusBadRequest, "100", "Invalid request", "validation")
		return
	}

	if req.Amount == nil || req.Amount.Value <= 0 {
		h.writeAdyenError(w, http.StatusBadRequest, "100", "Amount is required", "validation")
		return
	}

	if req.MerchantAccount == "" {
		h.writeAdyenError(w, http.StatusBadRequest, "100", "MerchantAccount is required", "validation")
		return
	}

	// Generate PSP reference (Adyen's payment ID)
	pspRef := "ADYEN_" + generateID()

	// Extract card number for behavior triggers
	var cardNumber string
	if req.PaymentMethod != nil {
		cardNumber = req.PaymentMethod.Number
	}

	// Check card behavior
	cardBehavior := behavior.GetBehavior(cardNumber)

	// Apply delay if configured
	if cardBehavior.DelayMs > 0 {
		time.Sleep(time.Duration(cardBehavior.DelayMs) * time.Millisecond)
	}

	// Check for timeout
	if cardBehavior.ShouldTimeout {
		time.Sleep(30 * time.Second)
		h.writeAdyenError(w, http.StatusGatewayTimeout, "000", "Request timeout", "internal")
		return
	}

	// Determine initial status
	initialStatus := state.StatusAuthorized
	resultCode := ResultCodeAuthorised
	var refusalReason, refusalCode string

	if cardBehavior.DeclineCode != "" {
		initialStatus = state.StatusFailed
		resultCode = ResultCodeRefused
		refusalCode = behavior.GetAdyenDeclineCode(cardBehavior.DeclineCode)
		refusalReason = refusalCode
	}

	// Create payment state
	payment := &state.PaymentState{
		ID:         pspRef,
		Provider:   "adyen",
		ProviderID: pspRef,
		Status:     initialStatus,
		Amount:     req.Amount.Value,
		Currency:   strings.ToLower(req.Amount.Currency),
		CardNumber: cardNumber,
		Metadata:   req.Metadata,
		ProviderData: map[string]any{
			"merchant_account":  req.MerchantAccount,
			"merchant_reference": req.Reference,
			"payment_method":    req.PaymentMethod,
		},
	}

	if req.ShopperEmail != "" {
		payment.CustomerEmail = req.ShopperEmail
	}
	if req.ShopperReference != "" {
		payment.CustomerID = req.ShopperReference
	}

	if err := h.store.CreatePayment(r.Context(), payment); err != nil {
		h.logger.Error("failed to create payment", "error", err)
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
		return
	}

	// Trigger webhook for AUTHORISATION
	if h.onWebhook != nil {
		if resultCode == ResultCodeAuthorised {
			h.onWebhook(payment.ID, "AUTHORISATION")
		} else {
			h.onWebhook(payment.ID, "AUTHORISATION_FAILED")
		}
	}

	h.logger.Info("payment created",
		"psp_reference", pspRef,
		"amount", req.Amount.Value,
		"currency", req.Amount.Currency,
		"result", resultCode)

	resp := &PaymentResponse{
		PspReference:      pspRef,
		ResultCode:        resultCode,
		MerchantReference: req.Reference,
		Amount:            req.Amount,
		RefusalReason:     refusalReason,
		RefusalReasonCode: refusalCode,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// CapturePayment handles POST /adyen/v71/payments/{id}/captures.
func (h *Handler) CapturePayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeAdyenError(w, http.StatusNotFound, "803", "Payment not found", "validation")
			return
		}
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
		return
	}

	if payment.Status != state.StatusAuthorized && payment.Status != state.StatusRequiresCapture {
		h.writeAdyenError(w, http.StatusBadRequest, "167", "Original payment not in correct state", "validation")
		return
	}

	var req CaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeAdyenError(w, http.StatusBadRequest, "100", "Invalid request", "validation")
		return
	}

	captureAmount := payment.Amount
	if req.Amount != nil && req.Amount.Value > 0 {
		if req.Amount.Value > payment.Amount {
			h.writeAdyenError(w, http.StatusBadRequest, "167", "Amount exceeds authorized amount", "validation")
			return
		}
		captureAmount = req.Amount.Value
	}

	// Generate capture PSP reference
	capturePspRef := "ADYEN_" + generateID()

	payment.Status = state.StatusCaptured
	payment.CapturedAmount = captureAmount

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "CAPTURE")
	}

	h.logger.Info("payment captured", "psp_reference", id, "amount", captureAmount)

	h.writeJSON(w, http.StatusOK, &CaptureResponse{
		PspReference:        capturePspRef,
		PaymentPspReference: id,
		Status:              ModificationStatusReceived,
		Amount:              &Amount{Value: captureAmount, Currency: strings.ToUpper(payment.Currency)},
		Reference:           req.Reference,
	})
}

// CancelPayment handles POST /adyen/v71/payments/{id}/cancels.
func (h *Handler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeAdyenError(w, http.StatusNotFound, "803", "Payment not found", "validation")
			return
		}
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
		return
	}

	// Can only cancel authorized payments that haven't been captured
	if payment.Status != state.StatusAuthorized && payment.Status != state.StatusRequiresCapture {
		h.writeAdyenError(w, http.StatusBadRequest, "167", "Original payment not in correct state", "validation")
		return
	}

	var req CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		h.writeAdyenError(w, http.StatusBadRequest, "100", "Invalid request", "validation")
		return
	}

	// Generate cancel PSP reference
	cancelPspRef := "ADYEN_" + generateID()

	payment.Status = state.StatusCanceled

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "CANCELLATION")
	}

	h.logger.Info("payment canceled", "psp_reference", id)

	h.writeJSON(w, http.StatusOK, &CancelResponse{
		PspReference:        cancelPspRef,
		PaymentPspReference: id,
		Status:              ModificationStatusReceived,
		Reference:           req.Reference,
	})
}

// RefundPayment handles POST /adyen/v71/payments/{id}/refunds.
func (h *Handler) RefundPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writeAdyenError(w, http.StatusNotFound, "803", "Payment not found", "validation")
			return
		}
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
		return
	}

	if payment.Status != state.StatusCaptured && payment.Status != state.StatusPartiallyRefunded {
		h.writeAdyenError(w, http.StatusBadRequest, "167", "Original payment not in correct state", "validation")
		return
	}

	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeAdyenError(w, http.StatusBadRequest, "100", "Invalid request", "validation")
		return
	}

	refundAmount := payment.CapturedAmount - payment.RefundedAmount
	if req.Amount != nil && req.Amount.Value > 0 {
		if req.Amount.Value > refundAmount {
			h.writeAdyenError(w, http.StatusBadRequest, "167", "Refund amount exceeds available balance", "validation")
			return
		}
		refundAmount = req.Amount.Value
	}

	// Generate refund PSP reference
	refundPspRef := "ADYEN_" + generateID()

	// Create refund record
	refund := &state.RefundRecord{
		ID:         refundPspRef,
		PaymentID:  payment.ID,
		ProviderID: refundPspRef,
		Amount:     refundAmount,
		Status:     state.RefundSucceeded,
	}

	if err := h.store.CreateRefund(r.Context(), refund); err != nil {
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
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
		h.writeAdyenError(w, http.StatusInternalServerError, "000", "Internal error", "internal")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "REFUND")
	}

	h.logger.Info("refund created", "psp_reference", refundPspRef, "payment_id", id, "amount", refundAmount)

	h.writeJSON(w, http.StatusOK, &RefundResponse{
		PspReference:        refundPspRef,
		PaymentPspReference: id,
		Status:              ModificationStatusReceived,
		Amount:              &Amount{Value: refundAmount, Currency: strings.ToUpper(payment.Currency)},
		Reference:           req.Reference,
	})
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeAdyenError writes an Adyen-formatted error response.
func (h *Handler) writeAdyenError(w http.ResponseWriter, status int, code, message, errType string) {
	h.writeJSON(w, status, AdyenError{
		Status:    status,
		ErrorCode: code,
		Message:   message,
		ErrorType: errType,
	})
}

// generateID creates a random ID string.
func generateID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
