package database

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrations tests that all migrations can be applied and rolled back cleanly.
// Requires a PostgreSQL database - set DATABASE_URL or use docker-compose.
//
// Run with: go test -tags=integration ./internal/database/...
func TestMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migration tests in short mode")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://payment:payment_secret@localhost:5432/payment_processing_test?sslmode=disable"
	}

	// Create test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("Could not connect to database (is PostgreSQL running?): %v", err)
	}
	defer pool.Close()

	// Ensure clean slate
	_, err = pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	if err != nil {
		t.Fatalf("Failed to reset schema: %v", err)
	}

	// Create migrator
	d, err := iofs.New(MigrationsFS, "migrations")
	if err != nil {
		t.Fatalf("Failed to create migration source: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, dbURL)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	t.Run("migrate up", func(t *testing.T) {
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			t.Fatalf("Failed to migrate up: %v", err)
		}

		// Verify expected tables exist
		expectedTables := []string{
			"payment_methods",
			"payment_intents",
			"authorization_holds",
			"payment_attempts",
			"decline_code_mappings",
			"accounts",
			"journal_entries",
			"ledger_entries",
			"outbox",
			"audit_log",
			"processed_events",
		}

		for _, table := range expectedTables {
			var exists bool
			err := pool.QueryRow(ctx,
				"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)",
				table,
			).Scan(&exists)
			if err != nil {
				t.Errorf("Failed to check table %s: %v", table, err)
			}
			if !exists {
				t.Errorf("Expected table %s to exist", table)
			}
		}
	})

	t.Run("verify seed data", func(t *testing.T) {
		// Check decline code mappings were seeded
		var declineCount int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM decline_code_mappings").Scan(&declineCount)
		if err != nil {
			t.Fatalf("Failed to count decline codes: %v", err)
		}
		if declineCount == 0 {
			t.Error("Expected decline_code_mappings to be seeded")
		}

		// Check clearing accounts were seeded
		var accountCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM accounts WHERE owner_id IS NULL").Scan(&accountCount)
		if err != nil {
			t.Fatalf("Failed to count system accounts: %v", err)
		}
		if accountCount == 0 {
			t.Error("Expected clearing accounts to be seeded")
		}
	})

	t.Run("verify constraints", func(t *testing.T) {
		// Test unique constraint on idempotency_key
		_, err := pool.Exec(ctx, `
			INSERT INTO payment_intents (idempotency_key, customer_id, amount, currency, status, provider)
			VALUES ('test-key-1', 'cust-1', 100.00, 'USD', 'CREATED', 'STRIPE')
		`)
		if err != nil {
			t.Fatalf("Failed to insert first payment intent: %v", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO payment_intents (idempotency_key, customer_id, amount, currency, status, provider)
			VALUES ('test-key-1', 'cust-2', 200.00, 'USD', 'CREATED', 'STRIPE')
		`)
		if err == nil {
			t.Error("Expected unique constraint violation on idempotency_key")
		}
	})

	t.Run("verify ledger append-only rules", func(t *testing.T) {
		// First create required parent records
		var accountID, journalID string
		err := pool.QueryRow(ctx, `
			INSERT INTO accounts (type, name, currency, status)
			VALUES ('ASSET', 'Test Account', 'USD', 'ACTIVE')
			RETURNING id
		`).Scan(&accountID)
		if err != nil {
			t.Fatalf("Failed to create test account: %v", err)
		}

		err = pool.QueryRow(ctx, `
			INSERT INTO journal_entries (description)
			VALUES ('Test entry')
			RETURNING id
		`).Scan(&journalID)
		if err != nil {
			t.Fatalf("Failed to create test journal entry: %v", err)
		}

		// Insert a ledger entry
		var entryID string
		err = pool.QueryRow(ctx, `
			INSERT INTO ledger_entries (journal_entry_id, account_id, amount, direction, balance_after)
			VALUES ($1, $2, 100.00, 'DEBIT', 100.00)
			RETURNING id
		`, journalID, accountID).Scan(&entryID)
		if err != nil {
			t.Fatalf("Failed to insert ledger entry: %v", err)
		}

		// Try to update (should fail due to rule)
		result, err := pool.Exec(ctx, `
			UPDATE ledger_entries SET amount = 200.00 WHERE id = $1
		`, entryID)
		if err != nil {
			t.Fatalf("Update query failed: %v", err)
		}
		// The rule makes updates do nothing, so rows affected should be 0
		if result.RowsAffected() != 0 {
			t.Error("Expected ledger_entries update to be blocked by rule")
		}
	})

	t.Run("migrate down", func(t *testing.T) {
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			t.Fatalf("Failed to migrate down: %v", err)
		}

		// Verify tables are removed
		var count int
		err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name NOT LIKE 'schema_%'
		`).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count tables: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected all tables to be removed after down migration, found %d", count)
		}
	})
}

// TestMigrationFiles verifies that migration files are properly paired (up/down)
func TestMigrationFiles(t *testing.T) {
	entries, err := MigrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("Failed to read migrations directory: %v", err)
	}

	upFiles := make(map[string]bool)
	downFiles := make(map[string]bool)

	for _, entry := range entries {
		name := entry.Name()
		if len(name) < 10 {
			continue
		}

		// Extract version (first 6 digits)
		version := name[:6]

		if len(name) > 7 && name[len(name)-7:] == ".up.sql" {
			upFiles[version] = true
		} else if len(name) > 9 && name[len(name)-9:] == ".down.sql" {
			downFiles[version] = true
		}
	}

	// Verify each up has a down
	for version := range upFiles {
		if !downFiles[version] {
			t.Errorf("Migration %s has .up.sql but no .down.sql", version)
		}
	}

	// Verify each down has an up
	for version := range downFiles {
		if !upFiles[version] {
			t.Errorf("Migration %s has .down.sql but no .up.sql", version)
		}
	}

	// Verify we have the expected number of migrations
	expectedMigrations := 14
	if len(upFiles) != expectedMigrations {
		t.Errorf("Expected %d migrations, found %d", expectedMigrations, len(upFiles))
	}

	fmt.Printf("Found %d migrations, all properly paired\n", len(upFiles))
}
