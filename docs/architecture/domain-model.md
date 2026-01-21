# Domain Model

Entity relationships, state machines, and core domain concepts.

---

## Core Domain Concepts

The domain model separates three core concerns:

| Concept | Purpose | Examples |
|---------|---------|----------|
| **PaymentIntent** | WHAT is being paid | Amount, currency, customer |
| **PaymentMethod** | HOW it's being paid | Card token, bank account, wallet |
| **Order** | WHAT is being purchased | Line items, merchant, invoice |

This separation enables support for diverse payment methods without architectural rewrites.

---

## Entity Relationships

```
+------------------+         +------------------+
| PaymentIntent    |         | PaymentMethod    |
|------------------|         |------------------|
| id               |         | id               |
| idempotency_key  |<------->| customer_id      |
| customer_id      |         | type             |
| amount           |         | token            |
| currency         |         | last_four        |
| status           |         | provider         |
| capture_method   |         | is_default       |
| provider         |         +------------------+
| payment_method_id|
+--------+---------+
         |
         | 1:1 (when authorized)
         v
+------------------+
| AuthorizationHold|
|------------------|
| id               |
| intent_id        |
| amount           |
| status           |    [ACTIVE, CAPTURED, VOIDED, EXPIRED]
| provider         |
| auth_code        |
| expires_at       |
| captured_amount  |
+--------+---------+
         |
         | 1:N
         v
+------------------+
| PaymentAttempt   |    (Linear State Machine - Immutable)
|------------------|
| id               |
| intent_id        |
| attempt_number   |
| status           |    [PENDING, PROCESSING, SUCCEEDED, FAILED]
| provider         |
| provider_code    |    (Raw provider response)
| canonical_code   |    (Normalized)
| decline_type     |
+------------------+
```

---

## PaymentIntent

Represents the customer's intention to pay a specific amount.

### Fields

| Field | Description |
|-------|-------------|
| id | Unique identifier |
| idempotency_key | Client-provided key preventing duplicates |
| customer_id | Customer reference |
| amount | Payment amount (decimal) |
| currency | ISO 4217 currency code |
| status | Current state in lifecycle |
| capture_method | AUTOMATIC or MANUAL |
| provider | STRIPE, ADYEN, or PAYPAL |
| provider_payment_id | Provider's ID for this payment |
| payment_method_id | Reference to attached payment method |
| metadata | Custom key-value data |

### State Machine

```
                              +-------------+
                              |   CREATED   |
                              +------+------+
                                     |
                                     | attach_method
                                     v
                              +-------------+
                              |REQUIRES_AUTH|
                              +------+------+
                                     |
                         +-----------+-----------+
                         |           |           |
                         | authorize |           | cancel
                         v           |           v
                +-------------+      |    +-------------+
                |  AUTHORIZED |      |    |  CANCELLED  |
                +------+------+      |    +-------------+
                       |             |
           +-----------+-----------+ |
           |           |           | |
           | capture   | void      | | decline (soft)
           v           v           v v
    +----------+ +----------+ +-------------+
    | CAPTURED | |  VOIDED  | |  RECOVERING |
    +----------+ +----------+ +------+------+
                                     |
                         +-----------+-----------+
                         |           |           |
                         | success   | exhaust   |
                         v           v           v
                  +----------+ +----------+ +----------+
                  |AUTHORIZED| |  FAILED  | |CANCELLED |
                  +----------+ +----------+ +----------+
```

---

## AuthorizationHold

First-class entity representing a hold on customer funds.

### Fields

| Field | Description |
|-------|-------------|
| id | Unique hold identifier |
| payment_intent_id | Parent payment intent |
| amount | Authorized amount |
| currency | ISO 4217 currency code |
| status | ACTIVE, CAPTURED, VOIDED, EXPIRED |
| provider | Which PSP holds the authorization |
| processor_auth_code | Provider's authorization code |
| expires_at | When the hold expires |
| captured_amount | Amount captured (may be partial) |

### Hold Lifecycle

| State | Description |
|-------|-------------|
| ACTIVE | Hold placed, awaiting capture |
| CAPTURED | Funds claimed (fully or partially) |
| VOIDED | Hold released before capture |
| EXPIRED | Hold expired without capture |

---

## PaymentAttempt

Immutable record of a single authorization attempt. Each retry creates a new attempt.

### Fields

