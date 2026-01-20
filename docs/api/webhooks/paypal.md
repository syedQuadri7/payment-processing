# PayPal Webhooks

Documentation for receiving and processing PayPal webhook events.

## Endpoint

```
POST /webhooks/paypal
```

## Authentication

PayPal uses a webhook ID verification system. Each webhook must be verified by calling the PayPal API.

### Headers

| Header | Description |
|--------|-------------|
| `PAYPAL-TRANSMISSION-ID` | Unique transmission ID |
| `PAYPAL-TRANSMISSION-TIME` | Transmission timestamp |
| `PAYPAL-TRANSMISSION-SIG` | Signature |
| `PAYPAL-CERT-URL` | Certificate URL for signature verification |
| `PAYPAL-AUTH-ALGO` | Algorithm used (e.g., `SHA256withRSA`) |

### Verification Process

1. Extract all verification headers
2. Call PayPal Webhook Verification API with:
   - Webhook ID (from configuration)
   - Transmission ID
   - Transmission time
   - Webhook event body
   - Certificate URL
   - Transmission signature
   - Auth algorithm
3. PayPal returns `verification_status`: `SUCCESS` or `FAILURE`

### Configuration

```bash
PAYPAL_CLIENT_ID=...
PAYPAL_CLIENT_SECRET=...
PAYPAL_WEBHOOK_ID=... # From PayPal Developer Dashboard
PAYPAL_API_URL=https://api-m.sandbox.paypal.com # or api-m.paypal.com for production
```

### Verification API Call

```bash
curl -X POST https://api-m.sandbox.paypal.com/v1/notifications/verify-webhook-signature \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer {access_token}" \
  -d '{
    "webhook_id": "YOUR_WEBHOOK_ID",
    "transmission_id": "...",
    "transmission_time": "...",
    "cert_url": "...",
    "auth_algo": "SHA256withRSA",
    "transmission_sig": "...",
    "webhook_event": {...}
  }'
```

## Event Mapping

PayPal events are normalized to canonical events before processing.

| PayPal Event | Canonical Event |
|--------------|-----------------|
| `PAYMENT.AUTHORIZATION.CREATED` | `AUTHORIZATION_SUCCEEDED` |
| `PAYMENT.AUTHORIZATION.VOIDED` | `VOID_SUCCEEDED` |
| `PAYMENT.CAPTURE.COMPLETED` | `CAPTURE_SUCCEEDED` |
| `PAYMENT.CAPTURE.DENIED` | `CAPTURE_FAILED` |
| `PAYMENT.CAPTURE.PENDING` | - (log only) |
| `PAYMENT.CAPTURE.REFUNDED` | `REFUND_SUCCEEDED` |
| `CUSTOMER.DISPUTE.CREATED` | `DISPUTE_OPENED` |
| `CUSTOMER.DISPUTE.RESOLVED` | `DISPUTE_WON` or `DISPUTE_LOST` |

## Decline Code Mapping

PayPal decline codes mapped to canonical codes.

| PayPal Code | Canonical Code | Decline Type |
|-------------|----------------|--------------|
| `INSUFFICIENT_FUNDS` | `INSUFFICIENT_FUNDS` | Soft |
| `INSTRUMENT_DECLINED` | `GENERIC_DECLINE` | Soft |
| `DO_NOT_HONOR` | `DO_NOT_HONOR` | Soft |
| `PAYER_ACTION_REQUIRED` | `TRY_AGAIN` | Soft |
| `INTERNAL_SERVICE_ERROR` | `PROCESSING_ERROR` | Soft |
| `CREDIT_CARD_EXPIRED` | `CARD_EXPIRED` | Hard |
| `INVALID_ACCOUNT` | `ACCOUNT_CLOSED` | Hard |
| `CARD_TYPE_NOT_SUPPORTED` | `CARD_RESTRICTED` | Hard |
| `CREDIT_CARD_CVV_CHECK_FAILED` | `INVALID_CVV` | Hard |
| `TRANSACTION_REFUSED` | `FRAUD_SUSPICION` | Fraud |

## Example Webhook Payloads

### PAYMENT.AUTHORIZATION.CREATED

```json
{
  "id": "WH-2WR32451HC0233532-67976317FL4543714",
  "event_version": "1.0",
  "create_time": "2026-01-17T10:30:00.000Z",
  "resource_type": "authorization",
  "event_type": "PAYMENT.AUTHORIZATION.CREATED",
  "summary": "A payment authorization was created",
  "resource": {
    "id": "5O190127TN364715T",
    "status": "CREATED",
    "amount": {
      "total": "100.00",
      "currency": "USD"
    },
    "invoice_id": "pi_abc123",
    "custom_id": "order_789",
    "create_time": "2026-01-17T10:30:00.000Z",
    "expiration_time": "2026-01-20T10:30:00.000Z",
    "links": [
      {
        "href": "https://api.paypal.com/v2/payments/authorizations/5O190127TN364715T",
        "rel": "self",
        "method": "GET"
      },
      {
        "href": "https://api.paypal.com/v2/payments/authorizations/5O190127TN364715T/capture",
        "rel": "capture",
        "method": "POST"
      }
    ]
  },
  "links": [
    {
      "href": "https://api.paypal.com/v1/notifications/webhooks-events/WH-2WR32451HC0233532-67976317FL4543714",
      "rel": "self",
      "method": "GET"
    }
  ]
}
```

