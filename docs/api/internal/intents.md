# Payment Intents API

Payment Intents represent the lifecycle of a payment from creation through capture or cancellation. This is the primary API for processing payments.

## Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/v1/intents` | Create a payment intent |
| GET | `/api/v1/intents/:id` | Get intent status |
| PUT | `/api/v1/intents/:id/method` | Attach payment method |
| POST | `/api/v1/intents/:id/authorize` | Request authorization |
| POST | `/api/v1/intents/:id/capture` | Capture authorized funds |
| POST | `/api/v1/intents/:id/cancel` | Cancel intent |
| GET | `/api/v1/intents/:id/attempts` | Get attempt history |
| GET | `/api/v1/intents/:id/hold` | Get authorization hold details |

---

## Create Payment Intent

Creates a new payment intent specifying the amount, currency, customer, and payment provider.

### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | string | Yes | Decimal amount (e.g., "100.00") |
| `currency` | string | Yes | ISO 4217 currency code (e.g., "USD") |
| `customer_id` | string | Yes | Customer identifier |
| `provider` | string | Yes | Payment provider: `STRIPE`, `ADYEN`, or `PAYPAL` |
| `capture_method` | string | No | `automatic` (default) or `manual` |
| `payment_method_id` | string | No | Pre-attach a payment method |
| `metadata` | object | No | Custom key-value pairs for client use |

### Response Fields

| Field | Description |
|-------|-------------|
| `id` | Unique payment intent identifier |
| `idempotency_key` | The provided deduplication key |
| `status` | Current state of the intent |
| `workflow_id` | Temporal workflow ID for tracking |
| `created_at` | Creation timestamp |

### Business Rules

- Amount must be greater than zero
- Currency must be a supported ISO 4217 code
- Provider must be one of the configured payment providers
- Idempotency key is required and must be unique within 24 hours

---

## Get Payment Intent

Retrieves the current status and details of a payment intent.

### Response Includes

- Current status and all state timestamps
- Attached payment method (if any)
- Authorization hold details (if authorized)
- Provider-assigned payment ID
- Custom metadata

---

## Attach Payment Method

Attaches or updates the payment method on an intent. Can only be done before authorization.

### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `payment_method_id` | string | Yes | Payment method identifier |

### Business Rules

- Payment method must belong to the same customer
- Cannot attach method after authorization has occurred
- Method can be changed multiple times before authorization

---

## Authorize Payment

Requests authorization from the payment provider, placing a hold on customer funds.

### Behavior

- Calls the configured payment provider to request authorization
- On success: creates an authorization hold and updates status to `authorized`
- On soft decline: status moves to `recovering` for automatic retry
- On hard decline: status moves to `requires_payment_method`

### Response Includes (Success)

- Authorization hold ID and amount
- Authorization code from issuer
- Hold expiration timestamp
- Provider transaction ID

### Response Includes (Decline)

- Canonical decline code (normalized across providers)
- Decline type: `soft`, `hard`, or `fraud`
- Whether retry is eligible
- Human-readable decline message

---

## Capture Payment

Captures previously authorized funds. For intents with `automatic` capture, this happens immediately after authorization.

### Request Fields (Optional)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | string | No | Partial capture amount (must be <= authorized amount) |

### Business Rules

- Can only capture an authorized intent
- Capture amount cannot exceed authorized amount
- Partial capture is supported (capture less than authorized)
- After capture, the intent reaches terminal state

---

## Cancel Payment Intent

Cancels a payment intent. Behavior depends on current state.

### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `reason` | string | No | Cancellation reason for audit |

### Behavior by State

| Current State | Cancellation Behavior |
|---------------|----------------------|
| `created` | Immediate cancellation |
| `requires_authorization` | Immediate cancellation |
| `authorized` | Voids the authorization hold, then cancels |
| `recovering` | Stops retry attempts, cancels |
| `captured` | Cannot cancel (use refund instead) |

---

## Get Attempt History

Returns the history of all payment attempts for an intent. Each retry creates a new attempt record.

### Response Fields per Attempt

| Field | Description |
|-------|-------------|
| `attempt_number` | Sequential attempt number (1, 2, 3...) |
| `status` | Attempt outcome: `succeeded` or `failed` |
| `provider` | Provider used for this attempt |
| `canonical_decline` | Normalized decline code (if failed) |
| `decline_type` | Category: `soft`, `hard`, `fraud` |
| `created_at` | When attempt started |
| `completed_at` | When attempt finished |

---

## Payment Intent States

### State Definitions

| State | Description |
|-------|-------------|
| `created` | Intent created, no payment method attached |
| `requires_authorization` | Payment method attached, awaiting authorization |
| `authorized` | Authorization approved, hold placed on funds |
| `recovering` | Soft decline occurred, automatic retry in progress |
| `captured` | Funds captured successfully (terminal) |
| `voided` | Authorization voided before capture (terminal) |
| `failed` | Hard decline or retries exhausted (terminal) |
| `cancelled` | Manually cancelled (terminal) |

### State Transitions

| From State | Valid Transitions |
|------------|-------------------|
| `created` | `requires_authorization`, `cancelled` |
| `requires_authorization` | `authorized`, `recovering`, `failed`, `cancelled` |
| `authorized` | `captured`, `voided`, `cancelled` |
| `recovering` | `authorized`, `failed`, `cancelled` |

### Terminal States

Once an intent reaches `captured`, `voided`, `failed`, or `cancelled`, no further state changes are possible. Refunds are handled as separate transactions.

---

## Decline Handling

### Decline Categories

| Category | Retry Eligible | Examples |
|----------|----------------|----------|
| Soft | Yes | Insufficient funds, generic decline, do not honor |
| Hard | No | Card expired, invalid number, account closed |
| Fraud | No | Suspected fraud, stolen card, lost card |

### Automatic Retry Behavior

For soft declines with `capture_method: automatic`:
- System automatically schedules retry attempts
- Retry timing varies by decline type (e.g., insufficient funds waits for payday)
- Maximum retry attempts is configurable (default: 6)
- Customer can update payment method to trigger immediate retry
