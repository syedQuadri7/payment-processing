package state

import (
	"context"
	"errors"
)

// Common errors returned by Store implementations.
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// Store defines the interface for payment state persistence.
type Store interface {
	// Payment operations
	CreatePayment(ctx context.Context, payment *PaymentState) error
	GetPayment(ctx context.Context, id string) (*PaymentState, error)
	GetPaymentByProviderID(ctx context.Context, provider, providerID string) (*PaymentState, error)
	UpdatePayment(ctx context.Context, payment *PaymentState) error
	ListPayments(ctx context.Context, opts ListOptions) ([]*PaymentState, error)
	DeletePayment(ctx context.Context, id string) error

	// Webhook operations
	CreateWebhook(ctx context.Context, webhook *WebhookRecord) error
	GetWebhook(ctx context.Context, id string) (*WebhookRecord, error)
	UpdateWebhook(ctx context.Context, webhook *WebhookRecord) error
	ListWebhooks(ctx context.Context, opts ListOptions) ([]*WebhookRecord, error)
	GetPendingWebhooks(ctx context.Context) ([]*WebhookRecord, error)

	// Refund operations
	CreateRefund(ctx context.Context, refund *RefundRecord) error
	GetRefund(ctx context.Context, id string) (*RefundRecord, error)
	ListRefundsByPayment(ctx context.Context, paymentID string) ([]*RefundRecord, error)

	// State management
	Reset(ctx context.Context) error
	Stats(ctx context.Context) (*StoreStats, error)
}

// ListOptions provides pagination and filtering for list operations.
type ListOptions struct {
	Provider string
	Status   string
	Limit    int
	Offset   int
}

// StoreStats contains statistics about the store state.
type StoreStats struct {
	TotalPayments  int            `json:"total_payments"`
	TotalWebhooks  int            `json:"total_webhooks"`
	TotalRefunds   int            `json:"total_refunds"`
	PaymentsByProvider map[string]int `json:"payments_by_provider"`
	PaymentsByStatus   map[string]int `json:"payments_by_status"`
	WebhooksByStatus   map[string]int `json:"webhooks_by_status"`
}
