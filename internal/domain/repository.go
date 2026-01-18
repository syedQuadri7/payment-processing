package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// PaymentIntentRepository defines operations for payment intents
type PaymentIntentRepository interface {
	Create(ctx context.Context, intent *PaymentIntent) error
	GetByID(ctx context.Context, id string) (*PaymentIntent, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*PaymentIntent, error)
	GetByWorkflowID(ctx context.Context, workflowID string) (*PaymentIntent, error)
	Update(ctx context.Context, intent *PaymentIntent) error
	UpdateStatus(ctx context.Context, id string, status PaymentIntentStatus) error
}

// PaymentMethodRepository defines operations for payment methods
type PaymentMethodRepository interface {
	Create(ctx context.Context, method *PaymentMethod) error
	GetByID(ctx context.Context, id string) (*PaymentMethod, error)
	GetByCustomerID(ctx context.Context, customerID string) ([]*PaymentMethod, error)
	GetDefaultForCustomer(ctx context.Context, customerID string) (*PaymentMethod, error)
	Update(ctx context.Context, method *PaymentMethod) error
	Delete(ctx context.Context, id string) error
}

// AuthorizationHoldRepository defines operations for authorization holds
type AuthorizationHoldRepository interface {
	Create(ctx context.Context, hold *AuthorizationHold) error
	GetByID(ctx context.Context, id string) (*AuthorizationHold, error)
	GetByPaymentIntentID(ctx context.Context, intentID string) (*AuthorizationHold, error)
	GetActiveByPaymentIntentID(ctx context.Context, intentID string) (*AuthorizationHold, error)
	UpdateStatus(ctx context.Context, id string, status HoldStatus) error
	UpdateCaptured(ctx context.Context, id string, amount decimal.Decimal, capturedAt *time.Time) error
}

// PaymentAttemptRepository defines operations for payment attempts
type PaymentAttemptRepository interface {
	Create(ctx context.Context, attempt *PaymentAttempt) error
	GetByID(ctx context.Context, id string) (*PaymentAttempt, error)
	GetByPaymentIntentID(ctx context.Context, intentID string) ([]*PaymentAttempt, error)
	GetLatestByPaymentIntentID(ctx context.Context, intentID string) (*PaymentAttempt, error)
	CountByPaymentIntentID(ctx context.Context, intentID string) (int, error)
	MarkCompleted(ctx context.Context, id string, status AttemptStatus, providerCode, declineCode *string, declineType *DeclineType) error
}

// DeclineCodeRepository defines operations for decline code mappings
type DeclineCodeRepository interface {
	GetByProviderCode(ctx context.Context, provider Provider, code string) (*DeclineCodeMapping, error)
	GetAll(ctx context.Context) ([]*DeclineCodeMapping, error)
	GetByProvider(ctx context.Context, provider Provider) ([]*DeclineCodeMapping, error)
}

// Repositories bundles all repository interfaces
type Repositories struct {
	PaymentIntents     PaymentIntentRepository
	PaymentMethods     PaymentMethodRepository
	AuthorizationHolds AuthorizationHoldRepository
	PaymentAttempts    PaymentAttemptRepository
	DeclineCodes       DeclineCodeRepository
}
