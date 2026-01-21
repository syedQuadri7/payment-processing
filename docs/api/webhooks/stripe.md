# Stripe Webhooks

Receiving and processing Stripe webhook events.

## Endpoint

POST `/webhooks/stripe`

## Authentication

Stripe signs webhook payloads using HMAC-SHA256. The signature is included in the `Stripe-Signature` header.

### Signature Header Components

| Component | Description |
|-----------|-------------|
| `t` | Timestamp (Unix seconds) |
| `v1` | HMAC-SHA256 signature |
| `v0` | Legacy signature (deprecated, ignore) |

### Verification Requirements

1. Extract timestamp and signature from header
2. Reject if timestamp is more than 5 minutes old (replay protection)
3. Construct signed payload as: `{timestamp}.{raw_body}`
4. Compute HMAC-SHA256 using webhook secret
5. Compare signatures using constant-time comparison

### Configuration

| Variable | Source | Purpose |
|----------|--------|---------|
| `STRIPE_WEBHOOK_SECRET` | Stripe Dashboard > Webhooks | Signing secret for HMAC verification |

---

## Event Mapping

Stripe events must be normalized to canonical events before processing.

| Stripe Event | Canonical Event | Notes |
|--------------|-----------------|-------|
| `payment_intent.succeeded` | `AUTHORIZATION_SUCCEEDED` | When not yet captured |
| `payment_intent.payment_failed` | `AUTHORIZATION_FAILED` | |
| `payment_intent.canceled` | `VOID_SUCCEEDED` | |
| `charge.captured` | `CAPTURE_SUCCEEDED` | |
| `charge.failed` | `CAPTURE_FAILED` | |
| `charge.refunded` | `REFUND_SUCCEEDED` | |
| `charge.refund.updated` | (log only) | No state change |
| `charge.dispute.created` | `DISPUTE_OPENED` | |
| `charge.dispute.closed` | `DISPUTE_WON` or `DISPUTE_LOST` | Based on dispute status |

---

## Decline Code Mapping

Stripe decline codes must be mapped to canonical codes for consistent handling.

### Soft Declines (Retry Eligible)

| Stripe Code | Canonical Code |
|-------------|----------------|
| `insufficient_funds` | `INSUFFICIENT_FUNDS` |
| `card_declined` | `GENERIC_DECLINE` |
| `do_not_honor` | `DO_NOT_HONOR` |
| `try_again_later` | `TRY_AGAIN` |
| `processing_error` | `PROCESSING_ERROR` |

### Hard Declines (Not Retry Eligible)

| Stripe Code | Canonical Code |
|-------------|----------------|
| `expired_card` | `CARD_EXPIRED` |
| `incorrect_cvc` | `INVALID_CVV` |
| `invalid_number` | `INVALID_NUMBER` |
| `card_not_supported` | `CARD_RESTRICTED` |

### Fraud Declines

| Stripe Code | Canonical Code |
|-------------|----------------|
| `fraudulent` | `FRAUD_SUSPICION` |
| `lost_card` | `LOST_CARD` |
| `stolen_card` | `STOLEN_CARD` |

---

## Response Requirements

### Success

Return HTTP 200 with empty body or acknowledgment. Event will be marked as delivered.

### Error Handling

| HTTP Status | Meaning | Stripe Behavior |
|-------------|---------|-----------------|
| 200 | Success | Event marked delivered |
| 401 | Invalid signature | Event marked failed, no retry |
| 4xx | Client error | Event marked failed, limited retries |
| 5xx | Server error | Stripe retries with exponential backoff |

### Retry Behavior

Stripe retries failed webhooks for up to 3 days with exponential backoff.

---

## Idempotency Requirements

Stripe may send the same event multiple times. The adapter must:

1. Track processed event IDs
2. Return 200 for already-processed events (do not reprocess)
3. Use the event `id` field as the unique identifier

---

## Key Data Fields

When processing Stripe webhooks, extract these fields:

| Field Path | Purpose |
|------------|---------|
| `id` | Event ID for idempotency |
| `type` | Event type for routing |
| `data.object.id` | Payment intent or charge ID |
| `data.object.metadata.internal_id` | Our internal payment intent ID |
| `data.object.amount` | Amount in smallest currency unit |
| `data.object.currency` | Currency code |
| `data.object.last_payment_error.decline_code` | Decline code (if failed) |

---

## Provider-Specific Considerations

### Metadata Correlation

Our internal payment intent ID should be stored in Stripe's metadata field when creating the payment. This enables correlation when receiving webhooks.

### Amount Format

Stripe amounts are in the smallest currency unit (e.g., cents for USD). Convert appropriately when normalizing to canonical format.

### Test vs Live Mode

The webhook endpoint receives both test and live events. Use the `livemode` field to distinguish if needed.

---

## Edge Cases

### Unknown Payment ID

When a webhook arrives for a payment ID not in our database:

| Cause | Handling |
|-------|----------|
| Payment created directly in Stripe Dashboard | Log warning, return 200 |
| Bug in payment creation flow | Log warning, return 200 |
| Replication lag (rare) | Log warning, return 200 |

**Rationale:** Returning an error causes Stripe to retry indefinitely. Unknown payments won't become known by retrying. Signature verification ensures it's a real Stripe event. Log provides visibility for investigation.

```go
if err == ErrPaymentNotFound {
    log.Warn("webhook for unknown payment", "stripe_id", paymentIntentID)
    return 200  // Acknowledge to prevent retries
}
```

### Duplicate Webhooks

Stripe may send the same event multiple times. Handle idempotently:

1. Track processed event IDs in database
2. On duplicate: return 200 immediately without reprocessing
3. Use Stripe's `event.id` as the unique identifier

### Late-Arriving Webhooks

When a webhook describes a state the payment has already passed:

| Example | Handling |
|---------|----------|
| Auth success arrives after capture complete | Log and ignore (already advanced) |
| Payment failed arrives after manual resolution | Log and ignore |

The workflow's current state is authoritative. Late webhooks are informational only.

### Event Ordering

Stripe does not guarantee event ordering. Events may arrive out of order:

- `charge.captured` may arrive before `payment_intent.succeeded`
- Handle each event based on current state, not assumed sequence
- Idempotency ensures duplicate processing is safe

### Rate Limiting

Stripe sends webhooks in bursts during high-volume periods. Our approach:

| Layer | Handling |
|-------|----------|
| Reverse proxy | Defer to nginx/load balancer rate limiting |
| Application | Process all valid webhooks (return 200) |
| Workflow | Temporal handles concurrent signals |

For this learning project, we don't implement application-level rate limiting for incoming webhooks. Production systems typically handle this at the infrastructure layer.
