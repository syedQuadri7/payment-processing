# Database Schema Documentation

This directory documents the PostgreSQL database schema for the payment processing service.

## Overview

The database is organized into several logical groups:

| Category | Tables | Purpose |
|----------|--------|---------|
| Core Payment | `payment_intents`, `payment_methods`, `payment_attempts` | Payment lifecycle tracking |
| Authorization | `authorization_holds` | Hold tracking and expiration |
| Ledger | `accounts`, `journal_entries`, `ledger_entries` | Double-entry bookkeeping |
| Events | `outbox` | Transactional outbox for CDC |
| Audit | `audit_log` | Change history and compliance |
| Configuration | `decline_code_mappings` | Provider decline code mapping |

## Schema Diagrams

### Core Payment Flow

```
┌─────────────────┐     ┌─────────────────┐
│ payment_intents │────▶│ payment_methods │
└────────┬────────┘     └─────────────────┘
         │
         │ 1:1 (when authorized)
         ▼
┌─────────────────────┐
│ authorization_holds │
└────────┬────────────┘
         │
         │ 1:N
         ▼
┌─────────────────┐
│ payment_attempts│
└─────────────────┘
```

### Double-Entry Ledger

```
┌─────────────────┐
│    accounts     │
└────────┬────────┘
         │
         │ N:1
         ▼
┌─────────────────┐     ┌─────────────────┐
│ ledger_entries  │────▶│ journal_entries │
└─────────────────┘     └─────────────────┘
```

## Document Index

- [Core Tables](core-tables.md) - Payment intents, methods, attempts, holds
- [Ledger Tables](ledger-tables.md) - Accounts, journal entries, ledger entries
- [Outbox and Audit](outbox-audit.md) - Event publishing and audit trails

## Key Design Decisions

### 1. Decimal Precision

All monetary values use `DECIMAL(19,4)` for precision:
- 19 total digits accommodate large amounts
- 4 decimal places support fractional currencies

### 2. UUID Primary Keys

All tables use UUID primary keys:
- No sequential ID guessing
- Safe for distributed generation
- Suitable for external exposure

### 3. Timestamp Precision

All timestamps use `TIMESTAMPTZ` (timestamp with time zone):
- Microsecond precision
- Timezone-aware storage
- Consistent across deployments

### 4. Immutable Patterns

Several tables follow append-only patterns:
- `ledger_entries` - Never updated, corrections via new entries
- `payment_attempts` - Each attempt is a new record
- `audit_log` - Append-only change history
- `outbox` - Events never modified after creation

### 5. Optimistic Locking

Tables with concurrent updates use version columns:
- `accounts.version` - Prevent lost balance updates
- Check-and-set pattern for updates

## Migration Files

Migrations are in `/migrations/`:

| File | Description |
|------|-------------|
| `001_create_accounts.up.sql` | Account tables |
| `002_create_intents.up.sql` | Payment intent and method tables |
| `003_create_holds.up.sql` | Authorization hold table |
| `004_create_attempts.up.sql` | Payment attempt table |
| `005_create_ledger.up.sql` | Journal and ledger entry tables |
| `006_create_outbox.up.sql` | Outbox table for CDC |
| `007_create_decline_mappings.up.sql` | Decline code mapping table |
| `008_seed_decline_codes.up.sql` | Initial decline code data |

## Running Migrations

```bash
# Apply all migrations
go run cmd/migrate/main.go up

# Rollback last migration
go run cmd/migrate/main.go down

# Check migration status
go run cmd/migrate/main.go status
```

## Connection Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | - | Full connection string |
| `DB_HOST` | `localhost` | Database host |
| `DB_PORT` | `5432` | Database port |
| `DB_NAME` | `payments` | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |
| `DB_SSLMODE` | `disable` | SSL mode |

## PgBouncer

Production uses PgBouncer for connection pooling:

```
Application → PgBouncer (port 6432) → PostgreSQL (port 5432)
```

Configuration:
- Pool mode: `transaction`
- Max connections per pool: 20
- Default pool size: 10
