# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

Payment Processing Service - a Go backend using Temporal for durable workflow orchestration. This is a learning project focused on production-grade patterns for financial transaction handling in the credit union/banking domain.

## Claude's Role

This project follows a **waterfall-hybrid development process**. Claude assists by:

1. **Design Before Code** - Research, document, and plan before implementing. When asked to build a feature, first check if documentation exists. If not, create or update docs before writing code.

2. **Documentation-Driven Development** - The `docs/` folder is the source of truth. Implementation should follow what's documented in requirements and technical specs.

3. **Architectural Consistency** - Follow the patterns documented in finalized decisions. Don't introduce new patterns without documenting the decision first.

4. **Learning Project Support** - Help explain concepts, trade-offs, and industry patterns. This project exists to learn production-grade payment system design.

## Documentation Structure

All documentation lives in `docs/`. Start with `docs/_reference.md` for navigation.

| Folder | Purpose |
|--------|---------|
| `docs/_reference.md` | **Start here** - Index of all documentation |
| `docs/overview.md` | Project goals, scope, and technology stack |
| `docs/architecture/` | System design and domain model |
| `docs/requirements/` | Functional and non-functional requirements |
| `docs/decisions/` | Architectural decisions (finalized and deferred) |
| `docs/research/` | Industry patterns and background research |
| `docs/api/` | API endpoint documentation with examples |
| `docs/schema/` | Database schema documentation |
| `docs/simulations/` | Testing tools and webhook simulator usage |

### Before Implementing

1. Check `docs/requirements/functional.md` for functional requirements
2. Check `docs/requirements/non-functional.md` for quality attributes
3. Check `docs/architecture/system-design.md` for architecture overview
4. Check `docs/decisions/finalized-decisions.md` for implementation choices
5. Check `docs/schema/` for database structure

### When Adding Features

1. Update or create documentation first
2. Follow existing patterns from finalized decisions
3. Update `docs/_reference.md` if adding new documents

## Project Structure

```text
payment-processing/
├── docs/                           # Documentation (unchanged)
├── infrastructure/                 # Infrastructure configuration
│   ├── docker/                    # Docker Compose files
│   └── scripts/                   # Deployment scripts
│
├── services/                       # All microservices
│   ├── payment-api/               # REST API service
│   │   ├── cmd/                   # Service entry point
│   │   ├── internal/              # Service-specific code
│   │   │   ├── handlers/          # HTTP request handlers
│   │   │   └── middleware/        # HTTP middleware
│   │   ├── server.go              # Legacy server setup
│   │   ├── errors.go              # API error types
│   │   ├── types.go               # API request/response types
│   │   └── Dockerfile
│   │
│   ├── payment-worker/            # Temporal worker service
│   │   ├── cmd/                   # Service entry point
│   │   ├── internal/              # Service-specific code
│   │   │   ├── worker/            # Worker setup and registration
│   │   │   └── workflow/          # Workflow and activity definitions
│   │   └── Dockerfile
│   │
│   └── provider-simulator/        # Test simulator service (standalone)
│
├── shared/                         # Shared libraries (used by all services)
│   ├── domain/                    # Domain types and interfaces
│   ├── repository/                # Database access layer
│   ├── adapter/                   # Provider webhook adapters
│   ├── database/                  # Database migrations
│   ├── logging/                   # Structured logging
│   ├── outbox/                    # Outbox pattern implementation
│   └── workflowtypes/             # Shared workflow types and constants
│
├── go.mod                         # Root module
├── go.sum
├── Makefile                       # Build and development commands
└── README.md
```

## Build and Run Commands

```bash
# Build all services
make build

# Build individual services
make build-api
make build-worker
make build-simulator

# Run tests
go test ./...

# Run single test
go test -run TestName ./services/payment-worker/internal/workflow/...
```

## Docker Development

```bash
# Start development dependencies (postgres, temporal)
make dev-up

# Start all services including API and worker
make dev-up-all

# Stop development stack
make dev-down

# Start test environment with provider simulator
make test-up

# Stop test environment
make test-down
```

## Architecture

The system is split into two independently deployable services:

