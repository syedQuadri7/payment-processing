# Payment Worker Service

The Payment Worker service runs Temporal workflows and activities for payment processing.

## Responsibilities

- Execute PaymentIntentWorkflow for payment lifecycle management
- Authorization, capture, and void activities
- Decline classification and retry logic
- Double-entry ledger activities
- Audit log and outbox writing activities

## Building

```bash
# From project root
make build-worker

# Or directly
go build -o bin/payment-worker ./services/payment-worker/cmd
```

## Running

```bash
# Set required environment variables
export TEMPORAL_HOST=localhost:7233
export DB_HOST=localhost
export DB_USER=payment
export DB_PASSWORD=payment_secret
export DB_NAME=payment_processing

# Run the service
./bin/payment-worker
```

## Docker

```bash
# Build image
make docker-build-worker

# Or directly
docker build -t payment-worker:latest -f services/payment-worker/Dockerfile .
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_USER` | - | PostgreSQL username |
| `DB_PASSWORD` | - | PostgreSQL password |
| `DB_NAME` | - | PostgreSQL database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |

## Task Queue

The worker listens on the `payment-processing` task queue.

## Workflows

### PaymentIntentWorkflow

Manages the complete payment lifecycle:

1. **CREATED** - Initial state
2. **REQUIRES_METHOD** - Waiting for payment method
3. **AUTHORIZING** - Authorization in progress
4. **AUTHORIZED** - Authorization successful, hold placed
5. **RECOVERING** - Soft decline, retry scheduled
6. **CAPTURING** - Capture in progress
7. **SUCCEEDED** - Payment complete
8. **FAILED** - Terminal failure
9. **CANCELED** - Void or cancellation

### Signals

- `payment-event` - Provider webhook events
- `update-method` - Payment method update (triggers retry)
- `capture` - Request capture
- `cancel` - Request cancellation

### Queries

- `status` - Current payment status and details

## Activities

See `workflow/activities.go` for the complete list of activities.
