# Core Tables

Database tables for payment processing core functionality.

## payment_intents

Represents a customer's intention to pay. Central entity in the payment lifecycle.

```sql
CREATE TABLE payment_intents (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key       VARCHAR(100) UNIQUE NOT NULL,
    customer_id           VARCHAR(50) NOT NULL,
    amount                DECIMAL(19,4) NOT NULL,
    currency              VARCHAR(3) NOT NULL,
    status                VARCHAR(20) NOT NULL,
    capture_method        VARCHAR(20) NOT NULL DEFAULT 'AUTOMATIC',
    provider              VARCHAR(20) NOT NULL,
    provider_payment_id   VARCHAR(100),
    payment_method_id     UUID REFERENCES payment_methods(id),
    workflow_id           VARCHAR(100) UNIQUE,
    metadata              JSONB,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_intents_customer ON payment_intents(customer_id);
CREATE INDEX idx_intents_status ON payment_intents(status);
CREATE INDEX idx_intents_provider ON payment_intents(provider);
CREATE INDEX idx_intents_provider_pmt_id ON payment_intents(provider, provider_payment_id);
CREATE INDEX idx_intents_created ON payment_intents(created_at);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `idempotency_key` | VARCHAR(100) | Client-provided deduplication key |
| `customer_id` | VARCHAR(50) | Customer identifier |
| `amount` | DECIMAL(19,4) | Payment amount in smallest currency unit |
| `currency` | VARCHAR(3) | ISO 4217 currency code |
| `status` | VARCHAR(20) | Current state (see states below) |
| `capture_method` | VARCHAR(20) | AUTOMATIC or MANUAL |
| `provider` | VARCHAR(20) | STRIPE, ADYEN, or PAYPAL |
| `provider_payment_id` | VARCHAR(100) | Provider's payment/charge ID |
| `payment_method_id` | UUID | Attached payment method |
| `workflow_id` | VARCHAR(100) | Temporal workflow ID |
| `metadata` | JSONB | Custom key-value data |
| `created_at` | TIMESTAMPTZ | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | Last update timestamp |

### Status Values

| Status | Description |
|--------|-------------|
| `CREATED` | Intent created, no method attached |
| `REQUIRES_AUTH` | Method attached, awaiting authorization |
| `AUTHORIZED` | Authorization approved, hold placed |
| `RECOVERING` | Soft decline, retry in progress |
| `CAPTURED` | Funds captured (terminal) |
| `VOIDED` | Authorization voided (terminal) |
| `FAILED` | Hard decline or retries exhausted (terminal) |
| `CANCELLED` | Manually cancelled (terminal) |

---

## payment_methods

Tokenized payment instruments associated with customers.

```sql
CREATE TABLE payment_methods (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id     VARCHAR(50) NOT NULL,
    type            VARCHAR(20) NOT NULL,
    provider        VARCHAR(20) NOT NULL,
    provider_token  VARCHAR(100) NOT NULL,
    last_four       VARCHAR(4),
    expiry_month    INTEGER,
    expiry_year     INTEGER,
    card_brand      VARCHAR(20),
    is_default      BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_methods_customer ON payment_methods(customer_id);
CREATE INDEX idx_methods_provider ON payment_methods(provider);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `customer_id` | VARCHAR(50) | Owning customer |
| `type` | VARCHAR(20) | CARD, BANK_ACCOUNT, WALLET |
| `provider` | VARCHAR(20) | Which PSP holds the token |
| `provider_token` | VARCHAR(100) | Provider's token ID |
| `last_four` | VARCHAR(4) | Last 4 digits (cards) |
| `expiry_month` | INTEGER | Card expiry month |
| `expiry_year` | INTEGER | Card expiry year |
| `card_brand` | VARCHAR(20) | VISA, MASTERCARD, AMEX, etc. |
| `is_default` | BOOLEAN | Default method for customer |
| `created_at` | TIMESTAMPTZ | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | Last update timestamp |

---

## authorization_holds

Tracks active authorization holds placed on customer funds.

```sql
CREATE TABLE authorization_holds (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_intent_id     UUID NOT NULL REFERENCES payment_intents(id),
    amount                DECIMAL(19,4) NOT NULL,
    currency              VARCHAR(3) NOT NULL,
    status                VARCHAR(20) NOT NULL,
    provider              VARCHAR(20) NOT NULL,
    authorization_code    VARCHAR(50),
    network_txn_id        VARCHAR(100),
    expires_at            TIMESTAMPTZ NOT NULL,
    captured_amount       DECIMAL(19,4),
    captured_at           TIMESTAMPTZ,
    voided_at             TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_holds_intent ON authorization_holds(payment_intent_id);
CREATE INDEX idx_holds_status ON authorization_holds(status);
CREATE INDEX idx_holds_expires ON authorization_holds(expires_at) WHERE status = 'ACTIVE';
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `payment_intent_id` | UUID | Parent payment intent |
| `amount` | DECIMAL(19,4) | Authorized amount |
| `currency` | VARCHAR(3) | Currency code |
| `status` | VARCHAR(20) | ACTIVE, CAPTURED, VOIDED, EXPIRED |
| `provider` | VARCHAR(20) | Authorizing provider |
| `authorization_code` | VARCHAR(50) | Issuer auth code |
| `network_txn_id` | VARCHAR(100) | Card network reference |
| `expires_at` | TIMESTAMPTZ | Hold expiration time |
| `captured_amount` | DECIMAL(19,4) | Amount captured (may be partial) |
| `captured_at` | TIMESTAMPTZ | Capture timestamp |
| `voided_at` | TIMESTAMPTZ | Void timestamp |
| `created_at` | TIMESTAMPTZ | Creation timestamp |

### Status Values

| Status | Description |
|--------|-------------|
| `ACTIVE` | Hold is active, can be captured |
| `CAPTURED` | Hold was captured (fully or partially) |
| `VOIDED` | Hold was voided before capture |
| `EXPIRED` | Hold expired without capture |

---

## payment_attempts

Immutable record of each payment attempt. Implements linear state machine pattern.

```sql
CREATE TABLE payment_attempts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_intent_id     UUID NOT NULL REFERENCES payment_intents(id),
    attempt_number        INTEGER NOT NULL,
    status                VARCHAR(20) NOT NULL,
    provider              VARCHAR(20) NOT NULL,
    provider_response     VARCHAR(100),
    canonical_decline     VARCHAR(50),
    decline_type          VARCHAR(20),
    processor_txn_id      VARCHAR(100),
    idempotency_key       VARCHAR(150) UNIQUE NOT NULL,
    error_message         TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at          TIMESTAMPTZ,

    CONSTRAINT uq_attempt_number UNIQUE (payment_intent_id, attempt_number)
);

