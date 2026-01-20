# Adyen Webhooks (Notifications)

Documentation for receiving and processing Adyen notification webhooks.

## Endpoint

```
POST /webhooks/adyen
```

## Authentication

Adyen signs notifications using HMAC-SHA256. The signature is included in request headers.

### Headers

| Header | Description |
|--------|-------------|
| `HmacSignature` | HMAC-SHA256 signature of the payload |

### Verification Process

1. Extract HMAC signature from header
2. Compute HMAC-SHA256 of raw request body using HMAC key
3. Compare computed signature with provided signature (constant-time comparison)
4. Reject with 401 if signatures don't match

### Configuration

```bash
ADYEN_HMAC_KEY=... # From Adyen Customer Area > Developers > Webhooks
```

## Notification Format

Adyen sends notifications in a specific format with `notificationItems`:

```json
{
  "live": "false",
  "notificationItems": [
    {
      "NotificationRequestItem": {
        "eventCode": "AUTHORISATION",
        "success": "true",
        "pspReference": "8835512345678901",
        "merchantReference": "pi_abc123",
        "amount": {
          "currency": "USD",
          "value": 10000
        },
        "paymentMethod": "visa",
        "reason": "",
        "eventDate": "2026-01-17T10:30:00+00:00"
      }
    }
  ]
}
```

## Event Mapping

Adyen events are normalized to canonical events before processing.

| Adyen Event | Success | Canonical Event |
|-------------|---------|-----------------|
| `AUTHORISATION` | true | `AUTHORIZATION_SUCCEEDED` |
| `AUTHORISATION` | false | `AUTHORIZATION_FAILED` |
| `CAPTURE` | true | `CAPTURE_SUCCEEDED` |
| `CAPTURE` | false | `CAPTURE_FAILED` |
| `CAPTURE_FAILED` | - | `CAPTURE_FAILED` |
| `CANCELLATION` | true | `VOID_SUCCEEDED` |
| `REFUND` | true | `REFUND_SUCCEEDED` |
| `REFUND` | false | `REFUND_FAILED` |
| `CHARGEBACK` | - | `DISPUTE_OPENED` |
| `CHARGEBACK_REVERSED` | - | `DISPUTE_WON` |

## Decline Code Mapping

Adyen uses reason codes in the format `Refused:XX`.

| Adyen Reason | Canonical Code | Decline Type |
|--------------|----------------|--------------|
| `Refused:51` | `INSUFFICIENT_FUNDS` | Soft |
| `Refused:05` | `GENERIC_DECLINE` | Soft |
| `Refused:57` | `DO_NOT_HONOR` | Soft |
| `Refused:91` | `TRY_AGAIN` | Soft |
| `Refused:96` | `PROCESSING_ERROR` | Soft |
| `Refused:33` | `CARD_EXPIRED` | Hard |
| `Refused:14` | `INVALID_NUMBER` | Hard |
| `Refused:82` | `INVALID_CVV` | Hard |
| `Refused:62` | `CARD_RESTRICTED` | Hard |
| `Refused:63` | `CARD_RESTRICTED` | Hard |
| `Refused:59` | `FRAUD_SUSPICION` | Fraud |
| `Refused:41` | `LOST_CARD` | Fraud |
| `Refused:43` | `STOLEN_CARD` | Fraud |

## Example Notification Payloads

### AUTHORISATION (Success)

```json
{
  "live": "false",
  "notificationItems": [
    {
      "NotificationRequestItem": {
        "eventCode": "AUTHORISATION",
        "success": "true",
        "pspReference": "8835512345678901",
        "merchantReference": "pi_abc123",
        "merchantAccountCode": "TestMerchant",
        "amount": {
          "currency": "USD",
          "value": 10000
        },
        "paymentMethod": "visa",
        "operations": ["CAPTURE", "CANCEL"],
        "eventDate": "2026-01-17T10:30:00+00:00",
        "additionalData": {
          "authCode": "AUTH123",
          "cardSummary": "1234"
        }
      }
    }
  ]
}
```

