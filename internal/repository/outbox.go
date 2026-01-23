package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"payment-processing/pkg/domain"
)

// OutboxRepository implements domain.OutboxRepository
type OutboxRepository struct {
	pool *pgxpool.Pool
}

// NewOutboxRepository creates a new OutboxRepository
func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

// Create inserts a new outbox event
func (r *OutboxRepository) Create(ctx context.Context, event *domain.OutboxEvent) error {
	query := `
		INSERT INTO outbox (
			id, aggregate_type, aggregate_id, event_type, payload, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	_, err := r.pool.Exec(ctx, query,
		event.ID, event.AggregateType, event.AggregateID,
		event.EventType, event.Payload, event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting outbox event: %w", err)
	}

	return nil
}

// GetUnpublished retrieves unpublished events for polling-based consumers
// Note: In production, CDC (Debezium) reads directly from the table
// This method is for fallback/simple polling scenarios
func (r *OutboxRepository) GetUnpublished(ctx context.Context, limit int) ([]*domain.OutboxEvent, error) {
	// Since we're using CDC, there's no "published" flag
	// This method returns recent events for polling-based consumption
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		FROM outbox
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying outbox events: %w", err)
	}
	defer rows.Close()

	var events []*domain.OutboxEvent
	for rows.Next() {
		event := &domain.OutboxEvent{}
		err := rows.Scan(
			&event.ID, &event.AggregateType, &event.AggregateID,
			&event.EventType, &event.Payload, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning outbox event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// MarkPublished marks events as published (deletes them for CDC pattern)
// In the CDC pattern, Debezium reads the entire row and we delete after
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := `DELETE FROM outbox WHERE id = ANY($1)`

	_, err := r.pool.Exec(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("deleting outbox events: %w", err)
	}

	return nil
}

// DeleteOlderThan deletes events older than the specified time
// Used for cleanup after CDC has processed them
func (r *OutboxRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM outbox WHERE created_at < $1`

	result, err := r.pool.Exec(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("deleting old outbox events: %w", err)
	}

	return result.RowsAffected(), nil
}

// GetByAggregateID retrieves events for a specific aggregate
func (r *OutboxRepository) GetByAggregateID(ctx context.Context, aggregateType domain.AggregateType, aggregateID string) ([]*domain.OutboxEvent, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at
		FROM outbox
		WHERE aggregate_type = $1 AND aggregate_id = $2
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, aggregateType, aggregateID)
	if err != nil {
		return nil, fmt.Errorf("querying outbox events by aggregate: %w", err)
	}
	defer rows.Close()

	var events []*domain.OutboxEvent
	for rows.Next() {
		event := &domain.OutboxEvent{}
		err := rows.Scan(
			&event.ID, &event.AggregateType, &event.AggregateID,
			&event.EventType, &event.Payload, &event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning outbox event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// Count returns the total number of events in the outbox
func (r *OutboxRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM outbox`

	var count int64
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting outbox events: %w", err)
	}

	return count, nil
}
