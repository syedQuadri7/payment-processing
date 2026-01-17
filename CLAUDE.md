# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Payment Processing Service - a Go backend using Temporal for durable workflow orchestration. This is a learning project focused on production-grade patterns for financial transaction handling in the credit union/banking domain.

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

Request flow:
```
POST /payment → API Server → Temporal Client → starts ProcessPaymentWorkflow
                                                    ↓
                                              ValidatePayment activity
                                                    ↓
                                              ExecutePayment activity
                                                    ↓
                                              Returns PaymentResult
```

### Key Components

| Path | Purpose |
|------|---------|
| `main.go` | Entry point - starts API server and worker |
| `server/` | HTTP handlers for `/payment`, `/payment/status`, `/health` |
| `worker/` | Temporal worker setup and registration |
| `workflow/` | Workflow definitions and activities |
| `technical-spec.md` | Full specification including future roadmap |

## Temporal Patterns

Use the `/temporal-workflow` skill when implementing workflows, activities, signals, or queries.

Key constraints:
- Workflows must be deterministic - use `workflow.Now(ctx)` not `time.Now()`
- Side effects (DB, APIs, randomness) belong in activities only
- Activities should be idempotent where possible
- Use typed errors to control retry behavior

Task queue: `payment-processing` (defined in `workflow.TaskQueueName`)

## API Endpoints

- `POST /payment` - Submit payment `{"amount": 100.00, "from": "acc1", "to": "acc2"}`
- `GET /payment/status?workflow_id=xxx` - Query workflow status
- `GET /health` - Health check

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `PORT` | `8080` | HTTP server port |
