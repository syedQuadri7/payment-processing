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

// LedgerEntryRepository implements domain.LedgerEntryRepository
type LedgerEntryRepository struct {
	pool *pgxpool.Pool
}

// NewLedgerEntryRepository creates a new LedgerEntryRepository
func NewLedgerEntryRepository(pool *pgxpool.Pool) *LedgerEntryRepository {
	return &LedgerEntryRepository{pool: pool}
}

// Create inserts a new ledger entry (append-only)
// Note: The database has rules preventing updates and deletes
func (r *LedgerEntryRepository) Create(ctx context.Context, entry *domain.LedgerEntry) error {
	if entry.Amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("ledger entry amount must be positive")
	}

	query := `
		INSERT INTO ledger_entries (
			id, journal_entry_id, account_id, amount, direction, balance_after, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, query,
		entry.ID, entry.JournalEntryID, entry.AccountID,
		entry.Amount, entry.Direction, entry.BalanceAfter, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting ledger entry: %w", err)
	}

	return nil
}

// CreateBatch inserts multiple ledger entries in a single operation
func (r *LedgerEntryRepository) CreateBatch(ctx context.Context, entries []*domain.LedgerEntry) error {
	if len(entries) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	query := `
		INSERT INTO ledger_entries (
			id, journal_entry_id, account_id, amount, direction, balance_after, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	now := time.Now()
	for _, entry := range entries {
		if entry.Amount.LessThanOrEqual(decimal.Zero) {
			return fmt.Errorf("ledger entry amount must be positive")
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = now
		}
		batch.Queue(query,
			entry.ID, entry.JournalEntryID, entry.AccountID,
			entry.Amount, entry.Direction, entry.BalanceAfter, entry.CreatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range entries {
		_, err := results.Exec()
		if err != nil {
			return fmt.Errorf("inserting ledger entry: %w", err)
		}
	}

	return nil
}

// GetByJournalEntryID retrieves all ledger entries for a journal entry
func (r *LedgerEntryRepository) GetByJournalEntryID(ctx context.Context, journalEntryID string) ([]*domain.LedgerEntry, error) {
	query := `
		SELECT id, journal_entry_id, account_id, amount, direction, balance_after, created_at
		FROM ledger_entries
		WHERE journal_entry_id = $1
		ORDER BY created_at
	`

	rows, err := r.pool.Query(ctx, query, journalEntryID)
	if err != nil {
		return nil, fmt.Errorf("querying ledger entries: %w", err)
	}
	defer rows.Close()

	return r.scanLedgerEntries(rows)
}

// GetByAccountID retrieves ledger entries for an account with a limit
func (r *LedgerEntryRepository) GetByAccountID(ctx context.Context, accountID string, limit int) ([]*domain.LedgerEntry, error) {
	query := `
		SELECT id, journal_entry_id, account_id, amount, direction, balance_after, created_at
		FROM ledger_entries
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying ledger entries by account: %w", err)
	}
	defer rows.Close()

	return r.scanLedgerEntries(rows)
}

// GetRunningBalance calculates the running balance for an account
// This sums all ledger entries for the account
func (r *LedgerEntryRepository) GetRunningBalance(ctx context.Context, accountID string) (decimal.Decimal, error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0) as debits,
			COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0) as credits
		FROM ledger_entries
		WHERE account_id = $1
	`

	var debitsStr, creditsStr string
	err := r.pool.QueryRow(ctx, query, accountID).Scan(&debitsStr, &creditsStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, nil
		}
		return decimal.Zero, fmt.Errorf("calculating running balance: %w", err)
	}

	debits, err := parseDecimal(debitsStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parsing debits: %w", err)
	}
	credits, err := parseDecimal(creditsStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parsing credits: %w", err)
	}

	// Balance = Credits - Debits for most account types
	// (The actual balance direction depends on account type)
	return credits.Sub(debits), nil
}

// GetLatestBalanceForAccount retrieves the most recent balance_after for an account
func (r *LedgerEntryRepository) GetLatestBalanceForAccount(ctx context.Context, accountID string) (decimal.Decimal, error) {
	query := `
		SELECT balance_after
		FROM ledger_entries
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var balanceStr string
	err := r.pool.QueryRow(ctx, query, accountID).Scan(&balanceStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return decimal.Zero, nil
		}
		return decimal.Zero, fmt.Errorf("querying latest balance: %w", err)
	}

	return parseDecimal(balanceStr)
}

// GetByTimeRange retrieves ledger entries within a time range for an account
func (r *LedgerEntryRepository) GetByTimeRange(ctx context.Context, accountID string, start, end time.Time) ([]*domain.LedgerEntry, error) {
	query := `
		SELECT id, journal_entry_id, account_id, amount, direction, balance_after, created_at
		FROM ledger_entries
		WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
		ORDER BY created_at
	`

	rows, err := r.pool.Query(ctx, query, accountID, start, end)
	if err != nil {
		return nil, fmt.Errorf("querying ledger entries by time range: %w", err)
	}
	defer rows.Close()

	return r.scanLedgerEntries(rows)
}

// scanLedgerEntries scans multiple ledger entries from rows
func (r *LedgerEntryRepository) scanLedgerEntries(rows pgx.Rows) ([]*domain.LedgerEntry, error) {
	var entries []*domain.LedgerEntry
	for rows.Next() {
		entry := &domain.LedgerEntry{}
		var amountStr, balanceAfterStr string

		err := rows.Scan(
			&entry.ID, &entry.JournalEntryID, &entry.AccountID,
			&amountStr, &entry.Direction, &balanceAfterStr, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning ledger entry: %w", err)
		}

		var parseErr error
		entry.Amount, parseErr = parseDecimal(amountStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing amount: %w", parseErr)
		}
		entry.BalanceAfter, parseErr = parseDecimal(balanceAfterStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing balance_after: %w", parseErr)
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// parseDecimal parses a string to decimal
func parseDecimal(s string) (decimal.Decimal, error) {
	return decimal.NewFromString(s)
}
