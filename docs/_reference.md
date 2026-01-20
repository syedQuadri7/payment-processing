# Quick Reference

Fast lookup for common values, codes, and patterns used in the payment processing service.

## Canonical Event Types

| Event Type | Description |
|------------|-------------|
| `AUTHORIZATION_SUCCEEDED` | Authorization approved, hold placed |
| `AUTHORIZATION_FAILED` | Authorization declined |
| `CAPTURE_SUCCEEDED` | Capture confirmed |
| `CAPTURE_FAILED` | Capture failed |
| `VOID_SUCCEEDED` | Authorization voided |
| `REFUND_SUCCEEDED` | Refund processed |
| `REFUND_FAILED` | Refund failed |
| `DISPUTE_OPENED` | Chargeback initiated |
| `DISPUTE_WON` | Dispute resolved in our favor |
| `DISPUTE_LOST` | Dispute resolved against us |

## Canonical Decline Codes

### Soft Declines (Retry Eligible)

| Code | Description | Retry Strategy |
|------|-------------|----------------|
| `INSUFFICIENT_FUNDS` | Not enough balance | Align with paydays |
| `OVER_LIMIT` | Credit limit exceeded | Wait 24-48 hours |
| `GENERIC_DECLINE` | Unspecified decline | Exponential backoff |
| `DO_NOT_HONOR` | Issuer refused without reason | Vary timing |
| `TRY_AGAIN` | Temporary processing error | Retry in 1-4 hours |
| `PROCESSING_ERROR` | System error | Retry with backoff |

### Hard Declines (Not Retry Eligible)

| Code | Description | Action Required |
|------|-------------|-----------------|
| `CARD_EXPIRED` | Card past expiration | Request new card |
| `INVALID_NUMBER` | Card number invalid | Verify card details |
| `INVALID_CVV` | CVV mismatch | Re-enter CVV |
| `ACCOUNT_CLOSED` | Account no longer active | Contact customer |
| `CARD_RESTRICTED` | Card has restrictions | Use different card |

### Fraud Declines (Never Retry)

| Code | Description | Action Required |
|------|-------------|-----------------|
| `FRAUD_SUSPICION` | Suspected fraud | Flag for review |
| `STOLEN_CARD` | Reported stolen | Do not process |
| `LOST_CARD` | Reported lost | Do not process |

## Provider Event Mapping

### Stripe to Canonical

| Stripe Event | Canonical Event |
|--------------|-----------------|
| `payment_intent.succeeded` | `AUTHORIZATION_SUCCEEDED` |
| `payment_intent.payment_failed` | `AUTHORIZATION_FAILED` |
| `charge.captured` | `CAPTURE_SUCCEEDED` |
| `charge.failed` | `CAPTURE_FAILED` |
| `charge.refunded` | `REFUND_SUCCEEDED` |
| `charge.dispute.created` | `DISPUTE_OPENED` |

### Adyen to Canonical

| Adyen Notification | Canonical Event |
|--------------------|-----------------|
| `AUTHORISATION` (success=true) | `AUTHORIZATION_SUCCEEDED` |
| `AUTHORISATION` (success=false) | `AUTHORIZATION_FAILED` |
| `CAPTURE` | `CAPTURE_SUCCEEDED` |
| `CAPTURE_FAILED` | `CAPTURE_FAILED` |
| `REFUND` | `REFUND_SUCCEEDED` |
| `CHARGEBACK` | `DISPUTE_OPENED` |

### PayPal to Canonical

| PayPal Event | Canonical Event |
|--------------|-----------------|
| `PAYMENT.AUTHORIZATION.CREATED` | `AUTHORIZATION_SUCCEEDED` |
| `PAYMENT.AUTHORIZATION.VOIDED` | `AUTHORIZATION_FAILED` |
| `PAYMENT.CAPTURE.COMPLETED` | `CAPTURE_SUCCEEDED` |
| `PAYMENT.CAPTURE.DENIED` | `CAPTURE_FAILED` |
| `PAYMENT.CAPTURE.REFUNDED` | `REFUND_SUCCEEDED` |
| `CUSTOMER.DISPUTE.CREATED` | `DISPUTE_OPENED` |

## Payment Intent States

```
CREATED → REQUIRES_AUTH → AUTHORIZED → CAPTURED
                ↓              ↓
            CANCELLED       VOIDED
                ↓
           RECOVERING → FAILED
```

## API Endpoints Quick Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/intents` | Create payment intent |
| `GET` | `/api/v1/intents/:id` | Get intent status |
| `PUT` | `/api/v1/intents/:id/method` | Attach payment method |
| `POST` | `/api/v1/intents/:id/authorize` | Request authorization |
| `POST` | `/api/v1/intents/:id/capture` | Capture funds |
| `POST` | `/api/v1/intents/:id/cancel` | Cancel intent |
| `POST` | `/webhooks/stripe` | Stripe webhooks |
| `POST` | `/webhooks/adyen` | Adyen notifications |
| `POST` | `/webhooks/paypal` | PayPal webhooks |
| `GET` | `/health` | Health check |
| `GET` | `/metrics` | Prometheus metrics |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TEMPORAL_HOST` | `localhost:7233` | Temporal server address |
| `PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `STRIPE_SECRET_KEY` | - | Stripe API key |
| `STRIPE_WEBHOOK_SECRET` | - | Stripe webhook signing secret |
| `ADYEN_API_KEY` | - | Adyen API key |
| `ADYEN_HMAC_KEY` | - | Adyen HMAC signing key |
| `PAYPAL_CLIENT_ID` | - | PayPal client ID |
| `PAYPAL_CLIENT_SECRET` | - | PayPal client secret |

## Temporal Configuration

| Setting | Value |
|---------|-------|
| Task Queue | `payment-processing` |
| Workflow Timeout | 30 days |
| Activity Initial Interval | 1 second |
| Activity Backoff Coefficient | 2.0 |
| Activity Max Attempts | 3 (PSP calls) |

## Database Tables

| Table | Purpose |
|-------|---------|
| `payment_intents` | Payment intent records |
| `authorization_holds` | Active authorization holds |
| `payment_attempts` | Individual payment attempts |
| `payment_methods` | Stored payment methods |
| `accounts` | Account balances |
| `ledger_entries` | Double-entry bookkeeping |
| `outbox` | Transactional outbox for CDC |
| `audit_log` | Audit trail |
| `decline_code_mappings` | Provider to canonical code mapping |

## HTTP Status Codes

| Code | Meaning | When Used |
|------|---------|-----------|
| `200` | Success | GET requests, webhook acknowledgment |
| `201` | Created | POST creating new resource |
| `202` | Accepted | Async operation started |
| `400` | Bad Request | Validation error |
| `401` | Unauthorized | Invalid webhook signature |
| `404` | Not Found | Resource doesn't exist |
| `409` | Conflict | Idempotency conflict |
| `422` | Unprocessable | Business rule violation |
| `500` | Server Error | Internal error |
