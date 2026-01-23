# Provider Simulator

A comprehensive provider simulation service that can operate in two modes:

1. **CLI Mode**: Send individual webhooks or run scenario files for testing webhook handlers
2. **Server Mode**: Full HTTP API simulation of Stripe, Adyen, and PayPal payment APIs

## Quick Start

```bash
# Build the simulator
go build -o provider-simulator .

# Start in server mode (simulates provider APIs)
./provider-simulator server --port 9000

# Or use CLI mode to send webhooks
./provider-simulator send stripe payment_intent.succeeded --amount 10000
```

## Server Mode

Server mode provides a full simulation of payment provider APIs with stateful payment flows, automatic webhook delivery, and configurable behaviors.

### Starting the Server

```bash
# Start on default port 9000
./provider-simulator server

# Custom port and webhook target
./provider-simulator server --port 9001 --webhook-target http://localhost:8080

# With webhook delay for realistic timing
./provider-simulator server --webhook-delay 1000
```

### Server Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `--port, -p` | `9000` | HTTP server port |
| `--webhook-target` | `http://localhost:8080` | Target for webhook delivery |
| `--webhook-secret` | `whsec_test_secret` | Secret for webhook signatures |
| `--webhook-delay` | `0` | Delay (ms) before sending webhooks |
| `--verbose, -v` | `false` | Enable verbose logging |

### Provider APIs

#### Stripe API (port 9000)

```bash
# Create a payment intent
curl -X POST http://localhost:9000/stripe/v1/payment_intents \
  -d '{"amount": 10000, "currency": "usd"}'

# Confirm with a test card
curl -X POST http://localhost:9000/stripe/v1/payment_intents/pi_xxx/confirm \
  -d '{"payment_method_data": {"type": "card", "card": {"number": "4242424242424242"}}}'

# Capture (for manual capture)
curl -X POST http://localhost:9000/stripe/v1/payment_intents/pi_xxx/capture

# Cancel
curl -X POST http://localhost:9000/stripe/v1/payment_intents/pi_xxx/cancel

# Create refund
curl -X POST http://localhost:9000/stripe/v1/refunds \
  -d '{"payment_intent": "pi_xxx", "amount": 5000}'
```

#### Adyen API (v71)

```bash
# Create a payment
curl -X POST http://localhost:9000/adyen/v71/payments \
  -d '{"amount": {"currency": "EUR", "value": 15000}, "merchantAccount": "TestMerchant", "reference": "order_123"}'

# Capture
curl -X POST http://localhost:9000/adyen/v71/payments/ADYEN_xxx/captures \
  -d '{"merchantAccount": "TestMerchant"}'

# Cancel
curl -X POST http://localhost:9000/adyen/v71/payments/ADYEN_xxx/cancels \
  -d '{"merchantAccount": "TestMerchant"}'

# Refund
curl -X POST http://localhost:9000/adyen/v71/payments/ADYEN_xxx/refunds \
  -d '{"merchantAccount": "TestMerchant", "amount": {"currency": "EUR", "value": 5000}}'
```

#### PayPal API (v2)

```bash
# Get OAuth token (mock)
curl -X POST http://localhost:9000/paypal/v1/oauth2/token \
  -d 'grant_type=client_credentials'

# Create order
curl -X POST http://localhost:9000/paypal/v2/checkout/orders \
  -d '{"intent": "CAPTURE", "purchase_units": [{"amount": {"currency_code": "USD", "value": "100.00"}}]}'

# Authorize order
curl -X POST http://localhost:9000/paypal/v2/checkout/orders/ORDER_ID/authorize

# Capture order (direct)
curl -X POST http://localhost:9000/paypal/v2/checkout/orders/ORDER_ID/capture

# Capture authorization
curl -X POST http://localhost:9000/paypal/v2/payments/authorizations/AUTH_ID/capture

# Void authorization
curl -X POST http://localhost:9000/paypal/v2/payments/authorizations/AUTH_ID/void

# Refund capture
curl -X POST http://localhost:9000/paypal/v2/payments/captures/CAP_ID/refund \
  -d '{"amount": {"currency_code": "USD", "value": "50.00"}}'
```

### Admin API

```bash
# Reset all state
curl -X POST http://localhost:9000/admin/reset

# List all payments
curl http://localhost:9000/admin/payments

# Filter by provider
curl http://localhost:9000/admin/payments?provider=stripe

# Get payment details
curl http://localhost:9000/admin/payments/pi_xxx

# Configure behavior for next action
curl -X POST http://localhost:9000/admin/payments/pi_xxx/behavior \
  -d '{"decline_on_next": "INSUFFICIENT_FUNDS"}'

# List webhooks
curl http://localhost:9000/admin/webhooks

# Redeliver a webhook
curl -X POST http://localhost:9000/admin/webhooks/wh_xxx/redeliver

# Get stats
curl http://localhost:9000/admin/stats

# Health check
curl http://localhost:9000/health
```

