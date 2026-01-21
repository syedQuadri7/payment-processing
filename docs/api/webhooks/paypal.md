# PayPal Webhooks

Receiving and processing PayPal webhook events.

## Endpoint

POST `/webhooks/paypal`

## Authentication

PayPal uses a webhook ID verification system that requires calling the PayPal API to verify each webhook.

### Verification Headers

| Header | Description |
|--------|-------------|
| `PAYPAL-TRANSMISSION-ID` | Unique transmission ID |
| `PAYPAL-TRANSMISSION-TIME` | Transmission timestamp |
| `PAYPAL-TRANSMISSION-SIG` | Signature |
| `PAYPAL-CERT-URL` | Certificate URL for signature verification |
| `PAYPAL-AUTH-ALGO` | Algorithm used (e.g., SHA256withRSA) |

### Verification Requirements

1. Extract all verification headers from the request
2. Call PayPal's Webhook Verification API with:
   - Configured webhook ID
   - All header values
   - Raw webhook event body
3. PayPal returns `verification_status`: `SUCCESS` or `FAILURE`
4. Reject with 401 if verification fails

### Configuration

| Variable | Source | Purpose |
|----------|--------|---------|
| `PAYPAL_CLIENT_ID` | PayPal Developer Dashboard | API authentication |
| `PAYPAL_CLIENT_SECRET` | PayPal Developer Dashboard | API authentication |
| `PAYPAL_WEBHOOK_ID` | PayPal Developer Dashboard | Webhook verification |
| `PAYPAL_API_URL` | Configuration | `api-m.sandbox.paypal.com` or `api-m.paypal.com` |

---

## Event Mapping

PayPal events must be normalized to canonical events.

| PayPal Event | Canonical Event |
|--------------|-----------------|
| `PAYMENT.AUTHORIZATION.CREATED` | `AUTHORIZATION_SUCCEEDED` |
| `PAYMENT.AUTHORIZATION.VOIDED` | `VOID_SUCCEEDED` |
| `PAYMENT.CAPTURE.COMPLETED` | `CAPTURE_SUCCEEDED` |
| `PAYMENT.CAPTURE.DENIED` | `CAPTURE_FAILED` |
| `PAYMENT.CAPTURE.PENDING` | (log only, no state change) |
| `PAYMENT.CAPTURE.REFUNDED` | `REFUND_SUCCEEDED` |
| `CUSTOMER.DISPUTE.CREATED` | `DISPUTE_OPENED` |
| `CUSTOMER.DISPUTE.RESOLVED` | `DISPUTE_WON` or `DISPUTE_LOST` |

---

## Decline Code Mapping

### Soft Declines (Retry Eligible)

| PayPal Code | Canonical Code |
|-------------|----------------|
| `INSUFFICIENT_FUNDS` | `INSUFFICIENT_FUNDS` |
| `INSTRUMENT_DECLINED` | `GENERIC_DECLINE` |
| `DO_NOT_HONOR` | `DO_NOT_HONOR` |
| `PAYER_ACTION_REQUIRED` | `TRY_AGAIN` |
| `INTERNAL_SERVICE_ERROR` | `PROCESSING_ERROR` |

### Hard Declines (Not Retry Eligible)

| PayPal Code | Canonical Code |
|-------------|----------------|
| `CREDIT_CARD_EXPIRED` | `CARD_EXPIRED` |
| `INVALID_ACCOUNT` | `ACCOUNT_CLOSED` |
| `CARD_TYPE_NOT_SUPPORTED` | `CARD_RESTRICTED` |
| `CREDIT_CARD_CVV_CHECK_FAILED` | `INVALID_CVV` |

### Fraud Declines

| PayPal Code | Canonical Code |
|-------------|----------------|
| `TRANSACTION_REFUSED` | `FRAUD_SUSPICION` |

---

## Response Requirements

### Success

Return HTTP 200 with empty body. Event will be marked as delivered.

### Error Handling

| HTTP Status | Meaning | PayPal Behavior |
|-------------|---------|-----------------|
| 200 | Success | Event marked delivered |
| 401 | Invalid signature | Event marked failed |
| 503 | Service unavailable | PayPal will retry |
| 5xx | Server error | PayPal will retry |

### Retry Behavior

PayPal retries failed webhooks for up to 3 days with exponential backoff.

---

## Idempotency Requirements

The adapter must handle duplicate webhooks:

