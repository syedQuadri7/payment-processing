package paypal

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"provider-simulator/internal/behavior"
	"provider-simulator/internal/state"
)

// Handler handles PayPal API simulation requests.
type Handler struct {
	store         state.Store
	logger        *slog.Logger
	webhookTarget string
	webhookSecret string
	onWebhook     func(paymentID, eventType string)
}

// NewHandler creates a new PayPal API handler.
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

// RegisterRoutes registers all PayPal routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// OAuth Token
	mux.HandleFunc("POST /paypal/v1/oauth2/token", h.Token)

	// Orders API v2
	mux.HandleFunc("POST /paypal/v2/checkout/orders", h.CreateOrder)
	mux.HandleFunc("GET /paypal/v2/checkout/orders/{id}", h.GetOrder)
	mux.HandleFunc("POST /paypal/v2/checkout/orders/{id}/authorize", h.AuthorizeOrder)
	mux.HandleFunc("POST /paypal/v2/checkout/orders/{id}/capture", h.CaptureOrder)

	// Payments API v2
	mux.HandleFunc("POST /paypal/v2/payments/authorizations/{id}/capture", h.CaptureAuthorization)
	mux.HandleFunc("POST /paypal/v2/payments/authorizations/{id}/void", h.VoidAuthorization)
	mux.HandleFunc("POST /paypal/v2/payments/captures/{id}/refund", h.RefundCapture)

	// Webhook Signature Verification
	mux.HandleFunc("POST /paypal/v1/notifications/verify-webhook-signature", h.VerifyWebhookSignature)
}

// Token handles POST /paypal/v1/oauth2/token (mock OAuth).
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
	// Generate a mock access token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	accessToken := base64.RawURLEncoding.EncodeToString(tokenBytes)

	resp := TokenResponse{
		Scope:       "https://uri.paypal.com/services/payments/payment https://uri.paypal.com/services/payments/refund",
		AccessToken: accessToken,
		TokenType:   "Bearer",
		AppID:       "APP-80W284485P519543T",
		ExpiresIn:   32400,
		Nonce:       nowRFC3339() + generateID(),
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// CreateOrder handles POST /paypal/v2/checkout/orders.
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writePayPalError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if len(req.PurchaseUnits) == 0 {
		h.writePayPalError(w, http.StatusBadRequest, "MISSING_REQUIRED_PARAMETER", "purchase_units is required")
		return
	}

	if req.Intent == "" {
		req.Intent = "CAPTURE"
	}

	// Parse amount
	unit := req.PurchaseUnits[0]
	if unit.Amount == nil {
		h.writePayPalError(w, http.StatusBadRequest, "MISSING_REQUIRED_PARAMETER", "amount is required")
		return
	}

	amountValue, err := strconv.ParseFloat(unit.Amount.Value, 64)
	if err != nil || amountValue <= 0 {
		h.writePayPalError(w, http.StatusBadRequest, "INVALID_PARAMETER_VALUE", "Invalid amount value")
		return
	}

	// Convert to cents
	amountCents := int64(amountValue * 100)

	// Generate order ID
	orderID := generateOrderID()

	// Extract card number for behavior triggers
	var cardNumber string
	if req.PaymentSource != nil && req.PaymentSource.Card != nil {
		cardNumber = req.PaymentSource.Card.Number
	}

	// Create payment state
	payment := &state.PaymentState{
		ID:         orderID,
		Provider:   "paypal",
		ProviderID: orderID,
		Status:     state.StatusCreated,
		Amount:     amountCents,
		Currency:   strings.ToLower(unit.Amount.CurrencyCode),
		CardNumber: cardNumber,
		ProviderData: map[string]any{
			"intent":       req.Intent,
			"reference_id": unit.ReferenceID,
			"invoice_id":   unit.InvoiceID,
			"description":  unit.Description,
		},
	}

	if req.Payer != nil && req.Payer.EmailAddress != "" {
		payment.CustomerEmail = req.Payer.EmailAddress
	}

	if err := h.store.CreatePayment(r.Context(), payment); err != nil {
		h.logger.Error("failed to create payment", "error", err)
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	h.logger.Info("order created",
		"id", orderID,
		"amount", amountCents,
		"currency", unit.Amount.CurrencyCode,
		"intent", req.Intent)

	h.writeJSON(w, http.StatusCreated, h.toOrder(payment))
}

// GetOrder handles GET /paypal/v2/checkout/orders/{id}.
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writePayPalError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Order not found")
			return
		}
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	h.writeJSON(w, http.StatusOK, h.toOrder(payment))
}

