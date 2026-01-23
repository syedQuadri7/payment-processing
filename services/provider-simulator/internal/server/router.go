package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"provider-simulator/internal/api/admin"
	"provider-simulator/internal/api/adyen"
	"provider-simulator/internal/api/paypal"
	"provider-simulator/internal/api/stripe"
)

// setupRouter creates the HTTP router with all endpoints.
func (s *Server) setupRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", s.handleHealth)

	// Admin API
	adminHandler := admin.NewHandler(s.store, s.logger)
	mux.HandleFunc("POST /admin/reset", adminHandler.Reset)
	mux.HandleFunc("GET /admin/payments", adminHandler.ListPayments)
	mux.HandleFunc("GET /admin/payments/{id}", adminHandler.GetPayment)
	mux.HandleFunc("POST /admin/payments/{id}/behavior", adminHandler.SetBehavior)
	mux.HandleFunc("GET /admin/webhooks", adminHandler.ListWebhooks)
	mux.HandleFunc("POST /admin/webhooks/{id}/redeliver", adminHandler.RedeliverWebhook)
	mux.HandleFunc("GET /admin/stats", adminHandler.Stats)

	// Create webhook callback
	webhookCallback := func(paymentID, eventType string) {
		// Determine provider from payment
		payment, err := s.store.GetPayment(context.Background(), paymentID)
		if err != nil {
			s.logger.Error("webhook callback: failed to get payment", "error", err, "payment_id", paymentID)
			return
		}
		if err := s.webhookEngine.Enqueue(paymentID, payment.Provider, eventType); err != nil {
			s.logger.Error("webhook callback: failed to enqueue", "error", err, "payment_id", paymentID)
		}
	}

	// Stripe API
	stripeHandler := stripe.NewHandler(s.store, s.logger, s.config.WebhookTarget, s.config.WebhookSecret)
	stripeHandler.SetWebhookCallback(webhookCallback)
	stripeHandler.RegisterRoutes(mux)

	// Adyen API
	adyenHandler := adyen.NewHandler(s.store, s.logger, s.config.WebhookTarget, s.config.WebhookSecret)
	adyenHandler.SetWebhookCallback(webhookCallback)
	adyenHandler.RegisterRoutes(mux)

	// PayPal API
	paypalHandler := paypal.NewHandler(s.store, s.logger, s.config.WebhookTarget, s.config.WebhookSecret)
	paypalHandler.SetWebhookCallback(webhookCallback)
	paypalHandler.RegisterRoutes(mux)

	// Wrap with logging middleware
	return s.loggingMiddleware(mux)
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		http.Error(w, "store error", http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
		"stats":  stats,
		"config": map[string]any{
			"webhook_target":   s.config.WebhookTarget,
			"webhook_delay_ms": s.config.WebhookDelayMs,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// loggingMiddleware logs all requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
