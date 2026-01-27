# Payment Processing Service

A production-grade payment processing service built with Go and Temporal, demonstrating patterns derived from industry leaders including Stripe, Square/Block, and Adyen.

## Overview

This service implements durable workflow orchestration for payment operations that may span hours or days. It combines intelligent payment retry logic with core banking transaction patterns relevant to credit unions and financial institutions.

**Key Architectural Insight:** Payments are promises about money movement, not money movement itself. Every design decision flows from understanding the distinction between authorization (the promise) and settlement (the actual transfer).

### Core Features

- **Multi-Provider Support** - Adapters for Stripe, Adyen, and PayPal with canonical event normalization
- **Durable Workflows** - Temporal-based orchestration surviving process crashes and infrastructure failures
- **Intelligent Retry** - Decline classification with context-aware retry scheduling
- **Double-Entry Bookkeeping** - Mathematically provable correctness with clearing account monitoring
- **Transactional Outbox** - CDC-based event publishing preventing dual-write issues
- **Idempotent Operations** - Atomic phases with recovery points for safe retries

## Architecture

```
                    ADAPTER LAYER (Edge)
    ┌──────────────────────────────────────────────────┐
    │  Stripe       Adyen        PayPal                │
    │  Webhook      Webhook      Webhook               │
    │     │            │            │                  │
    │     └────────────┼────────────┘                  │
    │                  ▼                               │
    │          Canonical Event                         │
    └──────────────────┼───────────────────────────────┘
                       │
    ┌──────────────────┼───────────────────────────────┐
    │                  ▼           CORE LAYER          │
    │  ┌─────────────────────┐  ┌───────────────────┐  │
    │  │     API Server      │  │ Temporal Workflows│  │
    │  │                     │─▶│                   │  │
    │  │  POST /intents      │  │  PaymentWorkflow  │  │
    │  │  POST /authorize    │  │  RecoveryWorkflow │  │
    │  └─────────────────────┘  └───────────────────┘  │
    └──────────────────────────────────────────────────┘
                       │
    ┌──────────────────┼───────────────────────────────┐
    │                  ▼           DATA LAYER          │
    │  PostgreSQL              Kafka                   │
    │  ├─ payment_intents      ├─ payments.authorized  │
    │  ├─ authorization_holds  ├─ payments.captured    │
    │  ├─ payment_attempts     └─ payments.failed      │
    │  ├─ ledger_entries                               │
    │  └─ outbox ──────────▶ (via Debezium CDC)       │
    └──────────────────────────────────────────────────┘
```

### Domain Model

The system separates three core concerns:

| Concept | Purpose | Examples |
|---------|---------|----------|
| **PaymentIntent** | WHAT is being paid | Amount, currency, customer |
| **PaymentMethod** | HOW it's being paid | Card token, bank account, wallet |
| **Order** | WHAT is being purchased | Line items, merchant, invoice |

This separation enables support for diverse payment methods without architectural rewrites.

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Language | Go 1.24 | Performance, strong typing, concurrency |
| Workflow Engine | Temporal | Durable execution, retry handling |
| Database | PostgreSQL | ACID compliance, CDC support |
| Connection Pooler | PgBouncer | Transaction pooling |
| Event Streaming | Kafka | Event replay, schema registry |
| CDC | Debezium | Database change capture |

## Getting Started

### Prerequisites

- Go 1.24+
- Temporal Server (local or cloud)
- PostgreSQL 14+
- Docker and Docker Compose (for local development)

### Quick Start

```bash
# Clone the repository
git clone <repository-url>
cd payment-processing

# Install dependencies
go mod download

# Start infrastructure (Temporal, PostgreSQL)
make dev-up

# Build all services
make build

# Start all services (in separate terminals or use docker-compose)
./bin/payment-api      # REST API on port 8080
./bin/payment-worker   # Temporal worker
```

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `STRIPE_SECRET_KEY` | - | Stripe API key (test mode) |
| `STRIPE_WEBHOOK_SECRET` | - | Stripe webhook signing secret |