// AuthorizeOrder handles POST /paypal/v2/checkout/orders/{id}/authorize.
func (h *Handler) AuthorizeOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writePayPalError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Order not found")
			return
		}
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	if payment.Status != state.StatusCreated {
		h.writePayPalError(w, http.StatusBadRequest, "ORDER_NOT_APPROVED", "Order is not in correct state for authorization")
		return
	}

	// Check card behavior
	cardBehavior := behavior.GetBehavior(payment.CardNumber)

	// Apply delay if configured
	if cardBehavior.DelayMs > 0 {
		time.Sleep(time.Duration(cardBehavior.DelayMs) * time.Millisecond)
	}

	// Check for timeout
	if cardBehavior.ShouldTimeout {
		time.Sleep(30 * time.Second)
		h.writePayPalError(w, http.StatusGatewayTimeout, "INTERNAL_ERROR", "Request timeout")
		return
	}

	// Check for decline
	if cardBehavior.DeclineCode != "" {
		payment.Status = state.StatusFailed
		payment.DeclineOnNext = cardBehavior.DeclineCode
		if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
			h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
			return
		}

		ppDecline := behavior.GetPayPalDeclineCode(cardBehavior.DeclineCode)
		h.writePayPalError(w, http.StatusUnprocessableEntity, ppDecline, "Payment declined")
		return
	}

	// Generate authorization ID
	authID := "AUTH-" + generateID()
	payment.Status = state.StatusAuthorized
	payment.ProviderData["authorization_id"] = authID

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "PAYMENT.AUTHORIZATION.CREATED")
	}

	h.logger.Info("order authorized", "id", id, "authorization_id", authID)
	h.writeJSON(w, http.StatusCreated, h.toOrder(payment))
}

// CaptureOrder handles POST /paypal/v2/checkout/orders/{id}/capture (direct capture without authorization).
func (h *Handler) CaptureOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			h.writePayPalError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Order not found")
			return
		}
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	if payment.Status != state.StatusCreated && payment.Status != state.StatusAuthorized {
		h.writePayPalError(w, http.StatusBadRequest, "ORDER_NOT_APPROVED", "Order is not in correct state for capture")
		return
	}

	// Check card behavior
	cardBehavior := behavior.GetBehavior(payment.CardNumber)

	// Apply delay if configured
	if cardBehavior.DelayMs > 0 {
		time.Sleep(time.Duration(cardBehavior.DelayMs) * time.Millisecond)
	}

	// Check for timeout
	if cardBehavior.ShouldTimeout {
		time.Sleep(30 * time.Second)
		h.writePayPalError(w, http.StatusGatewayTimeout, "INTERNAL_ERROR", "Request timeout")
		return
	}

	// Check for decline
	if cardBehavior.DeclineCode != "" {
		payment.Status = state.StatusFailed
		payment.DeclineOnNext = cardBehavior.DeclineCode
		if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
			h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
			return
		}

		ppDecline := behavior.GetPayPalDeclineCode(cardBehavior.DeclineCode)
		h.writePayPalError(w, http.StatusUnprocessableEntity, ppDecline, "Payment declined")
		return
	}

	// Generate capture ID
	captureID := "CAP-" + generateID()
	payment.Status = state.StatusCaptured
	payment.CapturedAmount = payment.Amount
	payment.ProviderData["capture_id"] = captureID

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "PAYMENT.CAPTURE.COMPLETED")
	}

	h.logger.Info("order captured", "id", id, "capture_id", captureID)
	h.writeJSON(w, http.StatusCreated, h.toOrder(payment))
}

