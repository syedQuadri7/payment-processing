# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

Payment Processing Service - a Go backend using Temporal for durable workflow orchestration. This is a learning project focused on production-grade patterns for financial transaction handling in the credit union/banking domain.

## Claude's Role

This project follows a **waterfall-hybrid development process**. Claude assists by:

1. **Design Before Code** - Research, document, and plan before implementing. When asked to build a feature, first check if documentation exists. If not, create or update docs before writing code.

2. **Documentation-Driven Development** - The `docs/` folder is the source of truth. Implementation should follow what's documented in requirements and technical specs.

3. **Architectural Consistency** - Follow the patterns documented in ADRs (Architectural Decision Records). Don't introduce new patterns without documenting the decision first.

4. **Learning Project Support** - Help explain concepts, trade-offs, and industry patterns. This project exists to learn production-grade payment system design.

## Documentation Structure

All documentation lives in `docs/`. Start with `docs/_reference.md` for navigation.

| Folder | Purpose |
|--------|---------|
| `docs/_reference.md` | **Start here** - Index of all documentation |
| `docs/requirements/` | What we're building (service-requirements.md, technical-spec.md) |
| `docs/research/` | Industry patterns and background research |
| `docs/decisions/` | ADRs explaining why we made specific choices |
| `docs/api/` | API endpoint documentation with examples |
| `docs/schema/` | Database schema documentation |
| `docs/simulations/` | Testing tools and webhook simulator usage |

### Before Implementing

1. Check `docs/requirements/service-requirements.md` for requirements
2. Check `docs/requirements/technical-spec.md` for implementation guidance
3. Check `docs/decisions/` for relevant architectural decisions
4. Check `docs/schema/` for database structure

### When Adding Features

1. Update or create documentation first
2. Follow existing patterns from ADRs
3. Update `docs/_reference.md` if adding new documents

## Build and Run Commands

```bash
# Build
go build -o payment-processing .

# Run (requires Temporal server on localhost:7233)
./payment-processing

# Run with custom settings
TEMPORAL_HOST=localhost:7233 PORT=8080 ./payment-processing

# Run tests
go test ./...

# Run single test
go test -run TestName ./workflow/...
```

## Architecture

The service runs two components in a single binary:
- **HTTP API Server** (port 8080) - accepts payment requests, returns 202 with workflow ID
- **Temporal Worker** - executes workflows and activities on the `payment-processing` task queue

See `docs/requirements/technical-spec.md` for full architecture diagrams.

### Key Components

| Path | Purpose |
|------|---------|
| `main.go` | Entry point - starts API server and worker |
| `server/` | HTTP handlers |
| `worker/` | Temporal worker setup and registration |
| `workflow/` | Workflow definitions and activities |
| `internal/adapter/` | Provider-specific webhook adapters |
| `internal/repository/` | Database access layer |
| `migrations/` | Database migrations |
| `tools/webhook-simulator/` | Testing tool for simulating provider webhooks |

## Key Architectural Patterns

These are documented in `docs/decisions/`. Follow them consistently.

| Pattern | ADR | Summary |
|---------|-----|---------|
| Temporal Workflows | ADR-001 | Durable execution for payment lifecycle |
| Multi-Provider Adapters | ADR-002 | Normalize provider webhooks at the edge |
| Transactional Outbox | ADR-003 | Reliable event publishing via CDC |
| Double-Entry Bookkeeping | ADR-004 | Mathematical correctness for money movement |

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
| `DATABASE_URL` | - | PostgreSQL connection string |
| `STRIPE_WEBHOOK_SECRET` | - | Stripe webhook signing secret |
| `ADYEN_HMAC_KEY` | - | Adyen HMAC signing key |

## Testing

Use the webhook simulator for integration testing:

```bash
cd tools/webhook-simulator
./webhook-simulator send stripe payment_intent.succeeded --amount 10000
```

See `docs/simulations/readme.md` for full testing documentation.