| Field | Description |
|-------|-------------|
| id | Unique attempt identifier |
| payment_intent_id | Parent payment intent |
| attempt_number | Sequential number (1, 2, 3...) |
| status | PENDING, PROCESSING, SUCCEEDED, FAILED |
| provider | Payment provider used |
| provider_response_code | Raw provider decline code |
| canonical_decline_code | Normalized decline code |
| decline_type | SOFT, HARD, FRAUD, TEMPORARY |
| processor_txn_id | Provider's transaction ID |

### Linear State Machine

```
     PENDING ----> PROCESSING ----> SUCCEEDED
                        |
                        +---------> FAILED

     (No backward transitions. New retry = new attempt record)
```

---

## Payment Lifecycle

The payment lifecycle distinguishes between authorization (promise) and capture (claim).

| Phase | Description | Money Movement | Reversible |
|-------|-------------|----------------|------------|
| Intent Created | Payment request received | None | Yes |
| Authorization | Issuer approves, places hold | Hold on funds | Yes (void) |
| Capture | Merchant claims funds | Initiates settlement | Limited (refund) |
| Settlement | Actual fund transfer | Funds move | No |

---

## Balance Model

Accounts maintain multiple balance states.

| Balance Type | Description | Calculation |
|--------------|-------------|-------------|
| Ledger Balance | Sum of all settled transactions | Immutable entry sum |
| Pending Balance | Authorized but unsettled | Active holds sum |
| Available Balance | Funds available for new transactions | Ledger - Pending debits |

---

## Canonical Event Model

All provider webhooks are normalized to canonical events.

### Event Type Mapping

| Canonical Type | Stripe | Adyen | PayPal |
|----------------|--------|-------|--------|
| AUTHORIZATION_SUCCEEDED | payment_intent.succeeded | AUTHORISATION (success) | PAYMENT.AUTHORIZATION.CREATED |
| AUTHORIZATION_FAILED | payment_intent.payment_failed | AUTHORISATION (fail) | PAYMENT.AUTHORIZATION.VOIDED |
| CAPTURE_SUCCEEDED | charge.captured | CAPTURE | PAYMENT.CAPTURE.COMPLETED |
| CAPTURE_FAILED | charge.failed | CAPTURE_FAILED | PAYMENT.CAPTURE.DENIED |
| REFUND_SUCCEEDED | charge.refunded | REFUND | PAYMENT.CAPTURE.REFUNDED |
| DISPUTE_OPENED | charge.dispute.created | CHARGEBACK | CUSTOMER.DISPUTE.CREATED |

### Canonical Decline Codes

| Canonical Code | Stripe | Adyen | PayPal |
|----------------|--------|-------|--------|
| INSUFFICIENT_FUNDS | insufficient_funds | Refused:51 | INSUFFICIENT_FUNDS |
| CARD_EXPIRED | expired_card | Refused:33 | CREDIT_CARD_EXPIRED |
| FRAUD_SUSPICION | fraudulent | Refused:59 | TRANSACTION_REFUSED |
| GENERIC_DECLINE | card_declined | Refused:05 | INSTRUMENT_DECLINED |

---

## Decline Categories

| Category | Examples | Retry Eligible | Resolution |
|----------|----------|----------------|------------|
| **Soft - Funds** | INSUFFICIENT_FUNDS, OVER_LIMIT | Yes | Wait for payday |
| **Soft - Generic** | DO_NOT_HONOR, GENERIC_DECLINE | Yes | Retry with timing variation |
| **Hard - Card** | CARD_EXPIRED, INVALID_NUMBER | No | Request new card |
| **Hard - Account** | ACCOUNT_CLOSED, RESTRICTED | No | Contact customer |
| **Fraud** | FRAUD_SUSPICION, STOLEN_CARD | No | Flag for review |

---

## Clearing Accounts

Track in-flight transactions for monitoring.

| Account | Purpose | Expected Balance |
|---------|---------|------------------|
| payment_clearing | Funds between authorization and capture | Near-zero |
| settlement_clearing | Funds awaiting bank settlement | Varies by settlement cycle |
| fee_clearing | Collected fees awaiting disbursement | Near-zero |

**Monitoring Rule:** Non-zero balances exceeding 24 hours indicate unresolved issues requiring investigation.

---

## See Also

- [System Design](system-design.md) - Architecture and data flow
- [Glossary](../glossary.md) - Term definitions
- [Database Schema](../schema/core-tables.md) - Table definitions
