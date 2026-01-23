package webhook

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"provider-simulator/internal/generator"
	"provider-simulator/internal/signer"
	"provider-simulator/internal/state"
)

// EngineConfig holds configuration for the webhook delivery engine.
type EngineConfig struct {
	TargetURL     string
	Secret        string
	DelayMs       int           // Delay before sending webhooks
	MaxRetries    int           // Maximum retry attempts
	RetryDelay    time.Duration // Delay between retries
	WorkerCount   int           // Number of concurrent workers
}

// DefaultConfig returns a default configuration.
func DefaultConfig() EngineConfig {
	return EngineConfig{
		TargetURL:   "http://localhost:8080",
		Secret:      "whsec_test_secret",
		DelayMs:     0,
		MaxRetries:  3,
		RetryDelay:  time.Second * 5,
		WorkerCount: 2,
	}
}

// Engine handles webhook delivery with retry logic.
type Engine struct {
	config     EngineConfig
	store      state.Store
	queue      *Queue
	client     *http.Client
	generators *generator.Registry
	signers    *signer.Registry
	logger     *slog.Logger

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

// NewEngine creates a new webhook delivery engine.
func NewEngine(cfg EngineConfig, store state.Store, logger *slog.Logger) *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	return &Engine{
		config:     cfg,
		store:      store,
		queue:      NewQueue(),
		client:     &http.Client{Timeout: 30 * time.Second},
		generators: generator.NewRegistry(),
		signers:    signer.NewRegistry(),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins the webhook delivery workers.
func (e *Engine) Start() {
	for i := 0; i < e.config.WorkerCount; i++ {
		e.wg.Add(1)
		go e.worker(i)
	}
	e.logger.Info("webhook engine started", "workers", e.config.WorkerCount)
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	e.cancel()
	e.queue.Close()
	e.wg.Wait()
	e.logger.Info("webhook engine stopped")
}

// Enqueue schedules a webhook for delivery.
func (e *Engine) Enqueue(paymentID, provider, eventType string) error {
	// Get payment state
	payment, err := e.store.GetPayment(e.ctx, paymentID)
	if err != nil {
		return fmt.Errorf("getting payment: %w", err)
	}

	// Generate webhook payload
	gen, err := e.generators.Get(provider)
	if err != nil {
		return fmt.Errorf("getting generator: %w", err)
	}

	data := map[string]any{
		"payment_id":     payment.ProviderID,
		"psp_reference":  payment.ProviderID,
		"authorization_id": payment.ProviderID,
		"capture_id":     payment.ProviderID,
		"amount":         payment.Amount,
		"currency":       payment.Currency,
	}

	// Add decline code if failed
	if payment.Status == state.StatusFailed && payment.DeclineOnNext != "" {
		data["decline_code"] = payment.DeclineOnNext
		data["reason"] = payment.DeclineOnNext
	}

	payload, err := gen.Generate(eventType, data)
	if err != nil {
		return fmt.Errorf("generating payload: %w", err)
	}

	// Calculate scheduled time
	scheduledAt := time.Now()
	if e.config.DelayMs > 0 {
		scheduledAt = scheduledAt.Add(time.Duration(e.config.DelayMs) * time.Millisecond)
	}

	// Create webhook record
	webhookID := "wh_" + generateID()
	webhook := &state.WebhookRecord{
		ID:          webhookID,
		PaymentID:   paymentID,
		Provider:    provider,
		EventType:   eventType,
		Payload:     payload,
		TargetURL:   e.config.TargetURL + gen.Endpoint(),
		Status:      state.WebhookPending,
		Attempts:    0,
		ScheduledAt: scheduledAt,
	}

	if err := e.store.CreateWebhook(e.ctx, webhook); err != nil {
		return fmt.Errorf("storing webhook: %w", err)
	}

	// Add to queue
	e.queue.Push(&WebhookJob{
		Webhook:     webhook,
		ScheduledAt: scheduledAt,
	})

	e.logger.Debug("webhook enqueued",
		"id", webhookID,
		"provider", provider,
		"event", eventType,
		"scheduled_at", scheduledAt)

	return nil
}

// worker processes jobs from the queue.
func (e *Engine) worker(id int) {
	defer e.wg.Done()

	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			job := e.queue.PopReady()
			if job == nil {
				// No ready jobs, wait a bit
				select {
				case <-e.ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
					continue
				}
			}

			// Wait if job is scheduled for the future
			waitTime := time.Until(job.ScheduledAt)
			if waitTime > 0 {
				select {
				case <-e.ctx.Done():
					return
				case <-time.After(waitTime):
				}
			}

			e.processJob(job)
		}
	}
}

