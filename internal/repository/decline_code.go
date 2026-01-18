package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payment-processing/internal/domain"
)

// DeclineCodeRepository implements domain.DeclineCodeRepository
type DeclineCodeRepository struct {
	pool *pgxpool.Pool
}

// NewDeclineCodeRepository creates a new DeclineCodeRepository
func NewDeclineCodeRepository(pool *pgxpool.Pool) *DeclineCodeRepository {
	return &DeclineCodeRepository{pool: pool}
}

// GetByProviderCode retrieves a decline code mapping by provider and code
func (r *DeclineCodeRepository) GetByProviderCode(ctx context.Context, provider domain.Provider, code string) (*domain.DeclineCodeMapping, error) {
	query := `
		SELECT id, provider, provider_code, canonical_code, decline_type,
			description, retry_eligible, suggested_action, created_at, updated_at
		FROM decline_code_mappings
		WHERE provider = $1 AND provider_code = $2
	`

	dcm := &domain.DeclineCodeMapping{}
	var declineType string

	err := r.pool.QueryRow(ctx, query, provider, code).Scan(
		&dcm.ID, &dcm.Provider, &dcm.ProviderCode, &dcm.CanonicalCode, &declineType,
		&dcm.Description, &dcm.RetryEligible, &dcm.SuggestedAction, &dcm.CreatedAt, &dcm.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying decline code mapping: %w", err)
	}

	dcm.DeclineType = domain.DeclineType(declineType)

	return dcm, nil
}

// GetAll retrieves all decline code mappings
func (r *DeclineCodeRepository) GetAll(ctx context.Context) ([]*domain.DeclineCodeMapping, error) {
	query := `
		SELECT id, provider, provider_code, canonical_code, decline_type,
			description, retry_eligible, suggested_action, created_at, updated_at
		FROM decline_code_mappings
		ORDER BY provider, provider_code
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying decline code mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*domain.DeclineCodeMapping
	for rows.Next() {
		dcm := &domain.DeclineCodeMapping{}
		var declineType string

		err := rows.Scan(
			&dcm.ID, &dcm.Provider, &dcm.ProviderCode, &dcm.CanonicalCode, &declineType,
			&dcm.Description, &dcm.RetryEligible, &dcm.SuggestedAction, &dcm.CreatedAt, &dcm.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning decline code mapping: %w", err)
		}
		dcm.DeclineType = domain.DeclineType(declineType)
		mappings = append(mappings, dcm)
	}

	return mappings, rows.Err()
}

// GetByProvider retrieves all decline code mappings for a specific provider
func (r *DeclineCodeRepository) GetByProvider(ctx context.Context, provider domain.Provider) ([]*domain.DeclineCodeMapping, error) {
	query := `
		SELECT id, provider, provider_code, canonical_code, decline_type,
			description, retry_eligible, suggested_action, created_at, updated_at
		FROM decline_code_mappings
		WHERE provider = $1
		ORDER BY provider_code
	`

	rows, err := r.pool.Query(ctx, query, provider)
	if err != nil {
		return nil, fmt.Errorf("querying decline code mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*domain.DeclineCodeMapping
	for rows.Next() {
		dcm := &domain.DeclineCodeMapping{}
		var declineType string

		err := rows.Scan(
			&dcm.ID, &dcm.Provider, &dcm.ProviderCode, &dcm.CanonicalCode, &declineType,
			&dcm.Description, &dcm.RetryEligible, &dcm.SuggestedAction, &dcm.CreatedAt, &dcm.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning decline code mapping: %w", err)
		}
		dcm.DeclineType = domain.DeclineType(declineType)
		mappings = append(mappings, dcm)
	}

	return mappings, rows.Err()
}
