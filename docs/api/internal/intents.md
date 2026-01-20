# Payment Intents API

Payment Intents represent the lifecycle of a payment from creation through capture or cancellation.

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/intents` | Create a payment intent |
| `GET` | `/api/v1/intents/:id` | Get intent status |
| `PUT` | `/api/v1/intents/:id/method` | Attach payment method |
| `POST` | `/api/v1/intents/:id/authorize` | Request authorization |
| `POST` | `/api/v1/intents/:id/capture` | Capture authorized funds |
| `POST` | `/api/v1/intents/:id/cancel` | Cancel intent |
| `GET` | `/api/v1/intents/:id/attempts` | Get attempt history |
| `GET` | `/api/v1/intents/:id/hold` | Get authorization hold |

---

## Create Payment Intent

Creates a new payment intent. The intent must specify which payment provider to use.

### Request

```
POST /api/v1/intents
```

### Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Yes | `application/json` |
| `Idempotency-Key` | Yes | Unique key for request deduplication |

### Body Parameters

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | string | Yes | Decimal amount (e.g., "100.00") |
| `currency` | string | Yes | ISO 4217 currency code (e.g., "USD") |
| `customer_id` | string | Yes | Customer identifier |
| `provider` | string | Yes | Payment provider: `STRIPE`, `ADYEN`, or `PAYPAL` |
| `capture_method` | string | No | `automatic` (default) or `manual` |
| `payment_method_id` | string | No | Pre-attach a payment method |
| `metadata` | object | No | Custom key-value pairs |

### Example Request

```bash
curl -X POST http://localhost:8080/api/v1/intents \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{
    "amount": "100.00",
    "currency": "USD",
    "customer_id": "cust_123",
    "provider": "STRIPE",
    "capture_method": "manual",
    "metadata": {
      "order_id": "order_789"
    }
  }'
```

### Example Response

```json
{
  "id": "pi_abc123",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "customer_id": "cust_123",
  "amount": "100.00",
  "currency": "USD",
  "status": "created",
  "provider": "STRIPE",
  "capture_method": "manual",
  "payment_method_id": null,
  "workflow_id": "payment-550e8400-e29b-41d4-a716-446655440000",
  "metadata": {
    "order_id": "order_789"
  },
  "created_at": "2026-01-17T10:30:00Z",
  "updated_at": "2026-01-17T10:30:00Z"
}
```

### Response Codes

| Code | Description |
|------|-------------|
| `201` | Intent created successfully |
| `400` | Invalid request body |
| `409` | Idempotency key conflict |

---

## Get Payment Intent

Retrieves the current status of a payment intent.

### Request

```
GET /api/v1/intents/:id
```

### Example Request

```bash
curl http://localhost:8080/api/v1/intents/pi_abc123
```

### Example Response

```json
{
  "id": "pi_abc123",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440000",
  "customer_id": "cust_123",
  "amount": "100.00",
  "currency": "USD",
  "status": "authorized",
  "provider": "STRIPE",
  "provider_payment_id": "pi_stripe_xyz789",
  "capture_method": "manual",
  "payment_method_id": "pm_456",
  "workflow_id": "payment-550e8400-e29b-41d4-a716-446655440000",
  "hold": {
    "id": "hold_def456",
    "amount": "100.00",
    "status": "active",
    "expires_at": "2026-01-24T10:30:00Z"
  },
  "metadata": {
    "order_id": "order_789"
  },
  "created_at": "2026-01-17T10:30:00Z",
  "updated_at": "2026-01-17T10:35:00Z"
}
```

---

## Attach Payment Method

Attaches a payment method to an existing intent. Can be updated until authorization.

### Request

```
PUT /api/v1/intents/:id/method
```

### Body Parameters

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `payment_method_id` | string | Yes | Payment method identifier |

### Example Request

```bash
curl -X PUT http://localhost:8080/api/v1/intents/pi_abc123/method \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: 660e8400-e29b-41d4-a716-446655440001" \
  -d '{
    "payment_method_id": "pm_456"
  }'
