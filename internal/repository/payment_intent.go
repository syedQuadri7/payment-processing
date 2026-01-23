package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"payment-processing/pkg/domain"
)

// PaymentIntentRepository implements domain.PaymentIntentRepository
type PaymentIntentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentIntentRepository creates a new PaymentIntentRepository
func NewPaymentIntentRepository(pool *pgxpool.Pool) *PaymentIntentRepository {
	return &PaymentIntentRepository{pool: pool}
}

// Create inserts a new payment intent
func (r *PaymentIntentRepository) Create(ctx context.Context, pi *domain.PaymentIntent) error {
	query := `
		INSERT INTO payment_intents (
			id, idempotency_key, customer_id, amount, currency, status,
			capture_method, provider, provider_payment_id, payment_method_id,
			workflow_id, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	now := time.Now()
	if pi.CreatedAt.IsZero() {
		pi.CreatedAt = now
	}
	pi.UpdatedAt = now

	var metadataJSON []byte
	var err error
	if pi.Metadata != nil {
		metadataJSON, err = json.Marshal(pi.Metadata)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
	}

	_, err = r.pool.Exec(ctx, query,
		pi.ID, pi.IdempotencyKey, pi.CustomerID, pi.Amount, pi.Currency, pi.Status,
		pi.CaptureMethod, pi.Provider, pi.ProviderPaymentID, pi.PaymentMethodID,
		pi.WorkflowID, metadataJSON, pi.CreatedAt, pi.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting payment intent: %w", err)
	}

	return nil
}

// GetByID retrieves a payment intent by ID
func (r *PaymentIntentRepository) GetByID(ctx context.Context, id string) (*domain.PaymentIntent, error) {
	return r.getByColumn(ctx, "id", id)
}

// GetByIdempotencyKey retrieves a payment intent by idempotency key
func (r *PaymentIntentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentIntent, error) {
	return r.getByColumn(ctx, "idempotency_key", key)
}

// GetByWorkflowID retrieves a payment intent by workflow ID
func (r *PaymentIntentRepository) GetByWorkflowID(ctx context.Context, workflowID string) (*domain.PaymentIntent, error) {
	return r.getByColumn(ctx, "workflow_id", workflowID)
}

// validPaymentIntentColumns defines the allowlist of columns that can be queried
var validPaymentIntentColumns = map[string]bool{
	"id":              true,
	"idempotency_key": true,
	"workflow_id":     true,
}

func (r *PaymentIntentRepository) getByColumn(ctx context.Context, column, value string) (*domain.PaymentIntent, error) {
	if !validPaymentIntentColumns[column] {
		return nil, fmt.Errorf("invalid column for payment intent query: %s", column)
	}

	query := fmt.Sprintf(`
		SELECT id, idempotency_key, customer_id, amount, currency, status,
			capture_method, provider, provider_payment_id, payment_method_id,
			workflow_id, metadata, created_at, updated_at
		FROM payment_intents
		WHERE %s = $1
	`, column)

	pi := &domain.PaymentIntent{}
	var amountStr string
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, value).Scan(
		&pi.ID, &pi.IdempotencyKey, &pi.CustomerID, &amountStr, &pi.Currency, &pi.Status,
		&pi.CaptureMethod, &pi.Provider, &pi.ProviderPaymentID, &pi.PaymentMethodID,
		&pi.WorkflowID, &metadataJSON, &pi.CreatedAt, &pi.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying payment intent: %w", err)
	}

	var parseErr error
	pi.Amount, parseErr = decimal.NewFromString(amountStr)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing amount: %w", parseErr)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &pi.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	return pi, nil
}

// Update updates a payment intent
func (r *PaymentIntentRepository) Update(ctx context.Context, pi *domain.PaymentIntent) error {
	query := `
		UPDATE payment_intents
		SET idempotency_key = $2, customer_id = $3, amount = $4, currency = $5, status = $6,
			capture_method = $7, provider = $8, provider_payment_id = $9, payment_method_id = $10,
			workflow_id = $11, metadata = $12, updated_at = $13
		WHERE id = $1
	`

	pi.UpdatedAt = time.Now()

	var metadataJSON []byte
	var err error
	if pi.Metadata != nil {
		metadataJSON, err = json.Marshal(pi.Metadata)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
	}

	result, err := r.pool.Exec(ctx, query,
		pi.ID, pi.IdempotencyKey, pi.CustomerID, pi.Amount, pi.Currency, pi.Status,
		pi.CaptureMethod, pi.Provider, pi.ProviderPaymentID, pi.PaymentMethodID,
		pi.WorkflowID, metadataJSON, pi.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("updating payment intent: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("payment intent not found: %s", pi.ID)
	}

	return nil
}

// UpdateStatus updates only the status of a payment intent
func (r *PaymentIntentRepository) UpdateStatus(ctx context.Context, id string, status domain.PaymentIntentStatus) error {
	query := `
		UPDATE payment_intents
		SET status = $2, updated_at = $3
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, status, time.Now())
	if err != nil {
		return fmt.Errorf("updating payment intent status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("payment intent not found: %s", id)
	}

	return nil
}