### AUTHORISATION (Failed)

```json
{
  "live": "false",
  "notificationItems": [
    {
      "NotificationRequestItem": {
        "eventCode": "AUTHORISATION",
        "success": "false",
        "pspReference": "8835512345678902",
        "merchantReference": "pi_abc123",
        "amount": {
          "currency": "USD",
          "value": 10000
        },
        "paymentMethod": "visa",
        "reason": "Refused:51",
        "eventDate": "2026-01-17T10:30:00+00:00",
        "additionalData": {
          "refusalReasonRaw": "DECLINED Insufficient Funds"
        }
      }
    }
  ]
}
```

### CAPTURE

```json
{
  "live": "false",
  "notificationItems": [
    {
      "NotificationRequestItem": {
        "eventCode": "CAPTURE",
        "success": "true",
        "pspReference": "8835512345678903",
        "originalReference": "8835512345678901",
        "merchantReference": "pi_abc123",
        "amount": {
          "currency": "USD",
          "value": 10000
        },
        "eventDate": "2026-01-17T10:35:00+00:00"
      }
    }
  ]
}
```

### CHARGEBACK

```json
{
  "live": "false",
  "notificationItems": [
    {
      "NotificationRequestItem": {
        "eventCode": "CHARGEBACK",
        "success": "true",
        "pspReference": "8835512345678904",
        "originalReference": "8835512345678901",
        "merchantReference": "pi_abc123",
        "amount": {
          "currency": "USD",
          "value": 10000
        },
        "reason": "Fraudulent",
        "eventDate": "2026-01-17T15:00:00+00:00",
        "additionalData": {
          "disputeStatus": "PENDING"
        }
      }
    }
  ]
}
```

## Response Handling

### Success Response

Adyen expects a specific acknowledgment response:

```
[accepted]
```

Return this as plain text with status `200`.

### Error Responses

| Status | Meaning | Adyen Behavior |
|--------|---------|----------------|
| `200` with `[accepted]` | Success | Notification marked delivered |
| `401` | Invalid HMAC | No retry |
| `4xx` | Client error | Limited retries |
| `5xx` | Server error | Retries with exponential backoff |

**Retry Schedule:** Adyen retries for up to 7 days.

## Batch Notifications

Adyen may batch multiple notifications in a single request:

```json
{
  "live": "false",
  "notificationItems": [
    {"NotificationRequestItem": {...}},
    {"NotificationRequestItem": {...}},
    {"NotificationRequestItem": {...}}
  ]
}
```

Process each notification item individually. Return `[accepted]` only if all items are successfully processed or stored for retry.

## Idempotency

Use `pspReference` as the unique identifier for idempotency:

```go
func (a *AdyenAdapter) HandleNotification(ctx context.Context, item NotificationRequestItem) error {
    eventKey := fmt.Sprintf("adyen-%s-%s", item.PspReference, item.EventCode)

    if a.repo.EventProcessed(ctx, eventKey) {
        return nil // Already processed
    }

    // Process notification...

    a.repo.MarkEventProcessed(ctx, eventKey, time.Now())
    return nil
}
```

## Testing

### Adyen Test Dashboard

1. Go to Customer Area > Developers > Webhooks
2. Select your webhook configuration
3. Click "Test" to send test notifications

### Webhook Simulator

Use the local webhook simulator:

```bash
# Send test notification
curl -X POST http://localhost:8081/simulate/adyen/authorisation \
  -d '{"merchant_reference": "pi_abc123", "amount": 10000, "success": true}'

# Send decline notification
curl -X POST http://localhost:8081/simulate/adyen/authorisation \
  -d '{"merchant_reference": "pi_abc123", "amount": 10000, "success": false, "reason": "Refused:51"}'
```

See [Simulations Documentation](../../simulations/readme.md) for details.
