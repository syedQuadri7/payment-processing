package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payment-processing/pkg/domain"
)

// JournalEntryRepository implements domain.JournalEntryRepository
type JournalEntryRepository struct {
	pool *pgxpool.Pool
}

// NewJournalEntryRepository creates a new JournalEntryRepository
func NewJournalEntryRepository(pool *pgxpool.Pool) *JournalEntryRepository {
	return &JournalEntryRepository{pool: pool}
}

// Create inserts a new journal entry
// Note: This only creates the journal entry record, not the ledger entries
// Ledger entries should be created separately using LedgerEntryRepository
func (r *JournalEntryRepository) Create(ctx context.Context, entry *domain.JournalEntry) error {
	// Validate the journal entry first
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("invalid journal entry: %w", err)
	}

	query := `
		INSERT INTO journal_entries (
			id, description, reference_type, reference_id, posted_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	now := time.Now()
	if entry.PostedAt.IsZero() {
		entry.PostedAt = now
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}

	_, err := r.pool.Exec(ctx, query,
		entry.ID, entry.Description, entry.ReferenceType, entry.ReferenceID,
		entry.PostedAt, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting journal entry: %w", err)
	}

	return nil
}

// GetByID retrieves a journal entry by ID (without ledger entries)
func (r *JournalEntryRepository) GetByID(ctx context.Context, id string) (*domain.JournalEntry, error) {
	query := `
		SELECT id, description, reference_type, reference_id, posted_at, created_at
		FROM journal_entries
		WHERE id = $1
	`

	entry := &domain.JournalEntry{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&entry.ID, &entry.Description, &entry.ReferenceType, &entry.ReferenceID,
		&entry.PostedAt, &entry.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying journal entry: %w", err)
	}

	return entry, nil
}

// GetByReference retrieves all journal entries for a reference
func (r *JournalEntryRepository) GetByReference(ctx context.Context, refType domain.ReferenceType, refID string) ([]*domain.JournalEntry, error) {
	query := `
		SELECT id, description, reference_type, reference_id, posted_at, created_at
		FROM journal_entries
		WHERE reference_type = $1 AND reference_id = $2
		ORDER BY posted_at DESC
	`

	rows, err := r.pool.Query(ctx, query, refType, refID)
	if err != nil {
		return nil, fmt.Errorf("querying journal entries by reference: %w", err)
	}
	defer rows.Close()

	var entries []*domain.JournalEntry
	for rows.Next() {
		entry := &domain.JournalEntry{}
		err := rows.Scan(
			&entry.ID, &entry.Description, &entry.ReferenceType, &entry.ReferenceID,
			&entry.PostedAt, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning journal entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// GetByIDWithEntries retrieves a journal entry with its ledger entries
func (r *JournalEntryRepository) GetByIDWithEntries(ctx context.Context, id string) (*domain.JournalEntry, error) {
	// First get the journal entry
	entry, err := r.GetByID(ctx, id)
	if err != nil || entry == nil {
		return entry, err
	}

	// Then get the ledger entries
	ledgerQuery := `
		SELECT id, journal_entry_id, account_id, amount, direction, balance_after, created_at
		FROM ledger_entries
		WHERE journal_entry_id = $1
		ORDER BY created_at
	`

	rows, err := r.pool.Query(ctx, ledgerQuery, id)
	if err != nil {
		return nil, fmt.Errorf("querying ledger entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		le := domain.LedgerEntry{}
		var amountStr, balanceAfterStr string
		err := rows.Scan(
			&le.ID, &le.JournalEntryID, &le.AccountID, &amountStr, &le.Direction,
			&balanceAfterStr, &le.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning ledger entry: %w", err)
		}

		le.Amount, err = parseDecimal(amountStr)
		if err != nil {
			return nil, fmt.Errorf("parsing amount: %w", err)
		}
		le.BalanceAfter, err = parseDecimal(balanceAfterStr)
		if err != nil {
			return nil, fmt.Errorf("parsing balance_after: %w", err)
		}

		entry.Entries = append(entry.Entries, le)
	}

	return entry, rows.Err()
}

// GetByTimeRange retrieves journal entries within a time range
func (r *JournalEntryRepository) GetByTimeRange(ctx context.Context, start, end time.Time, limit int) ([]*domain.JournalEntry, error) {
	query := `
		SELECT id, description, reference_type, reference_id, posted_at, created_at
		FROM journal_entries
		WHERE posted_at >= $1 AND posted_at < $2
		ORDER BY posted_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("querying journal entries by time range: %w", err)
	}
	defer rows.Close()

	var entries []*domain.JournalEntry
	for rows.Next() {
		entry := &domain.JournalEntry{}
		err := rows.Scan(
			&entry.ID, &entry.Description, &entry.ReferenceType, &entry.ReferenceID,
			&entry.PostedAt, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning journal entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