### PAYMENT.CAPTURE.COMPLETED

```json
{
  "id": "WH-2WR32451HC0233532-67976317FL4543715",
  "event_type": "PAYMENT.CAPTURE.COMPLETED",
  "resource_type": "capture",
  "create_time": "2026-01-17T10:35:00.000Z",
  "resource": {
    "id": "3C679366HH908993F",
    "status": "COMPLETED",
    "amount": {
      "total": "100.00",
      "currency": "USD"
    },
    "final_capture": true,
    "invoice_id": "pi_abc123",
    "custom_id": "order_789",
    "create_time": "2026-01-17T10:35:00.000Z",
    "update_time": "2026-01-17T10:35:00.000Z"
  }
}
```

### PAYMENT.CAPTURE.DENIED

```json
{
  "id": "WH-2WR32451HC0233532-67976317FL4543716",
  "event_type": "PAYMENT.CAPTURE.DENIED",
  "resource_type": "capture",
  "create_time": "2026-01-17T10:35:00.000Z",
  "resource": {
    "id": "3C679366HH908993G",
    "status": "DECLINED",
    "status_details": {
      "reason": "INSUFFICIENT_FUNDS"
    },
    "amount": {
      "total": "100.00",
      "currency": "USD"
    },
    "invoice_id": "pi_abc123"
  }
}
```

### CUSTOMER.DISPUTE.CREATED

```json
{
  "id": "WH-2WR32451HC0233532-67976317FL4543717",
  "event_type": "CUSTOMER.DISPUTE.CREATED",
  "resource_type": "dispute",
  "create_time": "2026-01-17T15:00:00.000Z",
  "resource": {
    "dispute_id": "PP-D-12345",
    "reason": "MERCHANDISE_OR_SERVICE_NOT_RECEIVED",
    "status": "OPEN",
    "dispute_amount": {
      "currency_code": "USD",
      "value": "100.00"
    },
    "create_time": "2026-01-17T15:00:00.000Z",
    "disputed_transactions": [
      {
        "seller_transaction_id": "3C679366HH908993F",
        "invoice_number": "pi_abc123"
      }
    ]
  }
}
```

## Response Handling

### Success Response

Return `200 OK` with empty body:

```json
{}
```

### Error Responses

| Status | Meaning | PayPal Behavior |
|--------|---------|-----------------|
| `200` | Success | Event marked delivered |
| `401` | Invalid signature | Event marked failed |
| `503` | Service unavailable | PayPal will retry |
| `5xx` | Server error | PayPal will retry |

**Retry Schedule:** PayPal retries for up to 3 days with exponential backoff.

## Idempotency

Use the webhook `id` field for idempotency:

```go
func (a *PayPalAdapter) HandleWebhook(ctx context.Context, event WebhookEvent) error {
    if a.repo.EventProcessed(ctx, event.ID) {
        return nil // Already processed
    }

    // Process event...

    a.repo.MarkEventProcessed(ctx, event.ID, time.Now())
    return nil
}
```

## Testing

### PayPal Sandbox

1. Go to PayPal Developer Dashboard
2. Navigate to Sandbox > Webhooks
3. Select your webhook URL
4. Use "Send test notification" to trigger events

### Webhook Simulator

Use the local webhook simulator:

```bash
# Send authorization created
curl -X POST http://localhost:8081/simulate/paypal/payment.authorization.created \
  -d '{"invoice_id": "pi_abc123", "amount": "100.00"}'

# Send capture completed
curl -X POST http://localhost:8081/simulate/paypal/payment.capture.completed \
  -d '{"invoice_id": "pi_abc123", "amount": "100.00"}'

# Send decline
curl -X POST http://localhost:8081/simulate/paypal/payment.capture.denied \
  -d '{"invoice_id": "pi_abc123", "amount": "100.00", "reason": "INSUFFICIENT_FUNDS"}'
```

See [Simulations Documentation](../../simulations/readme.md) for details.

## PayPal-Specific Considerations

### Invoice ID Mapping

PayPal uses `invoice_id` and `custom_id` fields to correlate webhooks with your internal records:

- `invoice_id`: Set this to your internal payment ID
- `custom_id`: Use for additional reference (e.g., order ID)

### Authorization Expiration

PayPal authorizations typically expire after **3 days** (can be extended to 29 days with honor period). The `expiration_time` field indicates when the authorization will expire.

### Multiple Captures

PayPal supports multiple partial captures on a single authorization. Track the `final_capture` boolean to know when the authorization is fully consumed.
