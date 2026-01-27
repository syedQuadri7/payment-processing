package repository

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"payment-processing/shared/domain"
)

// AuditLogRepository implements domain.AuditLogRepository
type AuditLogRepository struct {
	pool *pgxpool.Pool
}

// NewAuditLogRepository creates a new AuditLogRepository
func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{pool: pool}
}

// Create inserts a new audit log entry (append-only)
// Note: The database has rules preventing updates and deletes
func (r *AuditLogRepository) Create(ctx context.Context, entry *domain.AuditLogEntry) error {
	query := `
		INSERT INTO audit_log (
			id, entity_type, entity_id, action, actor_type, actor_id,
			old_values, new_values, metadata, ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	// Convert IP to string for database
	var ipAddress *string
	if entry.IPAddress != nil {
		ip := entry.IPAddress.String()
		ipAddress = &ip
	}

	_, err := r.pool.Exec(ctx, query,
		entry.ID, entry.EntityType, entry.EntityID, entry.Action,
		entry.ActorType, entry.ActorID, entry.OldValues, entry.NewValues,
		entry.Metadata, ipAddress, entry.UserAgent, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting audit log entry: %w", err)
	}

	return nil
}

// GetByEntity retrieves all audit entries for an entity
func (r *AuditLogRepository) GetByEntity(ctx context.Context, entityType domain.AuditEntityType, entityID string) ([]*domain.AuditLogEntry, error) {
	query := `
		SELECT id, entity_type, entity_id, action, actor_type, actor_id,
			old_values, new_values, metadata, ip_address, user_agent, created_at
		FROM audit_log
		WHERE entity_type = $1 AND entity_id = $2
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("querying audit log by entity: %w", err)
	}
	defer rows.Close()

	return r.scanAuditEntries(rows)
}

// GetByActor retrieves audit entries by actor
func (r *AuditLogRepository) GetByActor(ctx context.Context, actorType domain.AuditActorType, actorID string, limit int) ([]*domain.AuditLogEntry, error) {
	query := `
		SELECT id, entity_type, entity_id, action, actor_type, actor_id,
			old_values, new_values, metadata, ip_address, user_agent, created_at
		FROM audit_log
		WHERE actor_type = $1 AND actor_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, actorType, actorID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying audit log by actor: %w", err)
	}
	defer rows.Close()

	return r.scanAuditEntries(rows)
}

// GetByTimeRange retrieves audit entries within a time range
func (r *AuditLogRepository) GetByTimeRange(ctx context.Context, start, end time.Time, limit int) ([]*domain.AuditLogEntry, error) {
	query := `
		SELECT id, entity_type, entity_id, action, actor_type, actor_id,
			old_values, new_values, metadata, ip_address, user_agent, created_at
		FROM audit_log
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("querying audit log by time range: %w", err)
	}
	defer rows.Close()

	return r.scanAuditEntries(rows)
}

// GetByAction retrieves audit entries by action type
func (r *AuditLogRepository) GetByAction(ctx context.Context, action domain.AuditAction, limit int) ([]*domain.AuditLogEntry, error) {
	query := `
		SELECT id, entity_type, entity_id, action, actor_type, actor_id,
			old_values, new_values, metadata, ip_address, user_agent, created_at
		FROM audit_log
		WHERE action = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, action, limit)
	if err != nil {
		return nil, fmt.Errorf("querying audit log by action: %w", err)
	}
	defer rows.Close()

	return r.scanAuditEntries(rows)
}

// scanAuditEntries scans multiple audit entries from rows
func (r *AuditLogRepository) scanAuditEntries(rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]*domain.AuditLogEntry, error) {
	var entries []*domain.AuditLogEntry
	for rows.Next() {
		entry := &domain.AuditLogEntry{}
		var ipAddress *string

		err := rows.Scan(
			&entry.ID, &entry.EntityType, &entry.EntityID, &entry.Action,
			&entry.ActorType, &entry.ActorID, &entry.OldValues, &entry.NewValues,
			&entry.Metadata, &ipAddress, &entry.UserAgent, &entry.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning audit log entry: %w", err)
		}

		if ipAddress != nil {
			entry.IPAddress = net.ParseIP(*ipAddress)
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// Count returns the total number of audit log entries
func (r *AuditLogRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM audit_log`

	var count int64
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting audit log entries: %w", err)
	}

	return count, nil
}
