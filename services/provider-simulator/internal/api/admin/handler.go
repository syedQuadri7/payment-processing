// Package admin provides the admin API handlers for the provider simulator.
package admin

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"provider-simulator/internal/state"
)

// Handler handles admin API requests.
type Handler struct {
	store  state.Store
	logger *slog.Logger
}

// NewHandler creates a new admin handler.
func NewHandler(store state.Store, logger *slog.Logger) *Handler {
	return &Handler{
		store:  store,
		logger: logger,
	}
}

// Reset clears all state in the simulator.
func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Reset(r.Context()); err != nil {
		h.logger.Error("failed to reset store", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to reset state")
		return
	}

	h.logger.Info("state reset")
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// ListPayments returns all payments in the store.
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	opts := state.ListOptions{
		Provider: r.URL.Query().Get("provider"),
		Status:   r.URL.Query().Get("status"),
	}

	payments, err := h.store.ListPayments(r.Context(), opts)
	if err != nil {
		h.logger.Error("failed to list payments", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list payments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"payments": payments,
		"count":    len(payments),
	})
}

// GetPayment returns a single payment by ID.
func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing payment id")
		return
	}

	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			writeError(w, http.StatusNotFound, "payment not found")
			return
		}
		h.logger.Error("failed to get payment", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get payment")
		return
	}

	// Include refunds
	refunds, _ := h.store.ListRefundsByPayment(r.Context(), id)

	writeJSON(w, http.StatusOK, map[string]any{
		"payment": payment,
		"refunds": refunds,
	})
}

// SetBehaviorRequest is the request body for SetBehavior.
type SetBehaviorRequest struct {
	DeclineOnNext string `json:"decline_on_next,omitempty"`
	DelayMs       int    `json:"delay_ms,omitempty"`
	TimeoutOnNext bool   `json:"timeout_on_next,omitempty"`
}

// SetBehavior configures behavior for the next action on a payment.
func (h *Handler) SetBehavior(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing payment id")
		return
	}

	var req SetBehaviorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	payment, err := h.store.GetPayment(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			writeError(w, http.StatusNotFound, "payment not found")
			return
		}
		h.logger.Error("failed to get payment", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get payment")
		return
	}

	// Apply behavior settings
	payment.DeclineOnNext = req.DeclineOnNext
	payment.DelayMs = req.DelayMs
	payment.TimeoutOnNext = req.TimeoutOnNext

	if err := h.store.UpdatePayment(r.Context(), payment); err != nil {
		h.logger.Error("failed to update payment behavior", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to update payment")
		return
	}

	h.logger.Info("behavior set",
		"id", id,
		"decline_on_next", req.DeclineOnNext,
		"delay_ms", req.DelayMs,
		"timeout_on_next", req.TimeoutOnNext)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "behavior_set",
		"payment": payment,
	})
}

// ListWebhooks returns all webhooks in the store.
func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	opts := state.ListOptions{
		Status: r.URL.Query().Get("status"),
	}

	webhooks, err := h.store.ListWebhooks(r.Context(), opts)
	if err != nil {
		h.logger.Error("failed to list webhooks", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"webhooks": webhooks,
		"count":    len(webhooks),
	})
}

// RedeliverWebhook queues a webhook for redelivery.
func (h *Handler) RedeliverWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing webhook id")
		return
	}

	webhook, err := h.store.GetWebhook(r.Context(), id)
	if err != nil {
		if errors.Is(err, state.ErrNotFound) {
			writeError(w, http.StatusNotFound, "webhook not found")
			return
		}
		h.logger.Error("failed to get webhook", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}

	// Mark for redelivery
	webhook.Status = state.WebhookPending
	webhook.Attempts = 0
	webhook.LastError = ""

	if err := h.store.UpdateWebhook(r.Context(), webhook); err != nil {
		h.logger.Error("failed to update webhook", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}

	h.logger.Info("webhook queued for redelivery", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "redelivery_queued",
		"webhook": webhook,
	})
}

// Stats returns store statistics.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.Stats(r.Context())
	if err != nil {
		h.logger.Error("failed to get stats", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
