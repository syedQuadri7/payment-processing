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

// AccountRepository defines operations for ledger accounts
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id string) (*Account, error)
	GetByOwnerID(ctx context.Context, ownerID string) ([]*Account, error)
	GetByName(ctx context.Context, name string, currency string) (*Account, error)
	GetClearingAccounts(ctx context.Context, currency string) ([]*Account, error)
	UpdateBalances(ctx context.Context, id string, ledger, pending, available decimal.Decimal, expectedVersion int) error
	UpdateStatus(ctx context.Context, id string, status AccountStatus) error
}

// JournalEntryRepository defines operations for journal entries
type JournalEntryRepository interface {
	Create(ctx context.Context, entry *JournalEntry) error
	GetByID(ctx context.Context, id string) (*JournalEntry, error)
	GetByReference(ctx context.Context, refType ReferenceType, refID string) ([]*JournalEntry, error)
}

// LedgerEntryRepository defines operations for ledger entries
type LedgerEntryRepository interface {
	Create(ctx context.Context, entry *LedgerEntry) error
	GetByJournalEntryID(ctx context.Context, journalEntryID string) ([]*LedgerEntry, error)
	GetByAccountID(ctx context.Context, accountID string, limit int) ([]*LedgerEntry, error)
	GetRunningBalance(ctx context.Context, accountID string) (decimal.Decimal, error)
}

// OutboxRepository defines operations for the transactional outbox
type OutboxRepository interface {
	Create(ctx context.Context, event *OutboxEvent) error
	GetUnpublished(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkPublished(ctx context.Context, ids []string) error
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// AuditLogRepository defines operations for the audit log
type AuditLogRepository interface {
	Create(ctx context.Context, entry *AuditLogEntry) error
	GetByEntity(ctx context.Context, entityType AuditEntityType, entityID string) ([]*AuditLogEntry, error)
	GetByActor(ctx context.Context, actorType AuditActorType, actorID string, limit int) ([]*AuditLogEntry, error)
	GetByTimeRange(ctx context.Context, start, end time.Time, limit int) ([]*AuditLogEntry, error)
}

// ProcessedEventRepository defines operations for tracking processed webhook events
type ProcessedEventRepository interface {
	Create(ctx context.Context, event *ProcessedEvent) error
	Exists(ctx context.Context, provider Provider, eventID string) (bool, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// Repositories bundles all repository interfaces
type Repositories struct {
	PaymentIntents     PaymentIntentRepository
	PaymentMethods     PaymentMethodRepository
	AuthorizationHolds AuthorizationHoldRepository
	PaymentAttempts    PaymentAttemptRepository
	DeclineCodes       DeclineCodeRepository
	Accounts           AccountRepository
	JournalEntries     JournalEntryRepository
	LedgerEntries      LedgerEntryRepository
	Outbox             OutboxRepository
	AuditLog           AuditLogRepository
	ProcessedEvents    ProcessedEventRepository
}