// processJob attempts to deliver a webhook.
func (e *Engine) processJob(job *WebhookJob) {
	webhook := job.Webhook

	// Sign the payload
	sign, err := e.signers.Get(webhook.Provider)
	if err != nil {
		e.logger.Error("getting signer failed", "error", err, "webhook_id", webhook.ID)
		e.markFailed(webhook, "signer not found: "+err.Error())
		return
	}

	headers, err := sign.Sign(webhook.Payload, e.config.Secret, signer.SignOpts{})
	if err != nil {
		e.logger.Error("signing failed", "error", err, "webhook_id", webhook.ID)
		e.markFailed(webhook, "signing failed: "+err.Error())
		return
	}

	// Send the webhook
	req, err := http.NewRequestWithContext(e.ctx, "POST", webhook.TargetURL, bytes.NewReader(webhook.Payload))
	if err != nil {
		e.logger.Error("creating request failed", "error", err, "webhook_id", webhook.ID)
		e.markFailed(webhook, "request creation failed: "+err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	webhook.Attempts++
	now := time.Now()
	webhook.LastAttempt = &now

	resp, err := e.client.Do(req)
	if err != nil {
		webhook.LastError = err.Error()
		e.handleFailure(webhook, job)
		return
	}
	defer resp.Body.Close()

	// Read response body (limited)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	webhook.ResponseCode = resp.StatusCode

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Success
		deliveredAt := time.Now()
		webhook.DeliveredAt = &deliveredAt
		webhook.Status = state.WebhookDelivered
		webhook.LastError = ""

		if err := e.store.UpdateWebhook(e.ctx, webhook); err != nil {
			e.logger.Error("updating webhook failed", "error", err, "webhook_id", webhook.ID)
		}

		e.logger.Info("webhook delivered",
			"id", webhook.ID,
			"provider", webhook.Provider,
			"event", webhook.EventType,
			"status_code", resp.StatusCode,
			"attempts", webhook.Attempts)
	} else {
		// Failure
		webhook.LastError = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		e.handleFailure(webhook, job)
	}
}

// handleFailure handles a failed delivery attempt.
func (e *Engine) handleFailure(webhook *state.WebhookRecord, job *WebhookJob) {
	if webhook.Attempts >= e.config.MaxRetries {
		e.markFailed(webhook, webhook.LastError)
		return
	}

	// Schedule retry
	webhook.Status = state.WebhookRetrying
	if err := e.store.UpdateWebhook(e.ctx, webhook); err != nil {
		e.logger.Error("updating webhook failed", "error", err, "webhook_id", webhook.ID)
	}

	retryAt := time.Now().Add(e.config.RetryDelay * time.Duration(webhook.Attempts))
	job.ScheduledAt = retryAt
	e.queue.Push(job)

	e.logger.Warn("webhook delivery failed, scheduling retry",
		"id", webhook.ID,
		"attempts", webhook.Attempts,
		"retry_at", retryAt,
		"error", webhook.LastError)
}

// markFailed marks a webhook as permanently failed.
func (e *Engine) markFailed(webhook *state.WebhookRecord, reason string) {
	webhook.Status = state.WebhookFailed
	webhook.LastError = reason

	if err := e.store.UpdateWebhook(e.ctx, webhook); err != nil {
		e.logger.Error("updating webhook failed", "error", err, "webhook_id", webhook.ID)
	}

	e.logger.Error("webhook delivery permanently failed",
		"id", webhook.ID,
		"provider", webhook.Provider,
		"event", webhook.EventType,
		"attempts", webhook.Attempts,
		"error", reason)
}

// generateID creates a random ID string.
func generateID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
