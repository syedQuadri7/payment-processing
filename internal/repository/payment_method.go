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

// PaymentMethodRepository implements domain.PaymentMethodRepository
type PaymentMethodRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentMethodRepository creates a new PaymentMethodRepository
func NewPaymentMethodRepository(pool *pgxpool.Pool) *PaymentMethodRepository {
	return &PaymentMethodRepository{pool: pool}
}

// Create inserts a new payment method
func (r *PaymentMethodRepository) Create(ctx context.Context, pm *domain.PaymentMethod) error {
	query := `
		INSERT INTO payment_methods (
			id, customer_id, type, provider, token, last_four,
			expiry_month, expiry_year, is_default, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	now := time.Now()
	if pm.CreatedAt.IsZero() {
		pm.CreatedAt = now
	}
	pm.UpdatedAt = now

	_, err := r.pool.Exec(ctx, query,
		pm.ID, pm.CustomerID, pm.Type, pm.Provider, pm.Token, pm.LastFour,
		pm.ExpiryMonth, pm.ExpiryYear, pm.IsDefault, pm.Status, pm.CreatedAt, pm.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting payment method: %w", err)
	}

	return nil
}

// GetByID retrieves a payment method by ID
func (r *PaymentMethodRepository) GetByID(ctx context.Context, id string) (*domain.PaymentMethod, error) {
	query := `
		SELECT id, customer_id, type, provider, token, last_four,
			expiry_month, expiry_year, is_default, status, created_at, updated_at
		FROM payment_methods
		WHERE id = $1
	`

	pm := &domain.PaymentMethod{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&pm.ID, &pm.CustomerID, &pm.Type, &pm.Provider, &pm.Token, &pm.LastFour,
		&pm.ExpiryMonth, &pm.ExpiryYear, &pm.IsDefault, &pm.Status, &pm.CreatedAt, &pm.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying payment method: %w", err)
	}

	return pm, nil
}

// GetByCustomerID retrieves all payment methods for a customer
func (r *PaymentMethodRepository) GetByCustomerID(ctx context.Context, customerID string) ([]*domain.PaymentMethod, error) {
	query := `
		SELECT id, customer_id, type, provider, token, last_four,
			expiry_month, expiry_year, is_default, status, created_at, updated_at
		FROM payment_methods
		WHERE customer_id = $1 AND status = $2
		ORDER BY is_default DESC, created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, customerID, domain.PaymentMethodStatusActive)
	if err != nil {
		return nil, fmt.Errorf("querying payment methods: %w", err)
	}
	defer rows.Close()

	var methods []*domain.PaymentMethod
	for rows.Next() {
		pm := &domain.PaymentMethod{}
		err := rows.Scan(
			&pm.ID, &pm.CustomerID, &pm.Type, &pm.Provider, &pm.Token, &pm.LastFour,
			&pm.ExpiryMonth, &pm.ExpiryYear, &pm.IsDefault, &pm.Status, &pm.CreatedAt, &pm.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning payment method: %w", err)
		}
		methods = append(methods, pm)
	}

	return methods, rows.Err()
}

// GetDefaultForCustomer retrieves the default payment method for a customer
func (r *PaymentMethodRepository) GetDefaultForCustomer(ctx context.Context, customerID string) (*domain.PaymentMethod, error) {
	query := `
		SELECT id, customer_id, type, provider, token, last_four,
			expiry_month, expiry_year, is_default, status, created_at, updated_at
		FROM payment_methods
		WHERE customer_id = $1 AND is_default = true AND status = $2
	`

	pm := &domain.PaymentMethod{}
	err := r.pool.QueryRow(ctx, query, customerID, domain.PaymentMethodStatusActive).Scan(
		&pm.ID, &pm.CustomerID, &pm.Type, &pm.Provider, &pm.Token, &pm.LastFour,
		&pm.ExpiryMonth, &pm.ExpiryYear, &pm.IsDefault, &pm.Status, &pm.CreatedAt, &pm.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying default payment method: %w", err)
	}

	return pm, nil
}

// Update updates a payment method
func (r *PaymentMethodRepository) Update(ctx context.Context, pm *domain.PaymentMethod) error {
	query := `
		UPDATE payment_methods
		SET customer_id = $2, type = $3, provider = $4, token = $5, last_four = $6,
			expiry_month = $7, expiry_year = $8, is_default = $9, status = $10, updated_at = $11
		WHERE id = $1
	`

	pm.UpdatedAt = time.Now()

	result, err := r.pool.Exec(ctx, query,
		pm.ID, pm.CustomerID, pm.Type, pm.Provider, pm.Token, pm.LastFour,
		pm.ExpiryMonth, pm.ExpiryYear, pm.IsDefault, pm.Status, pm.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("updating payment method: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("payment method not found: %s", pm.ID)
	}

	return nil
}

// Delete soft-deletes a payment method
func (r *PaymentMethodRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE payment_methods
		SET status = $2, updated_at = $3
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id, domain.PaymentMethodStatusDeleted, time.Now())
	if err != nil {
		return fmt.Errorf("deleting payment method: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("payment method not found: %s", id)
	}

	return nil
}
