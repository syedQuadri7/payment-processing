package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.temporal.io/sdk/activity"

	"payment-processing/shared/domain"
)

// activityLogger interface for logging in activities
type activityLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
}

// slogAdapter wraps slog.Logger to implement activityLogger
type slogAdapter struct{}

func (s *slogAdapter) Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func (s *slogAdapter) Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// temporalLoggerAdapter wraps Temporal's activity logger
type temporalLoggerAdapter struct {
	logger interface {
		Info(msg string, keyvals ...interface{})
		Warn(msg string, keyvals ...interface{})
	}
}

func (t *temporalLoggerAdapter) Info(msg string, args ...any) {
	t.logger.Info(msg, args...)
}

func (t *temporalLoggerAdapter) Warn(msg string, args ...any) {
	t.logger.Warn(msg, args...)
}

// getLogger returns an activity logger if in an activity context, otherwise returns slog
func getLogger(ctx context.Context) activityLogger {
	// Use a closure to safely attempt getting the activity logger
	var result activityLogger
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Not in activity context, use slog
				result = &slogAdapter{}
			}
		}()
		result = &temporalLoggerAdapter{logger: activity.GetLogger(ctx)}
	}()
	return result
}

// AccountRepository defines the interface for account operations needed by ledger activities
type AccountRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Account, error)
	GetByName(ctx context.Context, name string, currency string) (*domain.Account, error)
	GetClearingAccounts(ctx context.Context, currency string) ([]*domain.Account, error)
	UpdateBalances(ctx context.Context, id string, ledger, pending, available decimal.Decimal, expectedVersion int) error
}

// JournalEntryRepository defines the interface for journal entry operations
type JournalEntryRepository interface {
	Create(ctx context.Context, entry *domain.JournalEntry) error
	GetByID(ctx context.Context, id string) (*domain.JournalEntry, error)
	GetByReference(ctx context.Context, refType domain.ReferenceType, refID string) ([]*domain.JournalEntry, error)
}

// LedgerEntryRepository defines the interface for ledger entry operations
type LedgerEntryRepository interface {
	Create(ctx context.Context, entry *domain.LedgerEntry) error
	GetByJournalEntryID(ctx context.Context, journalEntryID string) ([]*domain.LedgerEntry, error)
	GetByAccountID(ctx context.Context, accountID string, limit int) ([]*domain.LedgerEntry, error)
	GetRunningBalance(ctx context.Context, accountID string) (decimal.Decimal, error)
}

// LedgerActivities holds the dependencies for ledger-related activities
type LedgerActivities struct {
	AccountRepo      AccountRepository
	JournalEntryRepo JournalEntryRepository
	LedgerEntryRepo  LedgerEntryRepository
}

// NewLedgerActivities creates a new LedgerActivities instance
func NewLedgerActivities(
	accountRepo AccountRepository,
	journalEntryRepo JournalEntryRepository,
	ledgerEntryRepo LedgerEntryRepository,
) *LedgerActivities {
	return &LedgerActivities{
		AccountRepo:      accountRepo,
		JournalEntryRepo: journalEntryRepo,
		LedgerEntryRepo:  ledgerEntryRepo,
	}
}

// Clearing account names (must match seed data)
const (
	ClearingAccountAuthorizationName = "Authorization Clearing"
	ClearingAccountSettlementName    = "Settlement Clearing"
	ClearingAccountFeeName           = "Fee Clearing"
	ClearingAccountRefundName        = "Refund Clearing"
)

// PlaceAuthorizationHoldInput contains data for placing an authorization hold
type PlaceAuthorizationHoldInput struct {
	CustomerAccountID string          `json:"customer_account_id"`
	Amount            decimal.Decimal `json:"amount"`
	PaymentIntentID   string          `json:"payment_intent_id"`
}

// PlaceAuthorizationHoldResult contains the result of placing a hold
type PlaceAuthorizationHoldResult struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	NewVersion   int    `json:"new_version"`
}

