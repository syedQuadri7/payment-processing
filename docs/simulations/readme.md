# Simulations

This directory documents testing tools and simulation capabilities for the payment processing service.

## Overview

Testing payment systems requires simulating various webhook scenarios from payment providers. The project includes a webhook simulator tool that generates provider-specific webhook payloads with proper signatures.

## Webhook Simulator

The primary testing tool is located at `tools/webhook-simulator/`. See the full documentation in [tools/webhook-simulator/README.md](../../tools/webhook-simulator/README.md).

### Quick Start

```bash
# Build the simulator
cd tools/webhook-simulator
go build -o webhook-simulator .

# Send a test webhook
./webhook-simulator send stripe payment_intent.succeeded --amount 10000
```

### Supported Providers

| Provider | Endpoint | Signature Method |
|----------|----------|------------------|
| Stripe | `/webhooks/stripe` | HMAC-SHA256 with timestamp |
| Adyen | `/webhooks/adyen` | HMAC-SHA256 |
| PayPal | `/webhooks/paypal` | Mock headers (API verification mocked) |

### Common Commands

```bash
# Stripe successful authorization
./webhook-simulator send stripe payment_intent.succeeded --amount 10000

# Stripe decline with insufficient funds
./webhook-simulator send stripe payment_intent.payment_failed --decline-code insufficient_funds

# Adyen authorization
./webhook-simulator send adyen AUTHORISATION --amount 5000

# PayPal capture
./webhook-simulator send paypal PAYMENT.CAPTURE.COMPLETED --amount 7500

# Test invalid signature handling
./webhook-simulator send stripe payment_intent.succeeded --invalid-key
```

## Test Scenarios

Pre-built scenario files are in `tools/webhook-simulator/scenarios/`:

| Scenario | File | Description |
|----------|------|-------------|
| Happy Path | `auth_capture.yaml` | Authorization followed by capture |
| Soft Declines | `decline_soft.yaml` | Retry-eligible declines |
| Hard Declines | `decline_hard.yaml` | Non-retryable declines |
| Fraud | `decline_fraud.yaml` | Fraud flags and disputes |
| Invalid | `invalid.yaml` | Invalid signatures/payloads |

### Running Scenarios

```bash
# Run full auth/capture flow
./webhook-simulator run scenarios/auth_capture.yaml

# Run with verbose output
./webhook-simulator run scenarios/decline_soft.yaml -v

# Stop on first error
./webhook-simulator run scenarios/invalid.yaml --stop-on-error
```

## Testing Strategy

### 1. Unit Testing Adapters

Test each adapter's event mapping and signature verification in isolation:

```bash
go test ./internal/adapter/stripe/... -v
go test ./internal/adapter/adyen/... -v
go test ./internal/adapter/paypal/... -v
```

### 2. Integration Testing with Simulator

Use the webhook simulator against a running service:

```bash
# Terminal 1: Start the service
./payment-processing

# Terminal 2: Run webhook scenarios
cd tools/webhook-simulator
./webhook-simulator run scenarios/auth_capture.yaml
```

### 3. End-to-End Testing

Full payment flow testing:

1. Create a payment intent via API
2. Simulate provider authorization webhook
3. Capture via API
4. Simulate capture confirmation webhook
5. Verify ledger entries and events

### 4. Negative Testing

Test error handling:

```bash
# Missing signature
./webhook-simulator send stripe payment_intent.succeeded --skip-signature

# Invalid signature
./webhook-simulator send stripe payment_intent.succeeded --invalid-key

# Expired timestamp (replay attack protection)
# Use scenario file with timestamp_offset
```

## Decline Code Testing

### Soft Declines (Retry Eligible)

| Test Case | Command |
|-----------|---------|
| Insufficient funds | `send stripe payment_intent.payment_failed --decline-code insufficient_funds` |
| Generic decline | `send stripe payment_intent.payment_failed --decline-code card_declined` |
| Processing error | `send stripe payment_intent.payment_failed --decline-code processing_error` |

### Hard Declines (No Retry)

| Test Case | Command |
|-----------|---------|
| Expired card | `send stripe payment_intent.payment_failed --decline-code expired_card` |
| Invalid number | `send stripe payment_intent.payment_failed --decline-code incorrect_number` |
| Invalid CVV | `send stripe payment_intent.payment_failed --decline-code incorrect_cvc` |

### Fraud Declines

| Test Case | Command |
|-----------|---------|
| Fraud suspicion | `send stripe payment_intent.payment_failed --decline-code fraudulent` |
| Stolen card | `send stripe payment_intent.payment_failed --decline-code stolen_card` |
| Lost card | `send stripe payment_intent.payment_failed --decline-code lost_card` |

## Creating Custom Scenarios

Create YAML scenario files for complex test flows:

```yaml
name: "Custom Payment Flow"
target: "http://localhost:8080"

scenarios:
  - name: "authorization_then_capture"
    provider: "stripe"
    steps:
      # Step 1: Authorization succeeds
      - event: "payment_intent.succeeded"
        delay: "0ms"
        data:
          payment_id: "pi_test_custom"
          amount: 25000
          currency: "usd"
        expect_status: 200

      # Step 2: Capture succeeds after delay
      - event: "charge.captured"
        delay: "500ms"
        data:
          payment_id: "pi_test_custom"
          charge_id: "ch_test_custom"
          amount: 25000
        expect_status: 200
```

## Monitoring During Tests

While running simulations, monitor:

1. **Temporal UI**: Watch workflow executions at `http://localhost:8233`
2. **Service Logs**: Check for signature verification and event processing
3. **Database**: Verify payment states and ledger entries
4. **Kafka**: Confirm CDC events are published

## CI/CD Integration

The webhook simulator can be used in automated testing:

```bash
#!/bin/bash
# ci-test.sh

# Start service in background
./payment-processing &
SERVICE_PID=$!
sleep 5

# Run test scenarios
cd tools/webhook-simulator
./webhook-simulator run scenarios/auth_capture.yaml --stop-on-error
TEST_RESULT=$?

# Cleanup
kill $SERVICE_PID

exit $TEST_RESULT
```

## Related Documentation

- [Stripe Webhooks API](../api/webhooks/stripe.md)
- [Adyen Webhooks API](../api/webhooks/adyen.md)
- [PayPal Webhooks API](../api/webhooks/paypal.md)
- [Decline Code Reference](../_reference.md#canonical-decline-codes)
