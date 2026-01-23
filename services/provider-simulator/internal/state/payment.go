// Package state provides state management for the provider simulator.
package state

import (
	"time"
)

// PaymentStatus represents the current state of a payment.
type PaymentStatus string

const (
	StatusCreated             PaymentStatus = "created"
	StatusRequiresPayment     PaymentStatus = "requires_payment_method"
	StatusRequiresConfirm     PaymentStatus = "requires_confirmation"
	StatusProcessing          PaymentStatus = "processing"
	StatusAuthorized          PaymentStatus = "authorized"
	StatusCaptured            PaymentStatus = "captured"
	StatusPartiallyRefunded   PaymentStatus = "partially_refunded"
	StatusRefunded            PaymentStatus = "refunded"
	StatusCanceled            PaymentStatus = "canceled"
	StatusFailed              PaymentStatus = "failed"
	StatusRequiresCapture     PaymentStatus = "requires_capture"
	StatusRequiresAction      PaymentStatus = "requires_action"
)

// PaymentState represents the full state of a simulated payment.
type PaymentState struct {
	// Core identifiers
	ID         string `json:"id"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"` // pi_xxx, psp_xxx, etc.

	// Payment details
	Status         PaymentStatus `json:"status"`
	Amount         int64         `json:"amount"`
	Currency       string        `json:"currency"`
	CapturedAmount int64         `json:"captured_amount"`
	RefundedAmount int64         `json:"refunded_amount"`

	// Payment method info
	PaymentMethod string `json:"payment_method,omitempty"`
	CardLast4     string `json:"card_last4,omitempty"`
	CardBrand     string `json:"card_brand,omitempty"`
	CardNumber    string `json:"-"` // Never serialized - used for behavior triggers

	// Additional data
	Description   string            `json:"description,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CustomerEmail string            `json:"customer_email,omitempty"`
	CustomerID    string            `json:"customer_id,omitempty"`

	// Behavior configuration
	DeclineOnNext string `json:"decline_on_next,omitempty"` // Decline code for next action
	DelayMs       int    `json:"delay_ms,omitempty"`        // Response delay in milliseconds
	TimeoutOnNext bool   `json:"timeout_on_next,omitempty"` // Simulate timeout on next action

	// Provider-specific data
	ProviderData map[string]any `json:"provider_data,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookRecord represents a webhook that was or will be delivered.
type WebhookRecord struct {
	ID           string         `json:"id"`
	PaymentID    string         `json:"payment_id"`
	Provider     string         `json:"provider"`
	EventType    string         `json:"event_type"`
	Payload      []byte         `json:"payload"`
	TargetURL    string         `json:"target_url"`
	Status       WebhookStatus  `json:"status"`
	Attempts     int            `json:"attempts"`
	LastAttempt  *time.Time     `json:"last_attempt,omitempty"`
	LastError    string         `json:"last_error,omitempty"`
	ResponseCode int            `json:"response_code,omitempty"`
	ScheduledAt  time.Time      `json:"scheduled_at"`
	DeliveredAt  *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// WebhookStatus represents the delivery status of a webhook.
type WebhookStatus string

const (
	WebhookPending   WebhookStatus = "pending"
	WebhookDelivered WebhookStatus = "delivered"
	WebhookFailed    WebhookStatus = "failed"
	WebhookRetrying  WebhookStatus = "retrying"
)

// RefundRecord represents a refund associated with a payment.
type RefundRecord struct {
	ID         string        `json:"id"`
	PaymentID  string        `json:"payment_id"`
	ProviderID string        `json:"provider_id"` // re_xxx, etc.
	Amount     int64         `json:"amount"`
	Status     RefundStatus  `json:"status"`
	Reason     string        `json:"reason,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
}

// RefundStatus represents the status of a refund.
type RefundStatus string

const (
	RefundPending   RefundStatus = "pending"
	RefundSucceeded RefundStatus = "succeeded"
	RefundFailed    RefundStatus = "failed"
)
