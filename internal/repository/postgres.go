package repository

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"payment-processing/internal/database"
	"payment-processing/pkg/domain"
)

// Config holds database connection configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// DefaultConfig returns the default database configuration
func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     5432,
		User:     "payment",
		Password: "payment_secret",
		Database: "payment_processing",
		SSLMode:  "disable",
	}
}

// ConnectionString returns a PostgreSQL connection string
func (c Config) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode,
	)
}

// DB wraps a PostgreSQL connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB creates a new database connection pool
func NewDB(ctx context.Context, cfg Config) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	db.Pool.Close()
}

// RunMigrations runs all pending database migrations
func (db *DB) RunMigrations(cfg Config) error {
	d, err := iofs.New(database.MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, cfg.ConnectionString())
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

// Repositories returns all repository implementations
func (db *DB) Repositories() *domain.Repositories {
	return &domain.Repositories{
		PaymentIntents:     NewPaymentIntentRepository(db.Pool),
		PaymentMethods:     NewPaymentMethodRepository(db.Pool),
		AuthorizationHolds: NewAuthorizationHoldRepository(db.Pool),
		PaymentAttempts:    NewPaymentAttemptRepository(db.Pool),
		DeclineCodes:       NewDeclineCodeRepository(db.Pool),
		Accounts:           NewAccountRepository(db.Pool),
		JournalEntries:     NewJournalEntryRepository(db.Pool),
		LedgerEntries:      NewLedgerEntryRepository(db.Pool),
		Outbox:             NewOutboxRepository(db.Pool),
		AuditLog:           NewAuditLogRepository(db.Pool),
		ProcessedEvents:    NewProcessedEventRepository(db.Pool),
	}
}