1. Use the webhook `id` field as the unique identifier
2. Return 200 for already-processed events (do not reprocess)
3. Track processed webhook IDs

---

## Key Data Fields

| Field Path | Purpose |
|------------|---------|
| `id` | Webhook event ID for idempotency |
| `event_type` | Event type for routing |
| `resource.id` | PayPal resource ID (authorization, capture, etc.) |
| `resource.invoice_id` | Our internal payment intent ID |
| `resource.custom_id` | Additional reference (e.g., order ID) |
| `resource.amount.total` | Amount as decimal string |
| `resource.amount.currency` | Currency code |
| `resource.status_details.reason` | Decline reason (if failed) |

---

## Provider-Specific Considerations

### Invoice ID Mapping

When creating PayPal payments:
- Set `invoice_id` to our internal payment intent ID
- Set `custom_id` for additional references (e.g., order ID)

These fields enable correlation when receiving webhooks.

### Authorization Expiration

PayPal authorizations have specific expiration rules:
- Default: 3 days
- Honor period: Can extend up to 29 days
- The `expiration_time` field in the resource indicates when authorization expires

### Multiple Captures

PayPal supports multiple partial captures on a single authorization:
- Track the `final_capture` boolean in capture webhooks
- When `final_capture` is true, the authorization is fully consumed
- Remaining authorized amount is automatically released

### Amount Format

PayPal amounts are decimal strings (e.g., "100.00"), not integer smallest units. Convert appropriately when normalizing to canonical format.

---

## Edge Cases

### Unknown Payment ID (invoice_id)

When a webhook arrives for an invoice_id not in our database:

| Cause | Handling |
|-------|----------|
| Payment created directly in PayPal | Log warning, return 200 |
| invoice_id not set during payment creation | Log warning, return 200 |
| Bug in payment creation flow | Log warning, return 200 |

**Rationale:** Returning an error causes PayPal to retry for up to 3 days. Signature verification ensures it's a real PayPal event.

### Duplicate Webhooks

PayPal may send the same webhook multiple times. Handle idempotently:

1. Track processed webhooks using the `id` field
2. On duplicate: return 200 immediately without reprocessing
3. The `id` is unique per webhook event

### Verification Latency

PayPal webhook verification requires an API call to PayPal. This adds latency:

| Concern | Handling |
|---------|----------|
| Verification timeout | Return 503 (PayPal will retry) |
| Verification failure | Return 401 (check credentials) |
| Slow verification | Consider async processing |

**Timeout consideration:** PayPal expects a response within 30 seconds. If verification takes too long, consider:
1. Verifying asynchronously and acknowledging immediately
2. Caching verification results briefly
3. Using a dedicated verification service

For this learning project, synchronous verification is acceptable.

### Late-Arriving Webhooks

When a webhook describes a state the payment has already passed:

| Example | Handling |
|---------|----------|
| AUTHORIZATION.CREATED after CAPTURE.COMPLETED | Log and ignore |
| CAPTURE.PENDING after CAPTURE.COMPLETED | Log and ignore |

### CAPTURE.PENDING Events

PayPal sends `PAYMENT.CAPTURE.PENDING` when a capture is pending review. Handling:

- Do NOT update payment status to "captured" yet
- Log the pending status
- Wait for `PAYMENT.CAPTURE.COMPLETED` or `PAYMENT.CAPTURE.DENIED`

### Dispute Resolution

`CUSTOMER.DISPUTE.RESOLVED` requires checking the resolution to determine outcome:

```go
switch resource.Status {
case "RESOLVED_BUYER_FAVOUR":
    return EventDisputeLost
case "RESOLVED_SELLER_FAVOUR":
    return EventDisputeWon
default:
    log.Warn("unknown dispute resolution", "status", resource.Status)
    return EventUnknown
}
```

### Multiple Captures

PayPal supports multiple partial captures. Track the `final_capture` boolean:

| final_capture | Meaning |
|---------------|---------|
| false | More captures may follow |
| true | Authorization fully consumed |

When `final_capture` is true, any remaining authorized amount is automatically released.

### Sandbox vs Production URLs

Ensure webhook verification uses the correct PayPal API URL:

| Environment | API URL |
|-------------|---------|
| Sandbox | `api-m.sandbox.paypal.com` |
| Production | `api-m.paypal.com` |

Misconfigured URLs will cause verification failures.
