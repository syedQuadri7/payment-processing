package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"payment-processing/shared/domain"
)

// ErrOptimisticLock is returned when an optimistic lock conflict occurs
var ErrOptimisticLock = errors.New("optimistic lock conflict: record was modified by another process")

// AccountRepository implements domain.AccountRepository
type AccountRepository struct {
	pool *pgxpool.Pool
}

// NewAccountRepository creates a new AccountRepository
func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

// Create inserts a new account
func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO accounts (
			id, type, owner_id, name, currency,
			ledger_balance, pending_balance, available_balance, reserved_balance,
			daily_limit, status, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	now := time.Now()
	if account.CreatedAt.IsZero() {
		account.CreatedAt = now
	}
	account.UpdatedAt = now

	if account.Version == 0 {
		account.Version = 1
	}

	_, err := r.pool.Exec(ctx, query,
		account.ID, account.Type, account.OwnerID, account.Name, account.Currency,
		account.LedgerBalance, account.PendingBalance, account.AvailableBalance, account.ReservedBalance,
		account.DailyLimit, account.Status, account.Version, account.CreatedAt, account.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting account: %w", err)
	}

	return nil
}

// GetByID retrieves an account by ID
func (r *AccountRepository) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	query := `
		SELECT id, type, owner_id, name, currency,
			ledger_balance, pending_balance, available_balance, reserved_balance,
			daily_limit, status, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
	`

	return r.scanAccount(r.pool.QueryRow(ctx, query, id))
}

// GetByOwnerID retrieves all accounts for an owner
func (r *AccountRepository) GetByOwnerID(ctx context.Context, ownerID string) ([]*domain.Account, error) {
	query := `
		SELECT id, type, owner_id, name, currency,
			ledger_balance, pending_balance, available_balance, reserved_balance,
			daily_limit, status, version, created_at, updated_at
		FROM accounts
		WHERE owner_id = $1
		ORDER BY type, currency
	`

	rows, err := r.pool.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("querying accounts by owner: %w", err)
	}
	defer rows.Close()

	return r.scanAccounts(rows)
}

// GetByName retrieves a system account by name and currency
func (r *AccountRepository) GetByName(ctx context.Context, name string, currency string) (*domain.Account, error) {
	query := `
		SELECT id, type, owner_id, name, currency,
			ledger_balance, pending_balance, available_balance, reserved_balance,
			daily_limit, status, version, created_at, updated_at
		FROM accounts
		WHERE name = $1 AND currency = $2 AND owner_id IS NULL
	`

	return r.scanAccount(r.pool.QueryRow(ctx, query, name, currency))
}

// GetClearingAccounts retrieves all clearing accounts for a currency
func (r *AccountRepository) GetClearingAccounts(ctx context.Context, currency string) ([]*domain.Account, error) {
	query := `
		SELECT id, type, owner_id, name, currency,
			ledger_balance, pending_balance, available_balance, reserved_balance,
			daily_limit, status, version, created_at, updated_at
		FROM accounts
		WHERE type = 'CLEARING' AND currency = $1
		ORDER BY name
	`

	rows, err := r.pool.Query(ctx, query, currency)
	if err != nil {
		return nil, fmt.Errorf("querying clearing accounts: %w", err)
	}
	defer rows.Close()

	return r.scanAccounts(rows)
}

// UpdateBalances updates account balances with optimistic locking
// Returns ErrOptimisticLock if the version has changed
func (r *AccountRepository) UpdateBalances(ctx context.Context, id string, ledger, pending, available decimal.Decimal, expectedVersion int) error {
	query := `
		UPDATE accounts
		SET ledger_balance = $2, pending_balance = $3, available_balance = $4,
			version = version + 1, updated_at = $5
		WHERE id = $1 AND version = $6
	`

	result, err := r.pool.Exec(ctx, query, id, ledger, pending, available, time.Now(), expectedVersion)
	if err != nil {
		return fmt.Errorf("updating account balances: %w", err)
	}

	if result.RowsAffected() == 0 {
		// Check if account exists
		existing, err := r.GetByID(ctx, id)
		if err != nil {
			return fmt.Errorf("checking account existence: %w", err)
		}
		if existing == nil {
			return fmt.Errorf("account not found: %s", id)
		}
		// Account exists but version mismatch
		return ErrOptimisticLock
	}

	return nil
}

