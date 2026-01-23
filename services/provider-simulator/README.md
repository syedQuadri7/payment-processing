# Webhook Simulator

A CLI tool that generates and sends provider-specific webhook payloads to test the payment processing service's adapter layer. Supports Stripe, Adyen, and PayPal with proper signature generation and configurable test scenarios.

## Installation

```bash
cd tools/webhook-simulator
go build -o webhook-simulator .
```

## Usage

### Send a Single Webhook

```bash
# Send a successful Stripe payment intent
./webhook-simulator send stripe payment_intent.succeeded --amount 10000

# Send a failed payment with decline code
./webhook-simulator send stripe payment_intent.payment_failed --decline-code insufficient_funds

# Dry run to see payload without sending
./webhook-simulator send stripe charge.captured --dry-run

# Send Adyen authorization
./webhook-simulator send adyen AUTHORISATION --amount 5000

# Send PayPal capture
./webhook-simulator send paypal PAYMENT.CAPTURE.COMPLETED --amount 7500

# Test invalid signatures
./webhook-simulator send stripe payment_intent.succeeded --invalid-key
./webhook-simulator send stripe payment_intent.succeeded --skip-signature
```

### Run Scenario Files

```bash
# Run authorization and capture scenario
./webhook-simulator run scenarios/auth_capture.yaml

# Run with custom target
./webhook-simulator run scenarios/decline_soft.yaml --target http://localhost:9090

# Stop on first error
./webhook-simulator run scenarios/invalid.yaml --stop-on-error
```

## Global Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--target` | `-t` | `http://localhost:8080` | Target URL for webhook delivery |
| `--secret` | `-s` | `whsec_test_secret` | Webhook signing secret |
| `--verbose` | `-v` | `false` | Enable verbose output |

## Send Command Flags

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

## Supported Providers and Events

### Stripe

Endpoint: `/webhooks/stripe`

| Event | Description |
|-------|-------------|
| `payment_intent.succeeded` | Payment completed successfully |
| `payment_intent.payment_failed` | Payment failed |
| `charge.captured` | Charge captured |
| `charge.failed` | Charge failed |
| `charge.refunded` | Charge refunded |
| `charge.dispute.created` | Dispute opened |
| `charge.dispute.closed` | Dispute resolved |

Signature: HMAC-SHA256 with timestamp (`Stripe-Signature: t={timestamp},v1={hmac}`)

### Adyen

Endpoint: `/webhooks/adyen`

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

Signature: HMAC-SHA256 (`X-Adyen-Hmac-Signature` header)

### PayPal

Endpoint: `/webhooks/paypal`

| Event | Description |
|-------|-------------|
| `PAYMENT.AUTHORIZATION.CREATED` | Authorization created |
| `PAYMENT.AUTHORIZATION.VOIDED` | Authorization voided |
| `PAYMENT.CAPTURE.COMPLETED` | Capture completed |
| `PAYMENT.CAPTURE.DENIED` | Capture denied |
| `PAYMENT.CAPTURE.REFUNDED` | Capture refunded |
| `CUSTOMER.DISPUTE.CREATED` | Dispute created |
| `CUSTOMER.DISPUTE.RESOLVED` | Dispute resolved |

Signature: Mock headers for testing (real PayPal verification requires API calls)

## Decline Codes

### Soft Declines (Retry Eligible)

| Provider | Code | Canonical |
|----------|------|-----------|
| Stripe | `insufficient_funds` | INSUFFICIENT_FUNDS |
| Stripe | `card_declined` | GENERIC_DECLINE |
| Stripe | `processing_error` | PROCESSING_ERROR |
| Adyen | `Refused:51` | INSUFFICIENT_FUNDS |
| Adyen | `Refused:05` | DO_NOT_HONOR |
| PayPal | `INSUFFICIENT_FUNDS` | INSUFFICIENT_FUNDS |
| PayPal | `PAYMENT_DENIED` | GENERIC_DECLINE |

### Hard Declines (No Retry)

| Provider | Code | Canonical |
|----------|------|-----------|
| Stripe | `expired_card` | CARD_EXPIRED |
| Adyen | `Refused:33` | CARD_EXPIRED |
| Adyen | `Refused:14` | INVALID_CARD_NUMBER |
| PayPal | `CREDIT_CARD_EXPIRED` | CARD_EXPIRED |

### Fraud Declines

| Provider | Code | Canonical |
|----------|------|-----------|
| Stripe | `fraudulent` | FRAUD_SUSPICION |
| Adyen | `Refused:59` | FRAUD_SUSPICION |
| PayPal | `TRANSACTION_REFUSED` | FRAUD_SUSPICION |

## Scenario File Format

Scenario files are YAML documents that define multiple test scenarios:

```yaml
name: "Scenario File Name"
target: "http://localhost:8080"

scenarios:
  - name: "scenario_name"
    provider: "stripe"  # stripe, adyen, or paypal
    steps:
      - event: "payment_intent.succeeded"
        delay: "500ms"  # Optional delay before this step
        data:
          payment_id: "pi_test_001"
          amount: 10000
          currency: "usd"
        expect_status: 200
        sign_opts:  # Optional signature manipulation
          skip_signature: false
          invalid_key: false
          timestamp_offset: "-5m"  # Negative for past
```

## Pre-built Scenarios

| File | Description |
|------|-------------|
| `scenarios/auth_capture.yaml` | Happy path: authorization followed by capture |
| `scenarios/decline_soft.yaml` | Soft decline scenarios (retry eligible) |
| `scenarios/decline_hard.yaml` | Hard decline scenarios (no retry) |
| `scenarios/decline_fraud.yaml` | Fraud flags and dispute scenarios |
| `scenarios/invalid.yaml` | Invalid signatures and payloads |

## Examples

### Testing Happy Path

```bash
# Start the payment processing service
cd /path/to/payment-processing
./payment-processing

# In another terminal, run the auth/capture scenario
cd tools/webhook-simulator
./webhook-simulator run scenarios/auth_capture.yaml
```

### Testing Decline Handling

```bash
# Test soft declines
./webhook-simulator run scenarios/decline_soft.yaml

# Test hard declines
./webhook-simulator run scenarios/decline_hard.yaml

# Test fraud scenarios
./webhook-simulator run scenarios/decline_fraud.yaml
```

### Testing Signature Verification

```bash
# Test missing/invalid signatures (expect 401 responses)
./webhook-simulator run scenarios/invalid.yaml
```

### Quick Manual Tests

```bash
# Test a single successful payment
./webhook-simulator send stripe payment_intent.succeeded -v

# Test insufficient funds decline
./webhook-simulator send stripe payment_intent.payment_failed \
  --decline-code insufficient_funds -v

# Test with custom amount and ID
./webhook-simulator send stripe charge.captured \
  --payment-id pi_custom_123 \
  --amount 50000 \
  --currency eur
```