### Payment API Service (`services/payment-api/`)
- REST API for payment operations (port 8080)
- Webhook endpoints for Stripe, Adyen, PayPal
- Outbox consumer for event publishing
- Entry point: `services/payment-api/cmd/main.go`

### Payment Worker Service (`services/payment-worker/`)
- Temporal worker on `payment-processing` task queue
- Executes workflows and activities
- Entry point: `services/payment-worker/cmd/main.go`

See `docs/architecture/system-design.md` for full architecture diagrams.

### Key Components

| Path | Purpose |
|------|---------|
| `services/payment-api/cmd/` | API service entry point |
| `services/payment-api/internal/handlers/` | HTTP request handlers |
| `services/payment-api/internal/middleware/` | HTTP middleware (auth, rate limit, etc.) |
| `services/payment-worker/cmd/` | Worker service entry point |
| `services/payment-worker/internal/worker/` | Temporal worker setup and registration |
| `services/payment-worker/internal/workflow/` | Workflow and activity definitions |
| `services/provider-simulator/` | Testing tool for simulating provider webhooks |
| `shared/domain/` | Shared domain types and interfaces |
| `shared/repository/` | Database access layer |
| `shared/adapter/` | Provider-specific webhook adapters |
| `shared/workflowtypes/` | Shared workflow types and constants |
| `shared/logging/` | Shared structured logging |
| `infrastructure/docker/` | Docker Compose configurations |

## Key Architectural Patterns

These are documented in `docs/decisions/finalized-decisions.md`. Follow them consistently.

| Pattern | Decision | Summary |
|---------|----------|---------|
| Temporal Workflows | FD-011 | Durable execution for payment lifecycle |
| Multi-Provider Adapters | FD-012 | Normalize provider webhooks at the edge |
| Transactional Outbox | FD-013 | Reliable event publishing via CDC |
| Double-Entry Bookkeeping | FD-014 | Mathematical correctness for money movement |
| Failure Handling | FD-015 | Log and manual intervention for stuck workflows |
| API Versioning | FD-016 | Stripe-style date-based versioning |
| API Authentication | FD-017 | Key + secret pairs |

## Temporal Patterns

Use the `/temporal-workflow` skill when implementing workflows, activities, signals, or queries.

Key constraints:
- Workflows must be deterministic - use `workflow.Now(ctx)` not `time.Now()`
- Side effects (DB, APIs, randomness) belong in activities only
- Activities should be idempotent where possible
- Use typed errors to control retry behavior

Task queue: `payment-processing`

## API Documentation

Full API documentation with request/response examples is in `docs/api/`.

| Endpoint | Documentation |
|----------|---------------|
| Payment Intents | `docs/api/internal/intents.md` |
| Accounts | `docs/api/internal/accounts.md` |
| Health/Metrics | `docs/api/internal/health.md` |
| Stripe Webhooks | `docs/api/webhooks/stripe.md` |
| Adyen Webhooks | `docs/api/webhooks/adyen.md` |
| PayPal Webhooks | `docs/api/webhooks/paypal.md` |

## Database Schema

Schema documentation is in `docs/schema/`. Key tables:

| Table | Documentation |
|-------|---------------|
| payment_intents, payment_attempts | `docs/schema/core-tables.md` |
| accounts, ledger_entries | `docs/schema/ledger-tables.md` |
| outbox, audit_log | `docs/schema/outbox-audit.md` |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `PORT` | `8080` | HTTP server port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_USER` | - | PostgreSQL username |
| `DB_PASSWORD` | - | PostgreSQL password |
| `DB_NAME` | - | PostgreSQL database name |
| `STRIPE_WEBHOOK_SECRET` | - | Stripe webhook signing secret |
| `ADYEN_HMAC_KEY` | - | Adyen HMAC signing key |

## Testing

Use the provider simulator for integration testing:

```bash
# Start test environment with simulator
make test-up

# Or use the simulator directly
cd services/provider-simulator
./provider-simulator send stripe payment_intent.succeeded --amount 10000
```

See `docs/simulations/readme.md` for full testing documentation.