## API Reference

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/intents` | Create payment intent |
| GET | `/api/v1/intents/:id` | Get intent status |
| PUT | `/api/v1/intents/:id/method` | Attach payment method |
| POST | `/api/v1/intents/:id/authorize` | Request authorization |
| POST | `/api/v1/intents/:id/capture` | Capture authorized funds |
| POST | `/webhooks/stripe` | Stripe webhook receiver |
| POST | `/webhooks/adyen` | Adyen notification receiver |
| GET | `/health` | Health check |

### Create Payment Intent

```bash
curl -X POST http://localhost:8080/api/v1/intents \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: unique-key-123" \
  -d '{
    "amount": "100.00",
    "currency": "USD",
    "customer_id": "cust_123",
    "provider": "STRIPE",
    "capture_method": "manual"
  }'
```

## Payment Lifecycle

```
Intent Created → Authorization → Capture → Settlement
      │               │             │
      │          (hold placed)  (funds claimed)
      │               │             │
      └── Can update  └── Can void  └── Can refund
          method
```

### Decline Handling

Declines are classified into categories determining retry behavior:

| Category | Examples | Retry Eligible |
|----------|----------|----------------|
| **Soft** | Insufficient funds, over limit | Yes |
| **Hard** | Card expired, account closed | No |
| **Fraud** | Stolen card, fraud suspicion | No |
| **Temporary** | Rate limit, timeout | Yes (activity-level) |

## Project Structure

```
payment-processing/
├── services/                       # Microservices
│   ├── payment-api/               # REST API service
│   │   ├── cmd/                   # Entry point
│   │   └── internal/              # Handlers, middleware
│   ├── payment-worker/            # Temporal worker service
│   │   ├── cmd/                   # Entry point
│   │   └── internal/              # Worker, workflows, activities
│   └── provider-simulator/        # Testing tool for webhooks
├── shared/                         # Shared libraries
│   ├── domain/                    # Domain types and interfaces
│   ├── repository/                # Database access layer
│   ├── adapter/                   # Provider webhook adapters
│   ├── database/                  # Migrations
│   ├── logging/                   # Structured logging
│   ├── outbox/                    # Transactional outbox
│   └── workflowtypes/             # Shared workflow types
├── infrastructure/                 # Docker and scripts
│   └── docker/                    # Docker Compose files
├── docs/                          # Documentation
├── go.mod
└── Makefile
```

## Documentation

| Document | Description |
|----------|-------------|
| [Documentation Index](docs/readme.md) | Start here - navigation to all docs |
| [Project Overview](docs/overview.md) | Goals, scope, and technology stack |
| [System Design](docs/architecture/system-design.md) | Architecture and data flow |
| [Domain Model](docs/architecture/domain-model.md) | Entities and state machines |
| [Functional Requirements](docs/requirements/functional.md) | What the system does |
| [Non-Functional Requirements](docs/requirements/non-functional.md) | Performance, reliability, security |
| [Research](docs/research/payment-systems-research.md) | Production patterns from Stripe, Square, Adyen |

### Architecture Decisions

| Document | Description |
|----------|-------------|
| [Decision Index](docs/decisions/readme.md) | Overview of all decisions |
| [Finalized Decisions](docs/decisions/finalized-decisions.md) | 19 decisions with simple defaults |
| [Deferred Decisions](docs/decisions/deferred-decisions.md) | 7 enterprise-scale (documented only) |

## Development

### Running Tests

```bash
# All tests
go test ./...

# Specific package (e.g., workflow tests)
go test ./services/payment-worker/internal/workflow/...

# With coverage
go test -cover ./...
```

### Building

```bash
# Build all services
make build

# Build individual services
make build-api
make build-worker
make build-simulator

# Run services (after building)
./bin/payment-api
./bin/payment-worker
```

### Docker Development

```bash
# Start dev dependencies (PostgreSQL, Temporal)
make dev-up

# Start all services including API and worker
make dev-up-all

# Stop development stack
make dev-down

# Start test environment with provider simulator
make test-up
```

## Key Patterns Implemented

### Transactional Outbox

Events are written to an outbox table within the same database transaction as business data. Debezium CDC relays events to Kafka, ensuring reliable delivery without dual-write issues.

### Idempotency with Atomic Phases

Operations are broken into phases with recovery points:

1. `started` - Request received
2. `validated` - Input validated
3. `authorized` - Provider called
4. `ledger_written` - Entries created
5. `completed` - All done

### Linear State Machines

PaymentAttempt records are immutable. Each retry creates a new attempt with its own linear state progression, avoiding circular state machine complexity.

### Clearing Account Monitoring

Non-zero clearing account balances exceeding thresholds trigger alerts indicating unresolved issues requiring investigation.

## License

[License details]

---

*A learning project demonstrating production-grade payment processing patterns.*
