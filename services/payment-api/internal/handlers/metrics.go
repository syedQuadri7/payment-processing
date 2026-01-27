package handlers

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"payment-processing/services/payment-api"
	"payment-processing/services/payment-api/internal/middleware"
)

// MetricsCollector collects and exposes application metrics
type MetricsCollector struct {
	mu sync.RWMutex

	// Counters
	paymentIntentsTotal     map[string]int64 // key: provider:status
	paymentDeclineTotal     map[string]int64 // key: provider:canonical_code
	webhooksReceivedTotal   map[string]int64 // key: provider:event_type

	// Gauges
	clearingAccountBalances map[string]float64 // key: account_id
	activeWorkflows         map[string]int64   // key: workflow_type

	// Histograms (simplified - just track count and sum for averages)
	authDurationSum   map[string]float64 // key: provider
	authDurationCount map[string]int64   // key: provider
	webhookDurationSum   map[string]float64 // key: provider
	webhookDurationCount map[string]int64   // key: provider
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		paymentIntentsTotal:     make(map[string]int64),
		paymentDeclineTotal:     make(map[string]int64),
		webhooksReceivedTotal:   make(map[string]int64),
		clearingAccountBalances: make(map[string]float64),
		activeWorkflows:         make(map[string]int64),
		authDurationSum:         make(map[string]float64),
		authDurationCount:       make(map[string]int64),
		webhookDurationSum:      make(map[string]float64),
		webhookDurationCount:    make(map[string]int64),
	}
}

// IncrementPaymentIntent increments the payment intent counter
func (m *MetricsCollector) IncrementPaymentIntent(provider, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := provider + ":" + status
	m.paymentIntentsTotal[key]++
}

// IncrementDecline increments the decline counter
func (m *MetricsCollector) IncrementDecline(provider, canonicalCode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := provider + ":" + canonicalCode
	m.paymentDeclineTotal[key]++
}

// IncrementWebhook increments the webhook counter
func (m *MetricsCollector) IncrementWebhook(provider, eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := provider + ":" + eventType
	m.webhooksReceivedTotal[key]++
}

// SetClearingBalance sets the clearing account balance gauge
func (m *MetricsCollector) SetClearingBalance(accountID string, balance float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearingAccountBalances[accountID] = balance
}

// SetActiveWorkflows sets the active workflow count gauge
func (m *MetricsCollector) SetActiveWorkflows(workflowType string, count int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeWorkflows[workflowType] = count
}

// RecordAuthDuration records an authorization duration
func (m *MetricsCollector) RecordAuthDuration(provider string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authDurationSum[provider] += duration.Seconds()
	m.authDurationCount[provider]++
}

// RecordWebhookDuration records a webhook processing duration
func (m *MetricsCollector) RecordWebhookDuration(provider string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhookDurationSum[provider] += duration.Seconds()
	m.webhookDurationCount[provider]++
}

// MetricsHandler handles the metrics endpoint
type MetricsHandler struct {
	collector *MetricsCollector
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(collector *MetricsCollector) *MetricsHandler {
	return &MetricsHandler{
		collector: collector,
	}
}

// Metrics handles GET /metrics - Prometheus format metrics
func (h *MetricsHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	requestID := middleware.GetRequestID(r.Context())

	if r.Method != http.MethodGet {
		server.WriteError(w, server.NewMethodNotAllowedError(r.Method), requestID)
		return
	}

	h.collector.mu.RLock()
	defer h.collector.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// Write metrics in Prometheus format

	// payment_intents_total
	fmt.Fprintln(w, "# HELP payment_intents_total Total number of payment intents by provider and status")
	fmt.Fprintln(w, "# TYPE payment_intents_total counter")
	for key, count := range h.collector.paymentIntentsTotal {
		provider, status := splitKey(key)
		fmt.Fprintf(w, "payment_intents_total{provider=\"%s\",status=\"%s\"} %d\n", provider, status, count)
	}

	// payment_decline_total
	fmt.Fprintln(w, "# HELP payment_decline_total Total number of payment declines by provider and canonical code")
	fmt.Fprintln(w, "# TYPE payment_decline_total counter")
	for key, count := range h.collector.paymentDeclineTotal {
		provider, code := splitKey(key)
		fmt.Fprintf(w, "payment_decline_total{provider=\"%s\",canonical_code=\"%s\"} %d\n", provider, code, count)
	}

	// webhook_received_total
	fmt.Fprintln(w, "# HELP webhook_received_total Total number of webhooks received by provider and event type")
	fmt.Fprintln(w, "# TYPE webhook_received_total counter")
	for key, count := range h.collector.webhooksReceivedTotal {
		provider, eventType := splitKey(key)
		fmt.Fprintf(w, "webhook_received_total{provider=\"%s\",event_type=\"%s\"} %d\n", provider, eventType, count)
	}

	// clearing_account_balance
	fmt.Fprintln(w, "# HELP clearing_account_balance Current balance of clearing accounts")
	fmt.Fprintln(w, "# TYPE clearing_account_balance gauge")
	for account, balance := range h.collector.clearingAccountBalances {
		fmt.Fprintf(w, "clearing_account_balance{account=\"%s\"} %f\n", account, balance)
	}

	// temporal_workflow_active
	fmt.Fprintln(w, "# HELP temporal_workflow_active Number of active workflows by type")
	fmt.Fprintln(w, "# TYPE temporal_workflow_active gauge")
	for workflowType, count := range h.collector.activeWorkflows {
		fmt.Fprintf(w, "temporal_workflow_active{workflow_type=\"%s\"} %d\n", workflowType, count)
	}

	// payment_authorization_duration_seconds (average approximation)
	fmt.Fprintln(w, "# HELP payment_authorization_duration_seconds Authorization duration in seconds by provider")
	fmt.Fprintln(w, "# TYPE payment_authorization_duration_seconds summary")
	for provider, sum := range h.collector.authDurationSum {
		count := h.collector.authDurationCount[provider]
		if count > 0 {
			fmt.Fprintf(w, "payment_authorization_duration_seconds_sum{provider=\"%s\"} %f\n", provider, sum)
			fmt.Fprintf(w, "payment_authorization_duration_seconds_count{provider=\"%s\"} %d\n", provider, count)
		}
	}

	// webhook_processing_duration_seconds (average approximation)
	fmt.Fprintln(w, "# HELP webhook_processing_duration_seconds Webhook processing duration in seconds by provider")
	fmt.Fprintln(w, "# TYPE webhook_processing_duration_seconds summary")
	for provider, sum := range h.collector.webhookDurationSum {
		count := h.collector.webhookDurationCount[provider]
		if count > 0 {
			fmt.Fprintf(w, "webhook_processing_duration_seconds_sum{provider=\"%s\"} %f\n", provider, sum)
			fmt.Fprintf(w, "webhook_processing_duration_seconds_count{provider=\"%s\"} %d\n", provider, count)
		}
	}
}

// splitKey splits a "provider:value" key into its parts
func splitKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