### Test Card Numbers

Use these card numbers to trigger specific behaviors:

| Card Number | Behavior |
|-------------|----------|
| `4242424242424242` | Success |
| `4000000000003220` | Success with 5s delay |
| `4000000000000002` | Generic decline |
| `4000000000009995` | Insufficient funds |
| `4000000000000069` | Expired card |
| `4000000000009987` | Lost card |
| `4000000000009979` | Stolen card |
| `4000000000000127` | Invalid CVC |
| `4000000000000341` | Timeout (30s) |
| `4100000000000019` | Fraud suspicion |

### Metadata-Based Triggers

You can also trigger behaviors via request metadata:

```bash
# Decline via metadata (overrides card behavior)
curl -X POST http://localhost:9000/stripe/v1/payment_intents \
  -d '{
    "amount": 10000,
    "currency": "usd",
    "metadata": {"x-sim-decline": "INSUFFICIENT_FUNDS"},
    "confirm": true
  }'

# Add delay via metadata
curl -X POST http://localhost:9000/stripe/v1/payment_intents \
  -d '{
    "amount": 10000,
    "currency": "usd",
    "metadata": {"x-sim-delay": "2000"}
  }'
```

| Metadata Key | Description |
|--------------|-------------|
| `x-sim-decline` | Decline code to trigger |
| `x-sim-delay` | Response delay in milliseconds |
| `x-sim-timeout` | Set to "true" for timeout |

---

## CLI Mode

CLI mode sends webhooks directly to your webhook handlers for testing.

### Send a Single Webhook

```bash
# Send a successful Stripe payment intent
./provider-simulator send stripe payment_intent.succeeded --amount 10000

# Send a failed payment with decline code
./provider-simulator send stripe payment_intent.payment_failed --decline-code insufficient_funds

# Dry run to see payload without sending
./provider-simulator send stripe charge.captured --dry-run

# Send Adyen authorization
./provider-simulator send adyen AUTHORISATION --amount 5000

# Send PayPal capture
./provider-simulator send paypal PAYMENT.CAPTURE.COMPLETED --amount 7500

# Test invalid signatures
./provider-simulator send stripe payment_intent.succeeded --invalid-key
./provider-simulator send stripe payment_intent.succeeded --skip-signature
```

### Run Scenario Files

```bash
# Run authorization and capture scenario
./provider-simulator run scenarios/auth_capture.yaml

# Run with custom target
./provider-simulator run scenarios/decline_soft.yaml --target http://localhost:9090

# Stop on first error
./provider-simulator run scenarios/invalid.yaml --stop-on-error
```

### CLI Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--target` | `-t` | `http://localhost:8080` | Target URL for webhook delivery |
| `--secret` | `-s` | `whsec_test_secret` | Webhook signing secret |
| `--verbose` | `-v` | `false` | Enable verbose output |

### Send Command Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Show payload without sending |
| `--payment-id` | (generated) | Payment/transaction ID |
| `--charge-id` | (generated) | Charge ID (Stripe) |
| `--amount` | `10000` | Amount in smallest currency unit |
| `--currency` | `usd` | Currency code |
| `--decline-code` | (none) | Decline code for failed payments |
| `--skip-signature` | `false` | Omit signature header |
| `--invalid-key` | `false` | Use invalid signing key |

---

## Supported Events

### Stripe Webhooks

| Event | Description |
|-------|-------------|
| `payment_intent.succeeded` | Payment completed successfully |
| `payment_intent.payment_failed` | Payment failed |
| `charge.captured` | Charge captured |
| `charge.failed` | Charge failed |
| `charge.refunded` | Charge refunded |
| `charge.dispute.created` | Dispute opened |
| `charge.dispute.closed` | Dispute resolved |

### Adyen Webhooks

| Event | Description |
|-------|-------------|
| `AUTHORISATION` | Payment authorized |
| `CAPTURE` | Payment captured |
| `CAPTURE_FAILED` | Capture failed |
| `REFUND` | Payment refunded |
| `REFUND_FAILED` | Refund failed |
| `CHARGEBACK` | Chargeback received |
| `CHARGEBACK_REVERSED` | Chargeback reversed |
| `CANCELLATION` | Payment cancelled |

### PayPal Webhooks

| Event | Description |
|-------|-------------|
| `PAYMENT.AUTHORIZATION.CREATED` | Authorization created |
| `PAYMENT.AUTHORIZATION.VOIDED` | Authorization voided |
| `PAYMENT.CAPTURE.COMPLETED` | Capture completed |
| `PAYMENT.CAPTURE.DENIED` | Capture denied |
| `PAYMENT.CAPTURE.REFUNDED` | Capture refunded |
| `CUSTOMER.DISPUTE.CREATED` | Dispute created |
| `CUSTOMER.DISPUTE.RESOLVED` | Dispute resolved |