// CaptureAuthorization handles POST /paypal/v2/payments/authorizations/{id}/capture.
func (h *Handler) CaptureAuthorization(w http.ResponseWriter, r *http.Request) {
	authID := r.PathValue("id")

	// Find payment by authorization ID
	payments, err := h.store.ListPayments(r.Context(), state.ListOptions{Provider: "paypal"})
	if err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	var payment *state.PaymentState
	for _, p := range payments {
		if storedAuthID, ok := p.ProviderData["authorization_id"].(string); ok && storedAuthID == authID {
			payment = p
			break
		}
	}

	if payment == nil {
		h.writePayPalError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Authorization not found")
		return
	}

	if payment.Status != state.StatusAuthorized {
		h.writePayPalError(w, http.StatusBadRequest, "AUTHORIZATION_ALREADY_CAPTURED", "Authorization is not in correct state")
		return
	}

	var req CaptureAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		h.writePayPalError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	captureAmount := payment.Amount
	if req.Amount != nil && req.Amount.Value != "" {
		amountValue, err := strconv.ParseFloat(req.Amount.Value, 64)
		if err == nil && amountValue > 0 {
			captureAmount = int64(amountValue * 100)
		}
	}

	if captureAmount > payment.Amount {
		h.writePayPalError(w, http.StatusBadRequest, "AMOUNT_EXCEEDS_AUTHORIZED", "Capture amount exceeds authorized amount")
		return
	}

	// Generate capture ID
	captureID := "CAP-" + generateID()
	payment.Status = state.StatusCaptured
	payment.CapturedAmount = captureAmount
	payment.ProviderData["capture_id"] = captureID

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "PAYMENT.CAPTURE.COMPLETED")
	}

	h.logger.Info("authorization captured", "authorization_id", authID, "capture_id", captureID, "amount", captureAmount)

	h.writeJSON(w, http.StatusCreated, &Capture{
		ID:           captureID,
		Status:       CaptureStatusCompleted,
		Amount:       &Amount{CurrencyCode: strings.ToUpper(payment.Currency), Value: fmt.Sprintf("%.2f", float64(captureAmount)/100)},
		CreateTime:   nowRFC3339(),
		FinalCapture: true,
		Links:        h.captureLinks(captureID),
	})
}

// VoidAuthorization handles POST /paypal/v2/payments/authorizations/{id}/void.
func (h *Handler) VoidAuthorization(w http.ResponseWriter, r *http.Request) {
	authID := r.PathValue("id")

	// Find payment by authorization ID
	payments, err := h.store.ListPayments(r.Context(), state.ListOptions{Provider: "paypal"})
	if err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	var payment *state.PaymentState
	for _, p := range payments {
		if storedAuthID, ok := p.ProviderData["authorization_id"].(string); ok && storedAuthID == authID {
			payment = p
			break
		}
	}

	if payment == nil {
		h.writePayPalError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Authorization not found")
		return
	}

	if payment.Status != state.StatusAuthorized {
		h.writePayPalError(w, http.StatusBadRequest, "AUTHORIZATION_ALREADY_VOIDED", "Authorization is not in correct state")
		return
	}

	payment.Status = state.StatusCanceled

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "PAYMENT.AUTHORIZATION.VOIDED")
	}

	h.logger.Info("authorization voided", "authorization_id", authID)

	// Return 204 No Content for void
	w.WriteHeader(http.StatusNoContent)
}