// UpdateStatus updates an account's status
func (r *AccountRepository) UpdateStatus(ctx context.Context, id string, status domain.AccountStatus) error {
	query := `
		UPDATE accounts
		SET status = $2, version = version + 1, updated_at = $3
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, status, time.Now())
	if err != nil {
		return fmt.Errorf("updating account status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("account not found: %s", id)
	}

	return nil
}

// GetByIDForUpdate retrieves an account by ID with a row lock (SELECT FOR UPDATE)
func (r *AccountRepository) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id string) (*domain.Account, error) {
	query := `
		SELECT id, type, owner_id, name, currency,
			ledger_balance, pending_balance, available_balance, reserved_balance,
			daily_limit, status, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`

	return r.scanAccount(tx.QueryRow(ctx, query, id))
}

// scanAccount scans a single account from a row
func (r *AccountRepository) scanAccount(row pgx.Row) (*domain.Account, error) {
	account := &domain.Account{}
	var ledgerStr, pendingStr, availableStr, reservedStr string
	var dailyLimitStr *string

	err := row.Scan(
		&account.ID, &account.Type, &account.OwnerID, &account.Name, &account.Currency,
		&ledgerStr, &pendingStr, &availableStr, &reservedStr,
		&dailyLimitStr, &account.Status, &account.Version, &account.CreatedAt, &account.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning account: %w", err)
	}

	var parseErr error
	account.LedgerBalance, parseErr = decimal.NewFromString(ledgerStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing ledger balance: %w", parseErr)
	}
	account.PendingBalance, parseErr = decimal.NewFromString(pendingStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing pending balance: %w", parseErr)
	}
	account.AvailableBalance, parseErr = decimal.NewFromString(availableStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing available balance: %w", parseErr)
	}
	account.ReservedBalance, parseErr = decimal.NewFromString(reservedStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing reserved balance: %w", parseErr)
	}
	if dailyLimitStr != nil {
		dailyLimit, parseErr := decimal.NewFromString(*dailyLimitStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing daily limit: %w", parseErr)
		}
		account.DailyLimit = &dailyLimit
	}

	return account, nil
}

// scanAccounts scans multiple accounts from rows
func (r *AccountRepository) scanAccounts(rows pgx.Rows) ([]*domain.Account, error) {
	var accounts []*domain.Account
	for rows.Next() {
		account := &domain.Account{}
		var ledgerStr, pendingStr, availableStr, reservedStr string
		var dailyLimitStr *string

		err := rows.Scan(
			&account.ID, &account.Type, &account.OwnerID, &account.Name, &account.Currency,
			&ledgerStr, &pendingStr, &availableStr, &reservedStr,
			&dailyLimitStr, &account.Status, &account.Version, &account.CreatedAt, &account.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}

		var parseErr error
		account.LedgerBalance, parseErr = decimal.NewFromString(ledgerStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing ledger balance: %w", parseErr)
		}
		account.PendingBalance, parseErr = decimal.NewFromString(pendingStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing pending balance: %w", parseErr)
		}
		account.AvailableBalance, parseErr = decimal.NewFromString(availableStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing available balance: %w", parseErr)
		}
		account.ReservedBalance, parseErr = decimal.NewFromString(reservedStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing reserved balance: %w", parseErr)
		}
		if dailyLimitStr != nil {
			dailyLimit, parseErr := decimal.NewFromString(*dailyLimitStr)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing daily limit: %w", parseErr)
			}
			account.DailyLimit = &dailyLimit
		}

		accounts = append(accounts, account)
	}

	return accounts, rows.Err()
}
