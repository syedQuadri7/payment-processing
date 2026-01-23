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

// PaymentAttemptRepository implements domain.PaymentAttemptRepository
type PaymentAttemptRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentAttemptRepository creates a new PaymentAttemptRepository
func NewPaymentAttemptRepository(pool *pgxpool.Pool) *PaymentAttemptRepository {
	return &PaymentAttemptRepository{pool: pool}
}

// Create inserts a new payment attempt
func (r *PaymentAttemptRepository) Create(ctx context.Context, pa *domain.PaymentAttempt) error {
	query := `
		INSERT INTO payment_attempts (
			id, payment_intent_id, attempt_number, status, provider,
			provider_response_code, canonical_decline_code, decline_type,
			processor_txn_id, idempotency_key, created_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	if pa.CreatedAt.IsZero() {
		pa.CreatedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, query,
		pa.ID, pa.PaymentIntentID, pa.AttemptNumber, pa.Status, pa.Provider,
		pa.ProviderResponseCode, pa.CanonicalDeclineCode, pa.DeclineType,
		pa.ProcessorTxnID, pa.IdempotencyKey, pa.CreatedAt, pa.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting payment attempt: %w", err)
	}

	return nil
}

// GetByID retrieves a payment attempt by ID
func (r *PaymentAttemptRepository) GetByID(ctx context.Context, id string) (*domain.PaymentAttempt, error) {
	query := `
		SELECT id, payment_intent_id, attempt_number, status, provider,
			provider_response_code, canonical_decline_code, decline_type,
			processor_txn_id, idempotency_key, created_at, completed_at
		FROM payment_attempts
		WHERE id = $1
	`

	return r.scanRow(r.pool.QueryRow(ctx, query, id))
}

// GetByPaymentIntentID retrieves all attempts for a payment intent
func (r *PaymentAttemptRepository) GetByPaymentIntentID(ctx context.Context, intentID string) ([]*domain.PaymentAttempt, error) {
	query := `
		SELECT id, payment_intent_id, attempt_number, status, provider,
			provider_response_code, canonical_decline_code, decline_type,
			processor_txn_id, idempotency_key, created_at, completed_at
		FROM payment_attempts
		WHERE payment_intent_id = $1
		ORDER BY attempt_number ASC
	`

	rows, err := r.pool.Query(ctx, query, intentID)
	if err != nil {
		return nil, fmt.Errorf("querying payment attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*domain.PaymentAttempt
	for rows.Next() {
		pa := &domain.PaymentAttempt{}
		var declineType *string
		err := rows.Scan(
			&pa.ID, &pa.PaymentIntentID, &pa.AttemptNumber, &pa.Status, &pa.Provider,
			&pa.ProviderResponseCode, &pa.CanonicalDeclineCode, &declineType,
			&pa.ProcessorTxnID, &pa.IdempotencyKey, &pa.CreatedAt, &pa.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning payment attempt: %w", err)
		}
		if declineType != nil {
			dt := domain.DeclineType(*declineType)
			pa.DeclineType = &dt
		}
		attempts = append(attempts, pa)
	}

	return attempts, rows.Err()
}

// GetLatestByPaymentIntentID retrieves the most recent attempt for a payment intent
func (r *PaymentAttemptRepository) GetLatestByPaymentIntentID(ctx context.Context, intentID string) (*domain.PaymentAttempt, error) {
	query := `
		SELECT id, payment_intent_id, attempt_number, status, provider,
			provider_response_code, canonical_decline_code, decline_type,
			processor_txn_id, idempotency_key, created_at, completed_at
		FROM payment_attempts
		WHERE payment_intent_id = $1
		ORDER BY attempt_number DESC
		LIMIT 1
	`

	return r.scanRow(r.pool.QueryRow(ctx, query, intentID))
}

// CountByPaymentIntentID returns the number of attempts for a payment intent
func (r *PaymentAttemptRepository) CountByPaymentIntentID(ctx context.Context, intentID string) (int, error) {
	query := `SELECT COUNT(*) FROM payment_attempts WHERE payment_intent_id = $1`

	var count int
	err := r.pool.QueryRow(ctx, query, intentID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting payment attempts: %w", err)
	}

	return count, nil
}

func (r *PaymentAttemptRepository) scanRow(row pgx.Row) (*domain.PaymentAttempt, error) {
	pa := &domain.PaymentAttempt{}
	var declineType *string

	err := row.Scan(
		&pa.ID, &pa.PaymentIntentID, &pa.AttemptNumber, &pa.Status, &pa.Provider,
		&pa.ProviderResponseCode, &pa.CanonicalDeclineCode, &declineType,
		&pa.ProcessorTxnID, &pa.IdempotencyKey, &pa.CreatedAt, &pa.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying payment attempt: %w", err)
	}

	if declineType != nil {
		dt := domain.DeclineType(*declineType)
		pa.DeclineType = &dt
	}

	return pa, nil
}

// MarkCompleted marks an attempt as completed with result details
func (r *PaymentAttemptRepository) MarkCompleted(
	ctx context.Context,
	id string,
	status domain.AttemptStatus,
	providerCode, declineCode *string,
	declineType *domain.DeclineType,
) error {
	query := `
		UPDATE payment_attempts
		SET status = $2, provider_response_code = $3, canonical_decline_code = $4,
			decline_type = $5, completed_at = $6
		WHERE id = $1
	`

	now := time.Now()
	var dt *string
	if declineType != nil {
		s := string(*declineType)
		dt = &s
	}

	result, err := r.pool.Exec(ctx, query, id, status, providerCode, declineCode, dt, now)
	if err != nil {
		return fmt.Errorf("marking attempt completed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("payment attempt not found: %s", id)
	}

	return nil
}