CREATE INDEX idx_attempts_intent ON payment_attempts(payment_intent_id);
CREATE INDEX idx_attempts_status ON payment_attempts(status);
CREATE INDEX idx_attempts_canonical ON payment_attempts(canonical_decline);
CREATE INDEX idx_attempts_created ON payment_attempts(created_at);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `payment_intent_id` | UUID | Parent payment intent |
| `attempt_number` | INTEGER | Sequential attempt number (1, 2, 3...) |
| `status` | VARCHAR(20) | PENDING, PROCESSING, SUCCEEDED, FAILED |
| `provider` | VARCHAR(20) | Provider used for this attempt |
| `provider_response` | VARCHAR(100) | Raw provider response code |
| `canonical_decline` | VARCHAR(50) | Normalized decline code |
| `decline_type` | VARCHAR(20) | SOFT, HARD, FRAUD, TEMPORARY |
| `processor_txn_id` | VARCHAR(100) | Provider's transaction ID |
| `idempotency_key` | VARCHAR(150) | Attempt-specific idempotency key |
| `error_message` | TEXT | Human-readable error message |
| `created_at` | TIMESTAMPTZ | Attempt start time |
| `completed_at` | TIMESTAMPTZ | Attempt completion time |

### Status Values (Linear - No Backward Transitions)

```
PENDING → PROCESSING → SUCCEEDED
              │
              └──────→ FAILED
```

### Idempotency Key Format

```
{workflow_id}-{provider}-{operation}-{attempt_number}
```

Example: `payment-abc123-STRIPE-authorize-2`

---

## decline_code_mappings

Maps provider-specific decline codes to canonical codes.

```sql
CREATE TABLE decline_code_mappings (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider              VARCHAR(20) NOT NULL,
    provider_code         VARCHAR(100) NOT NULL,
    canonical_code        VARCHAR(50) NOT NULL,
    decline_type          VARCHAR(20) NOT NULL,
    description           TEXT,
    retry_eligible        BOOLEAN NOT NULL DEFAULT false,
    suggested_action      TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_provider_code UNIQUE (provider, provider_code)
);

CREATE INDEX idx_decline_provider ON decline_code_mappings(provider);
CREATE INDEX idx_decline_canonical ON decline_code_mappings(canonical_code);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `provider` | VARCHAR(20) | STRIPE, ADYEN, PAYPAL |
| `provider_code` | VARCHAR(100) | Raw provider decline code |
| `canonical_code` | VARCHAR(50) | Normalized internal code |
| `decline_type` | VARCHAR(20) | SOFT, HARD, FRAUD, TEMPORARY |
| `description` | TEXT | Human-readable description |
| `retry_eligible` | BOOLEAN | Whether retry is permitted |
| `suggested_action` | TEXT | Recommended customer action |
| `created_at` | TIMESTAMPTZ | Creation timestamp |

### Example Data

```sql
INSERT INTO decline_code_mappings (provider, provider_code, canonical_code, decline_type, retry_eligible, description) VALUES
('STRIPE', 'insufficient_funds', 'INSUFFICIENT_FUNDS', 'SOFT', true, 'Card has insufficient funds'),
('STRIPE', 'expired_card', 'CARD_EXPIRED', 'HARD', false, 'Card has expired'),
('STRIPE', 'fraudulent', 'FRAUD_SUSPICION', 'FRAUD', false, 'Suspected fraudulent transaction'),
('ADYEN', 'Refused:51', 'INSUFFICIENT_FUNDS', 'SOFT', true, 'Not enough balance'),
('ADYEN', 'Refused:33', 'CARD_EXPIRED', 'HARD', false, 'Expired card'),
('PAYPAL', 'INSUFFICIENT_FUNDS', 'INSUFFICIENT_FUNDS', 'SOFT', true, 'Insufficient funds in account');
```