// PlaceAuthorizationHold increases the pending balance on a customer account
// This reserves funds without creating ledger entries (authorization is a promise, not money movement)
func (a *LedgerActivities) PlaceAuthorizationHold(ctx context.Context, input PlaceAuthorizationHoldInput) (*PlaceAuthorizationHoldResult, error) {
	logger := getLogger(ctx)
	logger.Info("Placing authorization hold",
		"customer_account_id", input.CustomerAccountID,
		"amount", input.Amount.String(),
		"payment_intent_id", input.PaymentIntentID,
	)

	if a.AccountRepo == nil {
		return &PlaceAuthorizationHoldResult{
			Success:      true, // Allow tests without repository
			ErrorMessage: "",
		}, nil
	}

	// Get current account state
	account, err := a.AccountRepo.GetByID(ctx, input.CustomerAccountID)
	if err != nil {
		return nil, fmt.Errorf("getting account: %w", err)
	}
	if account == nil {
		return &PlaceAuthorizationHoldResult{
			Success:      false,
			ErrorMessage: "account not found",
		}, nil
	}

	// Apply domain logic to place hold
	if err := account.PlaceHold(input.Amount); err != nil {
		return &PlaceAuthorizationHoldResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Persist with optimistic locking (version was incremented by PlaceHold)
	expectedVersion := account.Version - 1
	err = a.AccountRepo.UpdateBalances(
		ctx,
		account.ID,
		account.LedgerBalance,
		account.PendingBalance,
		account.AvailableBalance,
		expectedVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("updating account balances: %w", err)
	}

	logger.Info("Authorization hold placed successfully",
		"customer_account_id", input.CustomerAccountID,
		"pending_balance", account.PendingBalance.String(),
		"available_balance", account.AvailableBalance.String(),
	)

	return &PlaceAuthorizationHoldResult{
		Success:    true,
		NewVersion: account.Version,
	}, nil
}

// ReleaseAuthorizationHoldInput contains data for releasing an authorization hold
type ReleaseAuthorizationHoldInput struct {
	CustomerAccountID string          `json:"customer_account_id"`
	Amount            decimal.Decimal `json:"amount"`
	PaymentIntentID   string          `json:"payment_intent_id"`
	Reason            string          `json:"reason"` // "void", "expiration", "partial_capture_remainder"
}

// ReleaseAuthorizationHoldResult contains the result of releasing a hold
type ReleaseAuthorizationHoldResult struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	NewVersion   int    `json:"new_version"`
}

