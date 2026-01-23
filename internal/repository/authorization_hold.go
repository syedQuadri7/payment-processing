package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"payment-processing/pkg/domain"
)

// AuthorizationHoldRepository implements domain.AuthorizationHoldRepository
type AuthorizationHoldRepository struct {
	pool *pgxpool.Pool
}

// NewAuthorizationHoldRepository creates a new AuthorizationHoldRepository
func NewAuthorizationHoldRepository(pool *pgxpool.Pool) *AuthorizationHoldRepository {
	return &AuthorizationHoldRepository{pool: pool}
}

// Create inserts a new authorization hold
func (r *AuthorizationHoldRepository) Create(ctx context.Context, ah *domain.AuthorizationHold) error {
	query := `
		INSERT INTO authorization_holds (
			id, payment_intent_id, amount, currency, status, provider,
			auth_code, network_txn_id, expires_at, captured_amount, captured_at,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	now := time.Now()
	if ah.CreatedAt.IsZero() {
		ah.CreatedAt = now
	}
	ah.UpdatedAt = now

	if ah.CapturedAmount.IsZero() {
		ah.CapturedAmount = decimal.Zero
	}

	_, err := r.pool.Exec(ctx, query,
		ah.ID, ah.PaymentIntentID, ah.Amount, ah.Currency, ah.Status, ah.Provider,
		ah.AuthCode, ah.NetworkTxnID, ah.ExpiresAt, ah.CapturedAmount, ah.CapturedAt,
		ah.CreatedAt, ah.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting authorization hold: %w", err)
	}

	return nil
}

// GetByID retrieves an authorization hold by ID
func (r *AuthorizationHoldRepository) GetByID(ctx context.Context, id string) (*domain.AuthorizationHold, error) {
	query := `
		SELECT id, payment_intent_id, amount, currency, status, provider,
			auth_code, network_txn_id, expires_at, captured_amount, captured_at,
			created_at, updated_at
		FROM authorization_holds
		WHERE id = $1
	`

	return r.scanRow(r.pool.QueryRow(ctx, query, id))
}

// GetByPaymentIntentID retrieves an authorization hold by payment intent ID
func (r *AuthorizationHoldRepository) GetByPaymentIntentID(ctx context.Context, intentID string) (*domain.AuthorizationHold, error) {
	query := `
		SELECT id, payment_intent_id, amount, currency, status, provider,
			auth_code, network_txn_id, expires_at, captured_amount, captured_at,
			created_at, updated_at
		FROM authorization_holds
		WHERE payment_intent_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	return r.scanRow(r.pool.QueryRow(ctx, query, intentID))
}

// GetActiveByPaymentIntentID retrieves the active authorization hold for a payment intent
func (r *AuthorizationHoldRepository) GetActiveByPaymentIntentID(ctx context.Context, intentID string) (*domain.AuthorizationHold, error) {
	query := `
		SELECT id, payment_intent_id, amount, currency, status, provider,
			auth_code, network_txn_id, expires_at, captured_amount, captured_at,
			created_at, updated_at
		FROM authorization_holds
		WHERE payment_intent_id = $1 AND status = $2 AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`

	return r.scanRow(r.pool.QueryRow(ctx, query, intentID, domain.HoldStatusActive))
}

func (r *AuthorizationHoldRepository) scanRow(row pgx.Row) (*domain.AuthorizationHold, error) {
	ah := &domain.AuthorizationHold{}
	var amountStr, capturedAmountStr string

	err := row.Scan(
		&ah.ID, &ah.PaymentIntentID, &amountStr, &ah.Currency, &ah.Status, &ah.Provider,
		&ah.AuthCode, &ah.NetworkTxnID, &ah.ExpiresAt, &capturedAmountStr, &ah.CapturedAt,
		&ah.CreatedAt, &ah.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying authorization hold: %w", err)
	}

	var parseErr error
	ah.Amount, parseErr = decimal.NewFromString(amountStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing amount: %w", parseErr)
	}
	ah.CapturedAmount, parseErr = decimal.NewFromString(capturedAmountStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing captured amount: %w", parseErr)
	}

	return ah, nil
}

// UpdateStatus updates the status of an authorization hold
func (r *AuthorizationHoldRepository) UpdateStatus(ctx context.Context, id string, status domain.HoldStatus) error {
	query := `
		UPDATE authorization_holds
		SET status = $2, updated_at = $3
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, status, time.Now())
	if err != nil {
		return fmt.Errorf("updating authorization hold status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("authorization hold not found: %s", id)
	}

	return nil
}

// UpdateCaptured updates the captured amount and timestamp
func (r *AuthorizationHoldRepository) UpdateCaptured(ctx context.Context, id string, amount decimal.Decimal, capturedAt *time.Time) error {
	query := `
		UPDATE authorization_holds
		SET captured_amount = captured_amount + $2, captured_at = $3, updated_at = $4
		WHERE id = $1
	`

	now := time.Now()
	if capturedAt == nil {
		capturedAt = &now
	}

	result, err := r.pool.Exec(ctx, query, id, amount, capturedAt, now)
	if err != nil {
		return fmt.Errorf("updating captured amount: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("authorization hold not found: %s", id)
	}

	return nil
}
