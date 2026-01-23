# Payment API Service

The Payment API service exposes REST endpoints for payment operations and receives webhooks from payment providers.

## Responsibilities

- REST API for payment intents (create, get, capture, cancel)
- Webhook endpoints for Stripe, Adyen, PayPal
- Audit log query endpoints
- Health and metrics endpoints
- Outbox consumer for event publishing

## Building

```bash
# From project root
make build-api

# Or directly
go build -o bin/payment-api ./services/payment-api/cmd
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
./bin/payment-api
```

## Docker

```bash
# Build image
make docker-build-api

# Or directly
docker build -t payment-api:latest -f services/payment-api/Dockerfile .
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_USER` | - | PostgreSQL username |
| `DB_PASSWORD` | - | PostgreSQL password |
| `DB_NAME` | - | PostgreSQL database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `OUTBOX_CONSUMER_ENABLED` | `true` | Enable outbox polling consumer |
| `STRIPE_WEBHOOK_SECRET` | - | Stripe webhook signing secret |
| `ADYEN_HMAC_KEY` | - | Adyen HMAC signing key |
| `PAYPAL_CLIENT_ID` | - | PayPal client ID |
| `PAYPAL_CLIENT_SECRET` | - | PayPal client secret |
| `PAYPAL_WEBHOOK_ID` | - | PayPal webhook ID |

## Endpoints

See `docs/api/` for full API documentation.

### Health Endpoints
- `GET /health` - Overall health status
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe
- `GET /metrics` - Prometheus metrics

### Payment Intent Endpoints
- `POST /api/v1/intents` - Create payment intent
- `GET /api/v1/intents/:id` - Get payment intent
- `PUT /api/v1/intents/:id/method` - Attach payment method
- `POST /api/v1/intents/:id/capture` - Capture payment
- `POST /api/v1/intents/:id/cancel` - Cancel payment

### Webhook Endpoints
- `POST /webhooks/stripe` - Stripe webhooks
- `POST /webhooks/adyen` - Adyen webhooks
- `POST /webhooks/paypal` - PayPal webhooks
