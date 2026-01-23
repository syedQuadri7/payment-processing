package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.temporal.io/sdk/activity"

	"payment-processing/pkg/domain"
)

// DeclineCodeRepository defines the interface for decline code lookups
type DeclineCodeRepository interface {
	GetByProviderCode(ctx context.Context, provider domain.Provider, code string) (*domain.DeclineCodeMapping, error)
}

// PaymentAttemptRepository defines the interface for payment attempt persistence
type PaymentAttemptRepository interface {
	Create(ctx context.Context, pa *domain.PaymentAttempt) error
	GetByID(ctx context.Context, id string) (*domain.PaymentAttempt, error)
	MarkCompleted(ctx context.Context, id string, status domain.AttemptStatus, providerCode, declineCode *string, declineType *domain.DeclineType) error
}

// OutboxRepository defines the interface for outbox event persistence
type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
	GetUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []string) error
}

// AuditLogRepository defines the interface for audit log persistence
type AuditLogRepository interface {
	Create(ctx context.Context, entry *domain.AuditLogEntry) error
}

// Activities holds the dependencies for workflow activities
type Activities struct {
	DeclineCodeRepo    DeclineCodeRepository
	PaymentAttemptRepo PaymentAttemptRepository
	OutboxRepo         OutboxRepository
	AuditLogRepo       AuditLogRepository
}

// NewActivities creates a new Activities instance
func NewActivities() *Activities {
	return &Activities{}
}

// NewActivitiesWithDependencies creates a new Activities instance with repository dependencies
func NewActivitiesWithDependencies(
	declineCodeRepo DeclineCodeRepository,
	paymentAttemptRepo PaymentAttemptRepository,
	outboxRepo OutboxRepository,
	auditLogRepo AuditLogRepository,
) *Activities {
	return &Activities{
		DeclineCodeRepo:    declineCodeRepo,
		PaymentAttemptRepo: paymentAttemptRepo,
		OutboxRepo:         outboxRepo,
		AuditLogRepo:       auditLogRepo,
	}
}

// AuthorizePaymentInput contains data for authorization
type AuthorizePaymentInput struct {
	PaymentIntentID string          `json:"payment_intent_id"`
	AttemptNumber   int             `json:"attempt_number"`
	Provider        domain.Provider `json:"provider"`
	Amount          decimal.Decimal `json:"amount"`
	Currency        string          `json:"currency"`
	PaymentMethodID string          `json:"payment_method_id"`
	IdempotencyKey  string          `json:"idempotency_key"`
}

// AuthorizePaymentResult contains the authorization response
type AuthorizePaymentResult struct {
	Success           bool                `json:"success"`
	ProviderPaymentID string              `json:"provider_payment_id,omitempty"`
	AuthorizationCode string              `json:"authorization_code,omitempty"`
	NetworkTxnID      string              `json:"network_txn_id,omitempty"`
	ExpiresAt         *time.Time          `json:"expires_at,omitempty"`
	DeclineCode       *string             `json:"decline_code,omitempty"`
	DeclineType       *domain.DeclineType `json:"decline_type,omitempty"`
	ErrorMessage      *string             `json:"error_message,omitempty"`
}

// AuthorizePayment sends an authorization request to the payment provider
func (a *Activities) AuthorizePayment(ctx context.Context, input AuthorizePaymentInput) (*AuthorizePaymentResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Authorizing payment",
		"payment_intent_id", input.PaymentIntentID,
		"attempt", input.AttemptNumber,
		"provider", input.Provider,
		"amount", input.Amount.String(),
	)

	// TODO: Call actual provider API based on input.Provider
	// This is a placeholder that simulates provider behavior

	// For now, return a successful authorization
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 day auth window
	return &AuthorizePaymentResult{
		Success:           true,
		ProviderPaymentID: fmt.Sprintf("psp_%s_%d", input.PaymentIntentID, time.Now().UnixNano()),
		AuthorizationCode: fmt.Sprintf("AUTH%d", time.Now().UnixNano()%1000000),
		ExpiresAt:         &expiresAt,
	}, nil
}

// CapturePaymentInput contains data for capture
type CapturePaymentInput struct {
	PaymentIntentID   string          `json:"payment_intent_id"`
	ProviderPaymentID string          `json:"provider_payment_id"`
	Provider          domain.Provider `json:"provider"`
	Amount            decimal.Decimal `json:"amount"`
	Currency          string          `json:"currency"`
	IdempotencyKey    string          `json:"idempotency_key"`
}

// CapturePaymentResult contains the capture response
type CapturePaymentResult struct {
	Success        bool                `json:"success"`
	CapturedAmount decimal.Decimal     `json:"captured_amount"`
	DeclineCode    *string             `json:"decline_code,omitempty"`
	DeclineType    *domain.DeclineType `json:"decline_type,omitempty"`
	ErrorMessage   *string             `json:"error_message,omitempty"`
}

// CapturePayment captures an authorized payment
func (a *Activities) CapturePayment(ctx context.Context, input CapturePaymentInput) (*CapturePaymentResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Capturing payment",
		"payment_intent_id", input.PaymentIntentID,
		"provider_payment_id", input.ProviderPaymentID,
		"amount", input.Amount.String(),
	)

	// TODO: Call actual provider API
	return &CapturePaymentResult{
		Success:        true,
		CapturedAmount: input.Amount,
	}, nil
}

// VoidPaymentInput contains data for void
type VoidPaymentInput struct {
	PaymentIntentID   string          `json:"payment_intent_id"`
	ProviderPaymentID string          `json:"provider_payment_id"`
	Provider          domain.Provider `json:"provider"`
	Reason            string          `json:"reason"`
	IdempotencyKey    string          `json:"idempotency_key"`
}

// VoidPaymentResult contains the void response
type VoidPaymentResult struct {
	Success      bool    `json:"success"`
	ErrorMessage *string `json:"error_message,omitempty"`
}

// VoidPayment voids an authorized payment
func (a *Activities) VoidPayment(ctx context.Context, input VoidPaymentInput) (*VoidPaymentResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Voiding payment",
		"payment_intent_id", input.PaymentIntentID,
		"provider_payment_id", input.ProviderPaymentID,
		"reason", input.Reason,
	)

	// TODO: Call actual provider API
	return &VoidPaymentResult{
		Success: true,
	}, nil
}
