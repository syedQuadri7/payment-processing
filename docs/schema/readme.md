# Database Schema Documentation

PostgreSQL database schema for the payment processing service.

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
| Idempotency | `processed_events` | Webhook deduplication |

## Entity Relationships

### Core Payment Flow

```
payment_intents ──────► payment_methods
       │
       │ 1:1 (when authorized)
       ▼
authorization_holds
       │
       │ 1:N (each retry creates new attempt)
       ▼
payment_attempts
```

### Double-Entry Ledger

```
accounts
    │
    │ N:1
    ▼
ledger_entries ──────► journal_entries
```

## Document Index

| Document | Contents |
|----------|----------|
| [Core Tables](core-tables.md) | Payment intents, methods, attempts, holds, decline mappings |
| [Ledger Tables](ledger-tables.md) | Accounts, journal entries, ledger entries, clearing accounts |
| [Outbox and Audit](outbox-audit.md) | Event publishing, audit trails, processed events |

## Key Design Principles

### Monetary Precision

All monetary values must use high precision decimal storage:
- 19 total digits to accommodate large amounts
- 4 decimal places to support fractional currencies
- Never use floating point for money

### Primary Keys

All tables use UUID primary keys:
- Prevents sequential ID enumeration
- Safe for distributed ID generation
- Suitable for external API exposure

### Timestamps

All timestamps must be timezone-aware with microsecond precision for accurate ordering and auditing.

### Immutability Patterns

Several tables follow append-only patterns to maintain audit trails:

| Table | Pattern | Rationale |
|-------|---------|-----------|
| `ledger_entries` | Append-only | Corrections via new entries, never updates |
| `payment_attempts` | Append-only | Each attempt is a new record |
| `audit_log` | Append-only | Complete change history |
| `outbox` | Append-only | Events never modified after creation |

### Concurrency Control

Tables with concurrent updates require optimistic locking:
- `accounts` table needs version column for balance updates
- Check-and-set pattern prevents lost updates

## Migrations

### Migration Sequence

Migrations must be created in this order due to foreign key dependencies:

1. Account tables (no dependencies)
2. Payment intent and method tables
3. Authorization hold table (depends on intents)
4. Payment attempt table (depends on intents)
5. Journal and ledger entry tables (depends on accounts)
6. Outbox table (no dependencies)
7. Decline code mapping table (no dependencies)
8. Seed data for decline codes

### Migration Requirements

- Each migration must be reversible
- Data migrations must be idempotent
- Schema changes must not lock tables for extended periods
- Test migrations against production-sized data before deployment

## Configuration

### Connection Settings

| Variable | Default | Purpose |
|----------|---------|---------|
| `DATABASE_URL` | - | Full connection string (preferred) |
| `DB_HOST` | `localhost` | Database host |
| `DB_PORT` | `5432` | Database port |
| `DB_NAME` | `payments` | Database name |
| `DB_USER` | - | Database user |
| `DB_PASSWORD` | - | Database password |
| `DB_SSLMODE` | `disable` | SSL mode (require in production) |

### Connection Pooling

Production deployments require connection pooling via PgBouncer:

| Setting | Recommended Value | Rationale |
|---------|-------------------|-----------|
| Pool mode | `transaction` | Connections returned after each transaction |
| Max connections | 20 per pool | Prevent database connection exhaustion |
| Default pool size | 10 | Balance between availability and resources |

### CDC Requirements

For the transactional outbox pattern:
- Outbox table requires `REPLICA IDENTITY FULL` for CDC capture
- Debezium connector reads PostgreSQL WAL
- CDC user needs replication permissions
