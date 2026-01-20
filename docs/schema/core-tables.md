# Core Tables

Database tables for payment processing core functionality.

## payment_intents

The central entity representing a customer's intention to pay. Tracks the complete payment lifecycle from creation through capture or cancellation.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `idempotency_key` | String(100) | Yes | Client-provided deduplication key (unique) |
| `customer_id` | String(50) | Yes | Customer identifier |
| `amount` | Decimal(19,4) | Yes | Payment amount |
| `currency` | String(3) | Yes | ISO 4217 currency code |
| `status` | String(20) | Yes | Current state |
| `capture_method` | String(20) | Yes | AUTOMATIC or MANUAL (default: AUTOMATIC) |
| `provider` | String(20) | Yes | STRIPE, ADYEN, or PAYPAL |
| `provider_payment_id` | String(100) | No | Provider's payment/charge ID |
| `payment_method_id` | UUID | No | Reference to attached payment method |
| `workflow_id` | String(100) | No | Temporal workflow ID (unique) |
| `metadata` | JSON | No | Custom key-value data |
| `created_at` | Timestamp | Yes | Creation timestamp |
| `updated_at` | Timestamp | Yes | Last update timestamp |

### Status Values

| Status | Terminal | Description |
|--------|----------|-------------|
| `CREATED` | No | Intent created, no method attached |
| `REQUIRES_AUTH` | No | Method attached, awaiting authorization |
| `AUTHORIZED` | No | Authorization approved, hold placed |
| `RECOVERING` | No | Soft decline, retry in progress |
| `CAPTURED` | Yes | Funds captured successfully |
| `VOIDED` | Yes | Authorization voided |
| `FAILED` | Yes | Hard decline or retries exhausted |
| `CANCELLED` | Yes | Manually cancelled |

### Indexes Required

- Customer ID (frequent lookups by customer)
- Status (filtering by payment state)
- Provider (reporting by provider)
- Provider + Provider Payment ID (webhook correlation)
- Created timestamp (time-based queries)

### Business Rules

- Idempotency key must be unique (enforced at database level)
- Workflow ID must be unique when set
- Status transitions must follow defined state machine
- Terminal states cannot transition to other states

---

## payment_methods

Tokenized payment instruments associated with customers. Stores only non-sensitive token references.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `customer_id` | String(50) | Yes | Owning customer |
| `type` | String(20) | Yes | CARD, BANK_ACCOUNT, WALLET |
| `provider` | String(20) | Yes | Which PSP holds the token |
| `provider_token` | String(100) | Yes | Provider's token ID |
| `last_four` | String(4) | No | Last 4 digits (for display) |
| `expiry_month` | Integer | No | Card expiry month |
| `expiry_year` | Integer | No | Card expiry year |
| `card_brand` | String(20) | No | VISA, MASTERCARD, AMEX, etc. |
| `is_default` | Boolean | Yes | Default method for customer |
| `created_at` | Timestamp | Yes | Creation timestamp |
| `updated_at` | Timestamp | Yes | Last update timestamp |

### Security Requirements

- Never store full card numbers, CVV, or sensitive data
- Only store tokenized references from payment providers
- Last four digits stored only for customer display purposes

---

## authorization_holds

Tracks active authorization holds placed on customer funds.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `payment_intent_id` | UUID | Yes | Reference to parent intent |
| `amount` | Decimal(19,4) | Yes | Authorized amount |
| `currency` | String(3) | Yes | Currency code |
| `status` | String(20) | Yes | Hold status |
| `provider` | String(20) | Yes | Authorizing provider |
| `authorization_code` | String(50) | No | Issuer authorization code |
| `network_txn_id` | String(100) | No | Card network reference |
| `expires_at` | Timestamp | Yes | Hold expiration time |
| `captured_amount` | Decimal(19,4) | No | Amount captured (may be partial) |
| `captured_at` | Timestamp | No | Capture timestamp |
| `voided_at` | Timestamp | No | Void timestamp |
| `created_at` | Timestamp | Yes | Creation timestamp |

### Status Values

| Status | Description |
|--------|-------------|
| `ACTIVE` | Hold is active, can be captured or voided |
| `CAPTURED` | Hold was captured (fully or partially) |
| `VOIDED` | Hold was voided before capture |
| `EXPIRED` | Hold expired without action |

### Business Rules

- One active hold per payment intent at a time
- Captured amount cannot exceed authorized amount
- Partial capture is supported
- Holds have provider-specific expiration periods (typically 5-30 days)

### Indexes Required

- Payment intent ID (lookup by parent)
- Status (finding active holds)
- Expires at (WHERE status = 'ACTIVE') - for expiration monitoring

---

## payment_attempts

Immutable record of each payment attempt. Implements linear state machine pattern where each retry creates a new record.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `payment_intent_id` | UUID | Yes | Reference to parent intent |
| `attempt_number` | Integer | Yes | Sequential attempt number (1, 2, 3...) |
| `status` | String(20) | Yes | Attempt status |
| `provider` | String(20) | Yes | Provider used for this attempt |
| `provider_response` | String(100) | No | Raw provider response code |
| `canonical_decline` | String(50) | No | Normalized decline code |
| `decline_type` | String(20) | No | SOFT, HARD, FRAUD, TEMPORARY |
| `processor_txn_id` | String(100) | No | Provider's transaction ID |
| `idempotency_key` | String(150) | Yes | Attempt-specific idempotency key (unique) |
| `error_message` | Text | No | Human-readable error message |
| `created_at` | Timestamp | Yes | Attempt start time |
| `completed_at` | Timestamp | No | Attempt completion time |

### Status Values (Linear State Machine)

| Status | Description | Transitions To |
|--------|-------------|----------------|
| `PENDING` | Attempt created, not started | PROCESSING |
| `PROCESSING` | In progress with provider | SUCCEEDED, FAILED |
| `SUCCEEDED` | Attempt succeeded | (terminal) |
| `FAILED` | Attempt failed | (terminal) |

### Constraints

- Unique constraint on (payment_intent_id, attempt_number)
- Unique constraint on idempotency_key
- No backward state transitions allowed

### Idempotency Key Format

The idempotency key for each attempt should follow the format:
`{workflow_id}-{provider}-{operation}-{attempt_number}`

This ensures each attempt has a unique key even for the same intent.

---

## decline_code_mappings

Configuration table mapping provider-specific decline codes to canonical codes. This table is seeded with initial data and can be updated without code changes.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `provider` | String(20) | Yes | STRIPE, ADYEN, PAYPAL |
| `provider_code` | String(100) | Yes | Raw provider decline code |
| `canonical_code` | String(50) | Yes | Normalized internal code |
| `decline_type` | String(20) | Yes | SOFT, HARD, FRAUD, TEMPORARY |
| `description` | Text | No | Human-readable description |
| `retry_eligible` | Boolean | Yes | Whether retry is permitted |
| `suggested_action` | Text | No | Recommended customer action |
| `created_at` | Timestamp | Yes | Creation timestamp |

### Constraints

- Unique constraint on (provider, provider_code)

### Decline Types

| Type | Retry Eligible | Behavior |
|------|----------------|----------|
| `SOFT` | Yes | Schedule automatic retry |
| `HARD` | No | Stop retrying, request new payment method |
| `FRAUD` | No | Flag for review, do not retry |
| `TEMPORARY` | Yes | Retry after short delay |

### Seed Data Requirements

The table must be seeded with mappings for all known decline codes from:
- Stripe decline codes
- Adyen refusal reasons
- PayPal decline reasons

Unknown codes should fall back to `GENERIC_DECLINE` with `SOFT` type.