// RefundCapture handles POST /paypal/v2/payments/captures/{id}/refund.
func (h *Handler) RefundCapture(w http.ResponseWriter, r *http.Request) {
	captureID := r.PathValue("id")

	// Find payment by capture ID
	payments, err := h.store.ListPayments(r.Context(), state.ListOptions{Provider: "paypal"})
	if err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	var payment *state.PaymentState
	for _, p := range payments {
		if storedCaptureID, ok := p.ProviderData["capture_id"].(string); ok && storedCaptureID == captureID {
			payment = p
			break
		}
	}

	if payment == nil {
		h.writePayPalError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Capture not found")
		return
	}

	if payment.Status != state.StatusCaptured && payment.Status != state.StatusPartiallyRefunded {
		h.writePayPalError(w, http.StatusBadRequest, "CAPTURE_NOT_REFUNDABLE", "Capture is not in correct state")
		return
	}

	var req RefundCaptureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength > 0 {
		h.writePayPalError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	refundAmount := payment.CapturedAmount - payment.RefundedAmount
	if req.Amount != nil && req.Amount.Value != "" {
		amountValue, err := strconv.ParseFloat(req.Amount.Value, 64)
		if err == nil && amountValue > 0 {
			refundAmount = int64(amountValue * 100)
		}
	}

	if refundAmount > payment.CapturedAmount-payment.RefundedAmount {
		h.writePayPalError(w, http.StatusBadRequest, "REFUND_AMOUNT_EXCEEDED", "Refund amount exceeds available balance")
		return
	}

	// Generate refund ID
	refundID := "REF-" + generateID()

	// Create refund record
	refund := &state.RefundRecord{
		ID:         refundID,
		PaymentID:  payment.ID,
		ProviderID: refundID,
		Amount:     refundAmount,
		Status:     state.RefundSucceeded,
	}

	if err := h.store.CreateRefund(r.Context(), refund); err != nil {
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
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
		h.writePayPalError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error")
		return
	}

	// Trigger webhook
	if h.onWebhook != nil {
		h.onWebhook(payment.ID, "PAYMENT.CAPTURE.REFUNDED")
	}

	h.logger.Info("capture refunded", "capture_id", captureID, "refund_id", refundID, "amount", refundAmount)

	h.writeJSON(w, http.StatusCreated, &Refund{
		ID:         refundID,
		Status:     RefundStatusCompleted,
		Amount:     &Amount{CurrencyCode: strings.ToUpper(payment.Currency), Value: fmt.Sprintf("%.2f", float64(refundAmount)/100)},
		CreateTime: nowRFC3339(),
		Links:      h.refundLinks(refundID),
	})
}

// VerifyWebhookSignature handles POST /paypal/v1/notifications/verify-webhook-signature.
func (h *Handler) VerifyWebhookSignature(w http.ResponseWriter, r *http.Request) {
	var req VerifyWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writePayPalError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Mock verification - always return SUCCESS for testing
	h.writeJSON(w, http.StatusOK, VerifyWebhookResponse{
		VerificationStatus: "SUCCESS",
	})
}

// toOrder converts internal state to PayPal Order response.
func (h *Handler) toOrder(p *state.PaymentState) *Order {
	amountStr := fmt.Sprintf("%.2f", float64(p.Amount)/100)
	currency := strings.ToUpper(p.Currency)

	order := &Order{
		ID:         p.ID,
		Status:     ToPayPalStatus(string(p.Status)),
		Intent:     getStringFromMap(p.ProviderData, "intent", "CAPTURE"),
		CreateTime: p.CreatedAt.UTC().Format(time.RFC3339),
		UpdateTime: p.UpdatedAt.UTC().Format(time.RFC3339),
		PurchaseUnits: []PurchaseUnitResponse{
			{
				ReferenceID: getStringFromMap(p.ProviderData, "reference_id", "default"),
				Amount:      &Amount{CurrencyCode: currency, Value: amountStr},
			},
		},
		Links: h.orderLinks(p.ID, p.Status),
	}

	// Add payments if applicable
	payments := &PaymentCollection{}

	if authID, ok := p.ProviderData["authorization_id"].(string); ok {
		payments.Authorizations = []Authorization{
			{
				ID:             authID,
				Status:         h.authStatus(p.Status),
				Amount:         &Amount{CurrencyCode: currency, Value: amountStr},
				CreateTime:     p.CreatedAt.UTC().Format(time.RFC3339),
				ExpirationTime: p.CreatedAt.Add(72 * time.Hour).UTC().Format(time.RFC3339),
				Links:          h.authLinks(authID),
			},
		}
	}

	if captureID, ok := p.ProviderData["capture_id"].(string); ok {
		capturedStr := fmt.Sprintf("%.2f", float64(p.CapturedAmount)/100)
		payments.Captures = []Capture{
			{
				ID:           captureID,
				Status:       CaptureStatusCompleted,
				Amount:       &Amount{CurrencyCode: currency, Value: capturedStr},
				CreateTime:   p.UpdatedAt.UTC().Format(time.RFC3339),
				FinalCapture: true,
				Links:        h.captureLinks(captureID),
			},
		}
	}

	if payments.Authorizations != nil || payments.Captures != nil {
		order.PurchaseUnits[0].Payments = payments
	}

	return order
}

func (h *Handler) authStatus(status state.PaymentStatus) string {
	switch status {
	case state.StatusAuthorized, state.StatusRequiresCapture:
		return AuthStatusCreated
	case state.StatusCaptured:
		return AuthStatusCaptured
	case state.StatusCanceled:
		return AuthStatusVoided
	case state.StatusFailed:
		return AuthStatusDenied
	default:
		return AuthStatusPending
	}
}

func (h *Handler) orderLinks(id string, status state.PaymentStatus) []Link {
	base := "https://api.sandbox.paypal.com/v2/checkout/orders/" + id
	links := []Link{
		{Href: base, Rel: "self", Method: "GET"},
	}

	if status == state.StatusCreated || status == state.StatusRequiresPayment {
		links = append(links,
			Link{Href: base + "/authorize", Rel: "authorize", Method: "POST"},
			Link{Href: base + "/capture", Rel: "capture", Method: "POST"},
		)
	}

	return links
}

func (h *Handler) authLinks(authID string) []Link {
	base := "https://api.sandbox.paypal.com/v2/payments/authorizations/" + authID
	return []Link{
		{Href: base, Rel: "self", Method: "GET"},
		{Href: base + "/capture", Rel: "capture", Method: "POST"},
		{Href: base + "/void", Rel: "void", Method: "POST"},
	}
}

func (h *Handler) captureLinks(captureID string) []Link {
	base := "https://api.sandbox.paypal.com/v2/payments/captures/" + captureID
	return []Link{
		{Href: base, Rel: "self", Method: "GET"},
		{Href: base + "/refund", Rel: "refund", Method: "POST"},
	}
}

func (h *Handler) refundLinks(refundID string) []Link {
	base := "https://api.sandbox.paypal.com/v2/payments/refunds/" + refundID
	return []Link{
		{Href: base, Rel: "self", Method: "GET"},
	}
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writePayPalError writes a PayPal-formatted error response.
func (h *Handler) writePayPalError(w http.ResponseWriter, status int, name, message string) {
	h.writeJSON(w, status, PayPalError{
		Name:    name,
		Message: message,
		Links: []Link{
			{
				Href: "https://developer.paypal.com/docs/api/orders/v2/#error-" + strings.ToLower(name),
				Rel:  "information_link",
			},
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

// generateID creates a random ID string.
func generateID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateOrderID creates a PayPal-style order ID.
func generateOrderID() string {
	bytes := make([]byte, 17)
	rand.Read(bytes)
	// PayPal order IDs are alphanumeric, uppercase
	id := base64.RawURLEncoding.EncodeToString(bytes)
	id = strings.ToUpper(id)
	id = strings.ReplaceAll(id, "-", "")
	id = strings.ReplaceAll(id, "_", "")
	if len(id) > 17 {
		id = id[:17]
	}
	return id
}
