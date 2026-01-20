# Stripe Webhooks

Documentation for receiving and processing Stripe webhook events.

## Endpoint

```
POST /webhooks/stripe
```

## Authentication

Stripe signs webhook payloads using HMAC-SHA256. The signature is included in the `Stripe-Signature` header.

### Header Format

```
Stripe-Signature: t=1492774577,v1=5257a869e7ecebeda32affa62cdca3fa51cad7e77a0e56ff536d0ce8e108d8bd,v0=...
```

| Component | Description |
|-----------|-------------|
| `t` | Timestamp (Unix seconds) |
| `v1` | HMAC-SHA256 signature |
| `v0` | Legacy signature (deprecated) |

### Verification Process

1. Extract timestamp and signature from header
2. Reject if timestamp is more than 5 minutes old (replay protection)
3. Construct signed payload: `{timestamp}.{raw_body}`
4. Compute HMAC-SHA256 using webhook secret
5. Compare computed signature with provided `v1` signature

### Configuration

```bash
STRIPE_WEBHOOK_SECRET=whsec_... # From Stripe Dashboard > Webhooks
```

## Event Mapping

Stripe events are normalized to canonical events before processing.

| Stripe Event | Canonical Event | Notes |
|--------------|-----------------|-------|
| `payment_intent.succeeded` | `AUTHORIZATION_SUCCEEDED` | When not yet captured |
| `payment_intent.payment_failed` | `AUTHORIZATION_FAILED` | |
| `payment_intent.canceled` | `VOID_SUCCEEDED` | |
| `charge.captured` | `CAPTURE_SUCCEEDED` | |
| `charge.failed` | `CAPTURE_FAILED` | |
| `charge.refunded` | `REFUND_SUCCEEDED` | |
| `charge.refund.updated` | - | Log only |
| `charge.dispute.created` | `DISPUTE_OPENED` | |
| `charge.dispute.closed` | `DISPUTE_WON` or `DISPUTE_LOST` | Based on status |

## Decline Code Mapping

| Stripe Code | Canonical Code | Decline Type |
|-------------|----------------|--------------|
| `insufficient_funds` | `INSUFFICIENT_FUNDS` | Soft |
| `card_declined` | `GENERIC_DECLINE` | Soft |
| `do_not_honor` | `DO_NOT_HONOR` | Soft |
| `try_again_later` | `TRY_AGAIN` | Soft |
| `processing_error` | `PROCESSING_ERROR` | Soft |
| `expired_card` | `CARD_EXPIRED` | Hard |
| `incorrect_cvc` | `INVALID_CVV` | Hard |
| `invalid_number` | `INVALID_NUMBER` | Hard |
| `card_not_supported` | `CARD_RESTRICTED` | Hard |
| `fraudulent` | `FRAUD_SUSPICION` | Fraud |
| `lost_card` | `LOST_CARD` | Fraud |
| `stolen_card` | `STOLEN_CARD` | Fraud |

## Example Webhook Payloads

### payment_intent.succeeded

```json
{
  "id": "evt_1234567890",
  "object": "event",
  "api_version": "2023-10-16",
  "created": 1705487400,
  "type": "payment_intent.succeeded",
  "data": {
    "object": {
      "id": "pi_xyz789",
      "object": "payment_intent",
      "amount": 10000,
      "currency": "usd",
      "status": "succeeded",
      "customer": "cus_abc123",
      "payment_method": "pm_456",
      "metadata": {
        "internal_id": "pi_abc123"
      },
      "created": 1705487350
    }
  }
}
```

### payment_intent.payment_failed

```json
{
  "id": "evt_0987654321",
  "object": "event",
  "type": "payment_intent.payment_failed",
  "data": {
    "object": {
      "id": "pi_xyz789",
      "object": "payment_intent",
      "amount": 10000,
      "currency": "usd",
      "status": "requires_payment_method",
      "last_payment_error": {
        "code": "insufficient_funds",
        "decline_code": "insufficient_funds",
        "message": "Your card has insufficient funds.",
        "type": "card_error"
      },
      "metadata": {
        "internal_id": "pi_abc123"
      }
    }
  }
}
```

### charge.captured

```json
{
  "id": "evt_capture123",
  "object": "event",
  "type": "charge.captured",
  "data": {
    "object": {
      "id": "ch_abc123",
      "object": "charge",
      "amount": 10000,
      "amount_captured": 10000,
      "captured": true,
      "currency": "usd",
      "payment_intent": "pi_xyz789",
      "status": "succeeded",
      "metadata": {
        "internal_id": "pi_abc123"
      }
    }
  }
}
```

### charge.dispute.created

```json
{
  "id": "evt_dispute123",
  "object": "event",
  "type": "charge.dispute.created",
  "data": {
    "object": {
      "id": "dp_abc123",
      "object": "dispute",
      "amount": 10000,
      "charge": "ch_abc123",
      "currency": "usd",
      "reason": "fraudulent",
      "status": "needs_response",
      "created": 1705487400
    }
  }
}
```

## Response Handling

### Success Response

Return `200 OK` with empty body or acknowledgment:

```json
{
  "received": true
}
```

### Error Responses

| Status | Meaning | Stripe Behavior |
|--------|---------|-----------------|
| `200` | Success | Event marked delivered |
| `401` | Invalid signature | Event marked failed, no retry |
| `4xx` | Client error | Event marked failed, limited retries |
| `5xx` | Server error | Stripe will retry with exponential backoff |

**Retry Schedule:** Stripe retries up to 3 days with exponential backoff.

## Idempotency

Stripe may send the same event multiple times. The adapter must handle duplicates:

```go
func (a *StripeAdapter) HandleWebhook(ctx context.Context, event stripe.Event) error {
    // Check if already processed
    if a.repo.EventProcessed(ctx, event.ID) {
        return nil // Return 200, already handled
    }

    // Process event...

    // Mark as processed
    a.repo.MarkEventProcessed(ctx, event.ID, time.Now())
    return nil
}
```

## Testing

### Stripe CLI

Use Stripe CLI to forward test webhooks:

```bash
# Install Stripe CLI
brew install stripe/stripe-cli/stripe

# Login
stripe login

# Forward webhooks to local endpoint
stripe listen --forward-to localhost:8080/webhooks/stripe

# Trigger test events
stripe trigger payment_intent.succeeded
stripe trigger payment_intent.payment_failed
stripe trigger charge.captured
stripe trigger charge.dispute.created
```

### Webhook Simulator

Use the local webhook simulator for comprehensive testing:

```bash
# Start simulator
go run cmd/simulator/main.go --provider stripe

# Send test webhook
curl -X POST http://localhost:8081/simulate/stripe/payment_intent.succeeded \
  -d '{"payment_intent_id": "pi_abc123", "amount": 10000}'
```

See [Simulations Documentation](../../simulations/readme.md) for details.
