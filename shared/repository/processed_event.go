package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payment-processing/shared/domain"
)

// ProcessedEventRepository implements domain.ProcessedEventRepository
type ProcessedEventRepository struct {
	pool *pgxpool.Pool
}

// NewProcessedEventRepository creates a new ProcessedEventRepository
func NewProcessedEventRepository(pool *pgxpool.Pool) *ProcessedEventRepository {
	return &ProcessedEventRepository{pool: pool}
}

// Create inserts a new processed event record
// Used to track webhook events that have been processed for idempotency
func (r *ProcessedEventRepository) Create(ctx context.Context, event *domain.ProcessedEvent) error {
	query := `
		INSERT INTO processed_events (
			id, provider, event_id, event_type, processed_at
		) VALUES ($1, $2, $3, $4, $5)
	`

	if event.ProcessedAt.IsZero() {
		event.ProcessedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, query,
		event.ID, event.Provider, event.EventID, event.EventType, event.ProcessedAt,
	)
	if err != nil {
		// Check for unique constraint violation
		if isDuplicateKeyError(err) {
			return ErrDuplicateEvent
		}
		return fmt.Errorf("inserting processed event: %w", err)
	}

	return nil
}

// Exists checks if an event has already been processed
func (r *ProcessedEventRepository) Exists(ctx context.Context, provider domain.Provider, eventID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM processed_events
			WHERE provider = $1 AND event_id = $2
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, provider, eventID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking processed event existence: %w", err)
	}

	return exists, nil
}

// DeleteOlderThan deletes processed events older than the specified time
// Used for cleanup of old processed event records
func (r *ProcessedEventRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM processed_events WHERE processed_at < $1`

	result, err := r.pool.Exec(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("deleting old processed events: %w", err)
	}

	return result.RowsAffected(), nil
}

// GetByProvider retrieves all processed events for a provider (for debugging/monitoring)
func (r *ProcessedEventRepository) GetByProvider(ctx context.Context, provider domain.Provider, limit int) ([]*domain.ProcessedEvent, error) {
	query := `
		SELECT id, provider, event_id, event_type, processed_at
		FROM processed_events
		WHERE provider = $1
		ORDER BY processed_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, provider, limit)
	if err != nil {
		return nil, fmt.Errorf("querying processed events: %w", err)
	}
	defer rows.Close()

	var events []*domain.ProcessedEvent
	for rows.Next() {
		event := &domain.ProcessedEvent{}
		err := rows.Scan(
			&event.ID, &event.Provider, &event.EventID, &event.EventType, &event.ProcessedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning processed event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// GetByEventID retrieves a processed event by provider and event ID
func (r *ProcessedEventRepository) GetByEventID(ctx context.Context, provider domain.Provider, eventID string) (*domain.ProcessedEvent, error) {
	query := `
		SELECT id, provider, event_id, event_type, processed_at
		FROM processed_events
		WHERE provider = $1 AND event_id = $2
	`

	event := &domain.ProcessedEvent{}
	err := r.pool.QueryRow(ctx, query, provider, eventID).Scan(
		&event.ID, &event.Provider, &event.EventID, &event.EventType, &event.ProcessedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying processed event: %w", err)
	}

	return event, nil
}

// Count returns the total number of processed events
func (r *ProcessedEventRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM processed_events`

	var count int64
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting processed events: %w", err)
	}

	return count, nil
}

// CountByProvider returns the number of processed events for a provider
func (r *ProcessedEventRepository) CountByProvider(ctx context.Context, provider domain.Provider) (int64, error) {
	query := `SELECT COUNT(*) FROM processed_events WHERE provider = $1`

	var count int64
	err := r.pool.QueryRow(ctx, query, provider).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting processed events by provider: %w", err)
	}

	return count, nil
}

// ErrDuplicateEvent is returned when trying to insert a duplicate event
var ErrDuplicateEvent = errors.New("event has already been processed")

// isDuplicateKeyError checks if an error is a duplicate key violation
func isDuplicateKeyError(err error) bool {
	// pgx wraps PostgreSQL errors; check for unique_violation (23505)
	return err != nil && (contains(err.Error(), "duplicate key") ||
		contains(err.Error(), "23505"))
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