---

## Canonical Decline Codes

The simulator maps all test card numbers to these 18 canonical decline codes, which are then translated to provider-specific codes.

### Soft Declines (Retry Eligible)

| Canonical | Stripe | Adyen | PayPal |
|-----------|--------|-------|--------|
| INSUFFICIENT_FUNDS | `insufficient_funds` | `Refused:51` | `INSTRUMENT_DECLINED` |
| OVER_LIMIT | `card_declined` | `Refused:61` | `INSTRUMENT_DECLINED` |
| GENERIC_DECLINE | `card_declined` | `Refused:05` | `INSTRUMENT_DECLINED` |
| DO_NOT_HONOR | `do_not_honor` | `Refused:05` | `INSTRUMENT_DECLINED` |
| TRY_AGAIN_LATER | `try_again_later` | `Refused:96` | `INTERNAL_SERVICE_ERROR` |
| PROCESSOR_ERROR | `processing_error` | `Refused:96` | `INTERNAL_SERVICE_ERROR` |

### Hard Declines (No Retry)

| Canonical | Stripe | Adyen | PayPal |
|-----------|--------|-------|--------|
| CARD_EXPIRED | `expired_card` | `Refused:54` | `CREDIT_CARD_EXPIRED` |
| INVALID_CARD_NUMBER | `invalid_number` | `Refused:14` | `INVALID_CARD_NUMBER` |
| INVALID_CVC | `incorrect_cvc` | `CVC Declined` | `INVALID_SECURITY_CODE` |
| INVALID_EXPIRY | `invalid_expiry_month` | `Refused:80` | `CARD_EXPIRED` |
| INVALID_ACCOUNT | `invalid_account` | `Refused:12` | `INVALID_ACCOUNT` |
| ACCOUNT_CLOSED | `card_declined` | `Refused:46` | `ACCOUNT_CLOSED` |
| ACCOUNT_RESTRICTED | `card_declined` | `Refused:36` | `ACCOUNT_RESTRICTED` |
| NOT_PERMITTED | `card_declined` | `Refused:57` | `TRANSACTION_REFUSED` |

### Fraud Declines (No Retry)

| Canonical | Stripe | Adyen | PayPal |
|-----------|--------|-------|--------|
| FRAUD_SUSPICION | `fraudulent` | `Refused:59` | `TRANSACTION_REFUSED` |
| STOLEN_CARD | `stolen_card` | `Refused:43` | `CARD_STOLEN` |
| LOST_CARD | `lost_card` | `Refused:41` | `CARD_LOST` |
| PICKUP_CARD | `pickup_card` | `Refused:04` | `CARD_RESTRICTED` |

---

## Scenario File Format

```yaml
name: "Scenario File Name"
target: "http://localhost:8080"

scenarios:
  - name: "scenario_name"
    provider: "stripe"
    steps:
      - event: "payment_intent.succeeded"
        delay: "500ms"
        data:
          payment_id: "pi_test_001"
          amount: 10000
          currency: "usd"
        expect_status: 200
        sign_opts:
          skip_signature: false
          invalid_key: false
          timestamp_offset: "-5m"
```

## Pre-built Scenarios

| File | Description |
|------|-------------|
| `scenarios/auth_capture.yaml` | Happy path: authorization followed by capture |
| `scenarios/decline_soft.yaml` | Soft decline scenarios (retry eligible) |
| `scenarios/decline_hard.yaml` | Hard decline scenarios (no retry) |
| `scenarios/decline_fraud.yaml` | Fraud flags and dispute scenarios |
| `scenarios/invalid.yaml` | Invalid signatures and payloads |

---

## Docker

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o provider-simulator .

FROM alpine:latest
COPY --from=builder /app/provider-simulator /provider-simulator
EXPOSE 9000
CMD ["/provider-simulator", "server"]
```

```bash
# Build and run
docker build -t provider-simulator .
docker run -p 9000:9000 provider-simulator
```

---

## Integration with Payment Service

1. Start the provider simulator in server mode
2. Configure your payment service to use the simulator URLs
3. Run integration tests against the simulator

```bash
# Start simulator
./provider-simulator server --port 9000

# Configure your service
export STRIPE_API_URL=http://localhost:9000/stripe
export ADYEN_API_URL=http://localhost:9000/adyen
export PAYPAL_API_URL=http://localhost:9000/paypal

# Run tests
go test ./...
```

The simulator automatically sends webhooks to your webhook handlers when payment state changes, allowing full end-to-end testing.
