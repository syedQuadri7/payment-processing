# Adyen Webhooks (Notifications)

Receiving and processing Adyen notification webhooks.

## Endpoint

POST `/webhooks/adyen`

## Authentication

Adyen signs notifications using HMAC-SHA256.

### Signature Header

| Header | Description |
|--------|-------------|
| `HmacSignature` | HMAC-SHA256 signature of the payload |

### Verification Requirements

1. Extract HMAC signature from header
2. Compute HMAC-SHA256 of raw request body using HMAC key
3. Compare signatures using constant-time comparison
4. Reject with 401 if signatures don't match

### Configuration

| Variable | Source | Purpose |
|----------|--------|---------|
| `ADYEN_HMAC_KEY` | Adyen Customer Area > Developers > Webhooks | HMAC signing key |

---

## Notification Format

Adyen sends notifications as an array of `notificationItems`. A single request may contain multiple notifications (batching).

### Key Fields

| Field | Description |
|-------|-------------|
| `eventCode` | Type of notification (AUTHORISATION, CAPTURE, etc.) |
| `success` | Whether the operation succeeded (string "true"/"false") |
| `pspReference` | Adyen's unique reference for this operation |
| `merchantReference` | Our internal payment intent ID |
| `originalReference` | Reference to original authorization (for captures, refunds) |
| `amount.value` | Amount in smallest currency unit |
| `amount.currency` | Currency code |
| `reason` | Decline reason code (if failed) |

---

## Event Mapping

Adyen events must be normalized to canonical events. Note that success/failure is indicated by the `success` field, not separate event codes.

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

---

## Decline Code Mapping

Adyen uses reason codes in the format `Refused:XX` where XX is a numeric code.

### Soft Declines (Retry Eligible)

| Adyen Reason | Canonical Code |
|--------------|----------------|
| `Refused:51` | `INSUFFICIENT_FUNDS` |
| `Refused:05` | `GENERIC_DECLINE` |
| `Refused:57` | `DO_NOT_HONOR` |
| `Refused:91` | `TRY_AGAIN` |
| `Refused:96` | `PROCESSING_ERROR` |

### Hard Declines (Not Retry Eligible)

| Adyen Reason | Canonical Code |
|--------------|----------------|
| `Refused:33` | `CARD_EXPIRED` |
| `Refused:14` | `INVALID_NUMBER` |
| `Refused:82` | `INVALID_CVV` |
| `Refused:62` | `CARD_RESTRICTED` |
| `Refused:63` | `CARD_RESTRICTED` |

### Fraud Declines

| Adyen Reason | Canonical Code |
|--------------|----------------|
| `Refused:59` | `FRAUD_SUSPICION` |
| `Refused:41` | `LOST_CARD` |
| `Refused:43` | `STOLEN_CARD` |

---

## Response Requirements

### Success

Adyen requires a specific acknowledgment response:
- HTTP status: 200
- Body: Plain text `[accepted]`

### Error Handling

| HTTP Status | Meaning | Adyen Behavior |
|-------------|---------|----------------|
| 200 with `[accepted]` | Success | Notification marked delivered |
| 401 | Invalid HMAC | No retry |
| 4xx | Client error | Limited retries |
| 5xx | Server error | Retries with exponential backoff |

### Retry Behavior

Adyen retries failed notifications for up to 7 days.

---

## Batch Processing

Adyen may send multiple notifications in a single request. Requirements:

1. Process each notification item individually
2. Only return `[accepted]` if ALL items are successfully processed or queued
3. If any item fails fatally, return error to trigger retry of entire batch

---

## Idempotency Requirements

The adapter must handle duplicate notifications:

1. Use `pspReference` + `eventCode` as the unique identifier
2. Return 200 for already-processed notifications (do not reprocess)
3. Track processed notification IDs

---

## Provider-Specific Considerations

### Merchant Reference

Our internal payment intent ID is passed as `merchantReference` when creating the payment. This enables correlation when receiving notifications.

### Original Reference

For operations on existing authorizations (capture, refund, cancellation), the `originalReference` field links back to the original authorization's `pspReference`.

### Test vs Live

The `live` field indicates whether this is a test ("false") or production ("true") notification.