```

### Example Response

```json
{
  "id": "pi_abc123",
  "status": "requires_authorization",
  "payment_method_id": "pm_456",
  "updated_at": "2026-01-17T10:32:00Z"
}
```

---

## Authorize Payment

Requests authorization from the payment provider. Places a hold on customer funds.

### Request

```
POST /api/v1/intents/:id/authorize
```

### Example Request

```bash
curl -X POST http://localhost:8080/api/v1/intents/pi_abc123/authorize \
  -H "Idempotency-Key: 770e8400-e29b-41d4-a716-446655440002"
```

### Example Response (Success)

```json
{
  "id": "pi_abc123",
  "status": "authorized",
  "provider_payment_id": "pi_stripe_xyz789",
  "hold": {
    "id": "hold_def456",
    "amount": "100.00",
    "status": "active",
    "authorization_code": "AUTH123",
    "expires_at": "2026-01-24T10:30:00Z"
  },
  "updated_at": "2026-01-17T10:35:00Z"
}
```

### Example Response (Declined)

```json
{
  "id": "pi_abc123",
  "status": "requires_payment_method",
  "last_decline": {
    "canonical_code": "INSUFFICIENT_FUNDS",
    "decline_type": "soft",
    "message": "The card has insufficient funds",
    "retry_eligible": true
  },
  "updated_at": "2026-01-17T10:35:00Z"
}
```

---

## Capture Payment

Captures previously authorized funds. For `automatic` capture, this happens automatically.

### Request

```
POST /api/v1/intents/:id/capture
```

### Body Parameters (Optional)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | string | No | Partial capture amount (must be <= authorized) |

### Example Request

```bash
curl -X POST http://localhost:8080/api/v1/intents/pi_abc123/capture \
  -H "Idempotency-Key: 880e8400-e29b-41d4-a716-446655440003"
```

### Example Response

```json
{
  "id": "pi_abc123",
  "status": "captured",
  "amount": "100.00",
  "captured_amount": "100.00",
  "captured_at": "2026-01-17T10:40:00Z",
  "updated_at": "2026-01-17T10:40:00Z"
}
```

---

## Cancel Payment Intent

Cancels a payment intent. Only valid before capture.

### Request

```
POST /api/v1/intents/:id/cancel
```

### Body Parameters

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `reason` | string | No | Cancellation reason |

### Example Request

```bash
curl -X POST http://localhost:8080/api/v1/intents/pi_abc123/cancel \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "customer_request"
  }'
```

### Example Response

```json
{
  "id": "pi_abc123",
  "status": "cancelled",
  "cancellation_reason": "customer_request",
  "cancelled_at": "2026-01-17T10:45:00Z"
}
```

---

## Get Attempt History

Returns the history of payment attempts for an intent.

### Request

```
GET /api/v1/intents/:id/attempts
```

### Example Response

```json
{
  "data": [
    {
      "id": "att_001",
      "attempt_number": 1,
      "status": "failed",
      "provider": "STRIPE",
      "provider_response": "insufficient_funds",
      "canonical_decline": "INSUFFICIENT_FUNDS",
      "decline_type": "soft",
      "created_at": "2026-01-17T10:35:00Z",
      "completed_at": "2026-01-17T10:35:02Z"
    },
    {
      "id": "att_002",
      "attempt_number": 2,
      "status": "succeeded",
      "provider": "STRIPE",
      "processor_txn_id": "txn_stripe_123",
      "created_at": "2026-01-18T09:00:00Z",
      "completed_at": "2026-01-18T09:00:03Z"
    }
  ],
  "has_more": false
}
```

---

## Payment Intent States

| State | Description | Valid Transitions |
|-------|-------------|-------------------|
| `created` | Intent created, no method attached | `requires_authorization`, `cancelled` |
| `requires_authorization` | Method attached, awaiting auth | `authorized`, `recovering`, `cancelled` |
| `authorized` | Auth approved, hold placed | `captured`, `voided`, `cancelled` |
| `recovering` | Soft decline, retrying | `authorized`, `failed`, `cancelled` |
| `captured` | Funds captured (terminal) | - |
| `voided` | Authorization voided (terminal) | - |
| `failed` | Hard decline or retries exhausted (terminal) | - |
| `cancelled` | Manually cancelled (terminal) | - |