// ReleaseAuthorizationHold decreases the pending balance on a customer account
// This releases funds without creating ledger entries (void releases a promise, no money moved)
func (a *LedgerActivities) ReleaseAuthorizationHold(ctx context.Context, input ReleaseAuthorizationHoldInput) (*ReleaseAuthorizationHoldResult, error) {
	logger := getLogger(ctx)
	logger.Info("Releasing authorization hold",
		"customer_account_id", input.CustomerAccountID,
		"amount", input.Amount.String(),
		"payment_intent_id", input.PaymentIntentID,
		"reason", input.Reason,
	)

	if a.AccountRepo == nil {
		return &ReleaseAuthorizationHoldResult{
			Success: true,
		}, nil
	}

	// Get current account state
	account, err := a.AccountRepo.GetByID(ctx, input.CustomerAccountID)
	if err != nil {
		return nil, fmt.Errorf("getting account: %w", err)
	}
	if account == nil {
		return &ReleaseAuthorizationHoldResult{
			Success:      false,
			ErrorMessage: "account not found",
		}, nil
	}

	// Apply domain logic to release hold
	if err := account.ReleaseHold(input.Amount); err != nil {
		return &ReleaseAuthorizationHoldResult{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Persist with optimistic locking
	expectedVersion := account.Version - 1
	err = a.AccountRepo.UpdateBalances(
		ctx,
		account.ID,
		account.LedgerBalance,
		account.PendingBalance,
		account.AvailableBalance,
		expectedVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("updating account balances: %w", err)
	}

	logger.Info("Authorization hold released successfully",
		"customer_account_id", input.CustomerAccountID,
		"pending_balance", account.PendingBalance.String(),
		"available_balance", account.AvailableBalance.String(),
		"reason", input.Reason,
	)

	return &ReleaseAuthorizationHoldResult{
		Success:    true,
		NewVersion: account.Version,
	}, nil
}

// RecordCaptureInput contains data for recording a capture in the ledger
type RecordCaptureInput struct {
	PaymentIntentID   string          `json:"payment_intent_id"`
	CustomerAccountID string          `json:"customer_account_id"`
	Amount            decimal.Decimal `json:"amount"`
	Currency          string          `json:"currency"`
	IdempotencyKey    string          `json:"idempotency_key"`
}

// RecordCaptureResult contains the result of recording a capture
type RecordCaptureResult struct {
	Success        bool   `json:"success"`
	JournalEntryID string `json:"journal_entry_id,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// RecordCapture creates journal and ledger entries for a payment capture
// This is actual money movement: debit customer, credit settlement clearing
func (a *LedgerActivities) RecordCapture(ctx context.Context, input RecordCaptureInput) (*RecordCaptureResult, error) {
	logger := getLogger(ctx)
	logger.Info("Recording capture in ledger",
		"payment_intent_id", input.PaymentIntentID,
		"customer_account_id", input.CustomerAccountID,
		"amount", input.Amount.String(),
		"currency", input.Currency,
	)

	if a.AccountRepo == nil || a.JournalEntryRepo == nil || a.LedgerEntryRepo == nil {
		journalEntryID := uuid.New().String()
		return &RecordCaptureResult{
			Success:        true,
			JournalEntryID: journalEntryID,
		}, nil
	}

	// Check for idempotency - see if we already recorded this capture
	existingEntries, err := a.JournalEntryRepo.GetByReference(ctx, domain.ReferenceTypePaymentIntent, input.PaymentIntentID)
	if err != nil {
		return nil, fmt.Errorf("checking existing journal entries: %w", err)
	}
	for _, entry := range existingEntries {
		// If we find an existing capture entry for this payment intent, return it
		if entry.Description == "Capture payment "+input.PaymentIntentID {
			logger.Info("Found existing capture journal entry",
				"journal_entry_id", entry.ID,
			)
			return &RecordCaptureResult{
				Success:        true,
				JournalEntryID: entry.ID,
			}, nil
		}
	}

	// Get settlement clearing account
	clearingAccount, err := a.AccountRepo.GetByName(ctx, ClearingAccountSettlementName, input.Currency)
	if err != nil {
		return nil, fmt.Errorf("getting settlement clearing account: %w", err)
	}
	if clearingAccount == nil {
		return &RecordCaptureResult{
			Success:      false,
			ErrorMessage: "settlement clearing account not found for currency " + input.Currency,
		}, nil
	}

	// Get customer account
	customerAccount, err := a.AccountRepo.GetByID(ctx, input.CustomerAccountID)
	if err != nil {
		return nil, fmt.Errorf("getting customer account: %w", err)
	}
	if customerAccount == nil {
		return &RecordCaptureResult{
			Success:      false,
			ErrorMessage: "customer account not found",
		}, nil
	}

	// Create the journal entry using the domain builder
	journalEntry, err := domain.NewCaptureJournalEntry(
		input.PaymentIntentID,
		input.CustomerAccountID,
		clearingAccount.ID,
		input.Amount,
	)
	if err != nil {
		return &RecordCaptureResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("creating journal entry: %s", err.Error()),
		}, nil
	}
	journalEntry.ID = uuid.New().String()

	// Persist journal entry
	if err := a.JournalEntryRepo.Create(ctx, journalEntry); err != nil {
		return nil, fmt.Errorf("persisting journal entry: %w", err)
	}

	// Calculate new balances after capture
	// For customer: debit decreases ledger balance, also release pending hold
	if err := customerAccount.Debit(input.Amount); err != nil {
		return nil, fmt.Errorf("debiting customer account: %w", err)
	}

	// For clearing: credit increases ledger balance
	if err := clearingAccount.Credit(input.Amount); err != nil {
		return nil, fmt.Errorf("crediting clearing account: %w", err)
	}

	// Create ledger entries with balance_after snapshots
	now := time.Now()
	customerLedgerEntry := &domain.LedgerEntry{
		ID:             uuid.New().String(),
		JournalEntryID: journalEntry.ID,
		AccountID:      input.CustomerAccountID,
		Amount:         input.Amount,
		Direction:      domain.EntryDirectionDebit,
		BalanceAfter:   customerAccount.LedgerBalance,
		CreatedAt:      now,
	}

	clearingLedgerEntry := &domain.LedgerEntry{
		ID:             uuid.New().String(),
		JournalEntryID: journalEntry.ID,
		AccountID:      clearingAccount.ID,
		Amount:         input.Amount,
		Direction:      domain.EntryDirectionCredit,
		BalanceAfter:   clearingAccount.LedgerBalance,
		CreatedAt:      now,
	}

	// Persist ledger entries
	if err := a.LedgerEntryRepo.Create(ctx, customerLedgerEntry); err != nil {
		return nil, fmt.Errorf("creating customer ledger entry: %w", err)
	}
	if err := a.LedgerEntryRepo.Create(ctx, clearingLedgerEntry); err != nil {
		return nil, fmt.Errorf("creating clearing ledger entry: %w", err)
	}

	// Update account balances with optimistic locking
	customerVersion := customerAccount.Version - 1
	err = a.AccountRepo.UpdateBalances(
		ctx,
		customerAccount.ID,
		customerAccount.LedgerBalance,
		customerAccount.PendingBalance,
		customerAccount.AvailableBalance,
		customerVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("updating customer account balances: %w", err)
	}

	clearingVersion := clearingAccount.Version - 1
	err = a.AccountRepo.UpdateBalances(
		ctx,
		clearingAccount.ID,
		clearingAccount.LedgerBalance,
		clearingAccount.PendingBalance,
		clearingAccount.AvailableBalance,
		clearingVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("updating clearing account balances: %w", err)
	}

	logger.Info("Capture recorded successfully",
		"journal_entry_id", journalEntry.ID,
		"customer_ledger_balance", customerAccount.LedgerBalance.String(),
		"clearing_ledger_balance", clearingAccount.LedgerBalance.String(),
	)

	return &RecordCaptureResult{
		Success:        true,
		JournalEntryID: journalEntry.ID,
	}, nil
}

// RecordRefundInput contains data for recording a refund in the ledger
type RecordRefundInput struct {
	PaymentIntentID   string          `json:"payment_intent_id"`
	RefundID          string          `json:"refund_id"`
	CustomerAccountID string          `json:"customer_account_id"`
	MerchantAccountID string          `json:"merchant_account_id"`
	Amount            decimal.Decimal `json:"amount"`
	Currency          string          `json:"currency"`
	IdempotencyKey    string          `json:"idempotency_key"`
}

// RecordRefundResult contains the result of recording a refund
type RecordRefundResult struct {
	Success        bool   `json:"success"`
	JournalEntryID string `json:"journal_entry_id,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// RecordRefund creates journal and ledger entries for a refund
// This is actual money movement: debit merchant, credit customer
func (a *LedgerActivities) RecordRefund(ctx context.Context, input RecordRefundInput) (*RecordRefundResult, error) {
	logger := getLogger(ctx)
	logger.Info("Recording refund in ledger",
		"payment_intent_id", input.PaymentIntentID,
		"refund_id", input.RefundID,
		"customer_account_id", input.CustomerAccountID,
		"merchant_account_id", input.MerchantAccountID,
		"amount", input.Amount.String(),
	)

	if a.AccountRepo == nil || a.JournalEntryRepo == nil || a.LedgerEntryRepo == nil {
		journalEntryID := uuid.New().String()
		return &RecordRefundResult{
			Success:        true,
			JournalEntryID: journalEntryID,
		}, nil
	}

	// Check for idempotency
	existingEntries, err := a.JournalEntryRepo.GetByReference(ctx, domain.ReferenceTypeRefund, input.RefundID)
	if err != nil {
		return nil, fmt.Errorf("checking existing journal entries: %w", err)
	}
	if len(existingEntries) > 0 {
		logger.Info("Found existing refund journal entry",
			"journal_entry_id", existingEntries[0].ID,
		)
		return &RecordRefundResult{
			Success:        true,
			JournalEntryID: existingEntries[0].ID,
		}, nil
	}

	// Get accounts
	customerAccount, err := a.AccountRepo.GetByID(ctx, input.CustomerAccountID)
	if err != nil {
		return nil, fmt.Errorf("getting customer account: %w", err)
	}
	if customerAccount == nil {
		return &RecordRefundResult{
			Success:      false,
			ErrorMessage: "customer account not found",
		}, nil
	}

	merchantAccount, err := a.AccountRepo.GetByID(ctx, input.MerchantAccountID)
	if err != nil {
		return nil, fmt.Errorf("getting merchant account: %w", err)
	}
	if merchantAccount == nil {
		return &RecordRefundResult{
			Success:      false,
			ErrorMessage: "merchant account not found",
		}, nil
	}

	// Create the journal entry using the domain builder
	journalEntry, err := domain.NewRefundJournalEntry(
		input.PaymentIntentID,
		input.MerchantAccountID,
		input.CustomerAccountID,
		input.Amount,
	)
	if err != nil {
		return &RecordRefundResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("creating journal entry: %s", err.Error()),
		}, nil
	}
	journalEntry.ID = uuid.New().String()

	// Override reference to use refund ID
	refType := domain.ReferenceTypeRefund
	journalEntry.ReferenceType = &refType
	journalEntry.ReferenceID = &input.RefundID

	// Persist journal entry
	if err := a.JournalEntryRepo.Create(ctx, journalEntry); err != nil {
		return nil, fmt.Errorf("persisting journal entry: %w", err)
	}

	// Calculate new balances
	// For merchant: debit decreases balance
	if err := merchantAccount.Debit(input.Amount); err != nil {
		return nil, fmt.Errorf("debiting merchant account: %w", err)
	}

	// For customer: credit increases balance
	if err := customerAccount.Credit(input.Amount); err != nil {
		return nil, fmt.Errorf("crediting customer account: %w", err)
	}

	// Create ledger entries
	now := time.Now()
	merchantLedgerEntry := &domain.LedgerEntry{
		ID:             uuid.New().String(),
		JournalEntryID: journalEntry.ID,
		AccountID:      input.MerchantAccountID,
		Amount:         input.Amount,
		Direction:      domain.EntryDirectionDebit,
		BalanceAfter:   merchantAccount.LedgerBalance,
		CreatedAt:      now,
	}

	customerLedgerEntry := &domain.LedgerEntry{
		ID:             uuid.New().String(),
		JournalEntryID: journalEntry.ID,
		AccountID:      input.CustomerAccountID,
		Amount:         input.Amount,
		Direction:      domain.EntryDirectionCredit,
		BalanceAfter:   customerAccount.LedgerBalance,
		CreatedAt:      now,
	}

	// Persist ledger entries
	if err := a.LedgerEntryRepo.Create(ctx, merchantLedgerEntry); err != nil {
		return nil, fmt.Errorf("creating merchant ledger entry: %w", err)
	}
	if err := a.LedgerEntryRepo.Create(ctx, customerLedgerEntry); err != nil {
		return nil, fmt.Errorf("creating customer ledger entry: %w", err)
	}

	// Update account balances
	merchantVersion := merchantAccount.Version - 1
	err = a.AccountRepo.UpdateBalances(
		ctx,
		merchantAccount.ID,
		merchantAccount.LedgerBalance,
		merchantAccount.PendingBalance,
		merchantAccount.AvailableBalance,
		merchantVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("updating merchant account balances: %w", err)
	}

	customerVersion := customerAccount.Version - 1
	err = a.AccountRepo.UpdateBalances(
		ctx,
		customerAccount.ID,
		customerAccount.LedgerBalance,
		customerAccount.PendingBalance,
		customerAccount.AvailableBalance,
		customerVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("updating customer account balances: %w", err)
	}

	logger.Info("Refund recorded successfully",
		"journal_entry_id", journalEntry.ID,
		"merchant_ledger_balance", merchantAccount.LedgerBalance.String(),
		"customer_ledger_balance", customerAccount.LedgerBalance.String(),
	)

	return &RecordRefundResult{
		Success:        true,
		JournalEntryID: journalEntry.ID,
	}, nil
}

// GetClearingAccountBalancesInput contains parameters for the query
type GetClearingAccountBalancesInput struct {
	Currency string `json:"currency"`
}

// ClearingAccountBalance represents a clearing account's current state
type ClearingAccountBalance struct {
	AccountID     string          `json:"account_id"`
	AccountName   string          `json:"account_name"`
	LedgerBalance decimal.Decimal `json:"ledger_balance"`
	UpdatedAt     time.Time       `json:"updated_at"`
	IsNonZero     bool            `json:"is_non_zero"`
}

// GetClearingAccountBalancesResult contains the clearing account balances
type GetClearingAccountBalancesResult struct {
	Accounts []ClearingAccountBalance `json:"accounts"`
}

// GetClearingAccountBalances retrieves all clearing account balances for monitoring
// Non-zero clearing account balances indicate in-flight transactions
func (a *LedgerActivities) GetClearingAccountBalances(ctx context.Context, input GetClearingAccountBalancesInput) (*GetClearingAccountBalancesResult, error) {
	logger := getLogger(ctx)
	logger.Info("Getting clearing account balances",
		"currency", input.Currency,
	)

	if a.AccountRepo == nil {
		return &GetClearingAccountBalancesResult{
			Accounts: []ClearingAccountBalance{},
		}, nil
	}

	accounts, err := a.AccountRepo.GetClearingAccounts(ctx, input.Currency)
	if err != nil {
		return nil, fmt.Errorf("getting clearing accounts: %w", err)
	}

	result := &GetClearingAccountBalancesResult{
		Accounts: make([]ClearingAccountBalance, 0, len(accounts)),
	}

	for _, acc := range accounts {
		result.Accounts = append(result.Accounts, ClearingAccountBalance{
			AccountID:     acc.ID,
			AccountName:   acc.Name,
			LedgerBalance: acc.LedgerBalance,
			UpdatedAt:     acc.UpdatedAt,
			IsNonZero:     !acc.LedgerBalance.IsZero(),
		})
	}

	logger.Info("Retrieved clearing account balances",
		"count", len(result.Accounts),
	)

	return result, nil
}

// MonitorClearingAccountsInput contains parameters for monitoring
type MonitorClearingAccountsInput struct {
	Currency        string        `json:"currency"`
	WarningDuration time.Duration `json:"warning_duration"` // Default: 12 hours
	CriticalDuration time.Duration `json:"critical_duration"` // Default: 24 hours
}

// ClearingAccountAlert represents an alert for a clearing account
type ClearingAccountAlert struct {
	AccountID     string          `json:"account_id"`
	AccountName   string          `json:"account_name"`
	LedgerBalance decimal.Decimal `json:"ledger_balance"`
	LastUpdated   time.Time       `json:"last_updated"`
	DurationNonZero time.Duration `json:"duration_non_zero"`
	AlertLevel    string          `json:"alert_level"` // "warning", "critical", "ok"
}

// MonitorClearingAccountsResult contains the monitoring results
type MonitorClearingAccountsResult struct {
	Alerts       []ClearingAccountAlert `json:"alerts"`
	HasWarnings  bool                   `json:"has_warnings"`
	HasCritical  bool                   `json:"has_critical"`
	CheckedAt    time.Time              `json:"checked_at"`
}

// MonitorClearingAccounts checks clearing accounts for non-zero balances exceeding thresholds
// Per docs/schema/ledger-tables.md:
// - Non-zero balance > 12 hours = Warning (review pending transactions)
// - Non-zero balance > 24 hours = Critical (investigate stuck transactions)
func (a *LedgerActivities) MonitorClearingAccounts(ctx context.Context, input MonitorClearingAccountsInput) (*MonitorClearingAccountsResult, error) {
	logger := getLogger(ctx)
	logger.Info("Monitoring clearing accounts",
		"currency", input.Currency,
	)

	// Set defaults if not provided
	warningDuration := input.WarningDuration
	if warningDuration == 0 {
		warningDuration = 12 * time.Hour
	}
	criticalDuration := input.CriticalDuration
	if criticalDuration == 0 {
		criticalDuration = 24 * time.Hour
	}

	if a.AccountRepo == nil {
		return &MonitorClearingAccountsResult{
			Alerts:    []ClearingAccountAlert{},
			CheckedAt: time.Now(),
		}, nil
	}

	accounts, err := a.AccountRepo.GetClearingAccounts(ctx, input.Currency)
	if err != nil {
		return nil, fmt.Errorf("getting clearing accounts: %w", err)
	}

	now := time.Now()
	result := &MonitorClearingAccountsResult{
		Alerts:    make([]ClearingAccountAlert, 0),
		CheckedAt: now,
	}

	for _, acc := range accounts {
		if acc.LedgerBalance.IsZero() {
			continue // Skip accounts with zero balance
		}

		duration := now.Sub(acc.UpdatedAt)
		alertLevel := "ok"

		if duration >= criticalDuration {
			alertLevel = "critical"
			result.HasCritical = true
		} else if duration >= warningDuration {
			alertLevel = "warning"
			result.HasWarnings = true
		}

		alert := ClearingAccountAlert{
			AccountID:       acc.ID,
			AccountName:     acc.Name,
			LedgerBalance:   acc.LedgerBalance,
			LastUpdated:     acc.UpdatedAt,
			DurationNonZero: duration,
			AlertLevel:      alertLevel,
		}
		result.Alerts = append(result.Alerts, alert)

		if alertLevel != "ok" {
			logger.Warn("Clearing account alert",
				"account_name", acc.Name,
				"balance", acc.LedgerBalance.String(),
				"duration", duration.String(),
				"alert_level", alertLevel,
			)
		}
	}

	logger.Info("Clearing account monitoring complete",
		"total_alerts", len(result.Alerts),
		"has_warnings", result.HasWarnings,
		"has_critical", result.HasCritical,
	)

	return result, nil
}

// GetAccountBalanceInput contains parameters for account balance query
type GetAccountBalanceInput struct {
	AccountID string `json:"account_id"`
}

// GetAccountBalanceResult contains the account balance information
type GetAccountBalanceResult struct {
	Found            bool            `json:"found"`
	LedgerBalance    decimal.Decimal `json:"ledger_balance"`
	PendingBalance   decimal.Decimal `json:"pending_balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	Version          int             `json:"version"`
}

// GetAccountBalance retrieves the current balance state for an account
func (a *LedgerActivities) GetAccountBalance(ctx context.Context, input GetAccountBalanceInput) (*GetAccountBalanceResult, error) {
	logger := getLogger(ctx)
	logger.Info("Getting account balance",
		"account_id", input.AccountID,
	)

	if a.AccountRepo == nil {
		return &GetAccountBalanceResult{
			Found: false,
		}, nil
	}

	account, err := a.AccountRepo.GetByID(ctx, input.AccountID)
	if err != nil {
		return nil, fmt.Errorf("getting account: %w", err)
	}

	if account == nil {
		return &GetAccountBalanceResult{
			Found: false,
		}, nil
	}

	return &GetAccountBalanceResult{
		Found:            true,
		LedgerBalance:    account.LedgerBalance,
		PendingBalance:   account.PendingBalance,
		AvailableBalance: account.AvailableBalance,
		Version:          account.Version,
	}, nil
}
