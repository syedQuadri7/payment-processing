# Functional Requirements

What the Payment Processing Service does.

---

## Payment Intent Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-INT-01 | System shall accept payment intent creation via REST API | Must Have |
| FR-INT-02 | System shall validate payment intent before processing | Must Have |
| FR-INT-03 | Payment intent shall be independent of payment method | Must Have |
| FR-INT-04 | System shall support updating payment method on existing intent | Must Have |
| FR-INT-05 | System shall support payment intent cancellation before capture | Must Have |
| FR-INT-06 | System shall use idempotency keys with atomic phases | Must Have |
| FR-INT-07 | System shall support attaching metadata to payment intents | Should Have |
| FR-INT-08 | Payment intent shall specify which provider to use | Must Have |

---

## Authorization and Capture

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-AUTH-01 | System shall request authorization from payment processor | Must Have |
| FR-AUTH-02 | System shall track authorization holds as first-class entities | Must Have |
| FR-AUTH-03 | System shall support separate authorization and capture | Must Have |
| FR-AUTH-04 | System shall support immediate capture (auth + capture) | Must Have |
| FR-AUTH-05 | System shall track authorization expiration windows | Should Have |
| FR-AUTH-06 | System shall support single partial capture (Stripe-style) | Should Have |
| FR-AUTH-07 | System shall support void of uncaptured authorizations | Should Have |
| FR-AUTH-08 | System shall use provider-specific activities for PSP calls | Must Have |

---

## Multi-Provider Adapter Layer

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-ADP-01 | System shall provide dedicated webhook endpoints per provider | Must Have |
| FR-ADP-02 | Each adapter shall verify provider-specific signatures | Must Have |
| FR-ADP-03 | Adapters shall normalize events to canonical model before processing | Must Have |
| FR-ADP-04 | Adapters shall preserve raw webhook payload for debugging | Must Have |
| FR-ADP-05 | Adapters shall map provider decline codes to canonical codes | Must Have |
| FR-ADP-06 | Adapters shall map provider event types to canonical event types | Must Have |
| FR-ADP-07 | Adapter failures shall return appropriate HTTP status to provider | Must Have |
| FR-ADP-08 | System shall support adding new providers without core changes | Should Have |

### Provider Verification Methods

| Provider | Verification Method |
|----------|---------------------|
| Stripe | Webhook signature with timestamp (verify within 5 minutes) |
| Adyen | HMAC-SHA256 using shared secret |
| PayPal | Webhook ID verification API call |

---

## Decline Handling and Recovery

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-DEC-01 | System shall classify declines into categories (soft, hard, fraud) | Must Have |
| FR-DEC-02 | System shall maintain decline code mapping per provider | Must Have |
| FR-DEC-03 | System shall normalize provider codes to canonical codes | Must Have |
| FR-DEC-04 | System shall automatically retry soft declines with intelligent timing | Must Have |
| FR-DEC-05 | System shall not retry hard declines or fraud flags | Must Have |
| FR-DEC-06 | System shall create new attempt records for each retry | Must Have |
| FR-DEC-07 | System shall limit retry attempts to configurable maximum (default: 6) | Must Have |
| FR-DEC-08 | System shall use fixed retry intervals (4h, 12h, 24h, 48h) | Should Have |
| FR-DEC-09 | System shall support immediate retry when payment method updated | Should Have |

---

## Ledger and Bookkeeping

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-LED-01 | System shall use double-entry bookkeeping for all transactions | Must Have |
| FR-LED-02 | System shall maintain clearing accounts for in-flight transactions | Must Have |
| FR-LED-03 | System shall ensure every debit has a corresponding credit | Must Have |
| FR-LED-04 | System shall prevent ledger entry modification or deletion | Must Have |
| FR-LED-05 | System shall track ledger, pending, and available balances | Must Have |
| FR-LED-06 | System shall monitor clearing account balances | Must Have |
| FR-LED-07 | System shall support balance reconciliation queries | Should Have |

---

## Event Publishing

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-EVT-01 | System shall use transactional outbox pattern for event publishing | Must Have |
| FR-EVT-02 | System shall never dual-write to database and message broker | Must Have |
| FR-EVT-03 | System shall support CDC (Debezium) to relay outbox events to Kafka | Must Have |
| FR-EVT-04 | System shall publish canonical events (not provider-specific) | Must Have |
| FR-EVT-05 | System shall support simple outbox polling as alternative to CDC | Should Have |
| FR-EVT-06 | System shall support event replay for consumer recovery | Should Have |
| FR-EVT-07 | Event consumers shall be idempotent | Must Have |

---

## Concurrency and Race Conditions

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-CON-01 | System shall handle simultaneous webhook and API responses safely | Must Have |
| FR-CON-02 | System shall use SELECT FOR UPDATE when processing payment updates | Must Have |
| FR-CON-03 | System shall use optimistic locking with version columns for balances | Must Have |
| FR-CON-04 | System shall prevent duplicate charges from concurrent processing | Must Have |
| FR-CON-05 | System shall handle provider webhook retries idempotently | Must Have |

---

## Scheduled Payments

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-SCH-01 | System shall support one-time future-dated payments | Should Have |
| FR-SCH-02 | System shall support recurring payments (weekly, biweekly, monthly) | Should Have |
| FR-SCH-03 | System shall verify account status before executing scheduled payments | Should Have |
| FR-SCH-04 | System shall support cancellation of scheduled payments | Should Have |

---

## Audit and Compliance

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-AUD-01 | System shall log all payment state changes to audit log | Must Have |
| FR-AUD-02 | System shall record actor information for all changes | Must Have |
| FR-AUD-03 | System shall preserve original and new values for updates | Must Have |
| FR-AUD-04 | System shall timestamp all audit entries with microsecond precision | Must Have |
| FR-AUD-05 | System shall support audit log querying by entity, actor, or time range | Should Have |
| FR-AUD-06 | Audit log shall be append-only (no updates or deletes) | Must Have |
| FR-AUD-07 | System shall log provider and correlation IDs for webhook tracing | Must Have |
| FR-AUD-08 | System shall retain audit logs for 90 days in database | Must Have |

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/v1/intents | Create payment intent |
| GET | /api/v1/intents/:id | Get intent status |
| PUT | /api/v1/intents/:id/method | Attach payment method |
| POST | /api/v1/intents/:id/authorize | Request authorization |
| POST | /api/v1/intents/:id/capture | Capture authorized funds |
| POST | /api/v1/intents/:id/cancel | Cancel payment intent |
| GET | /api/v1/intents/:id/attempts | Get attempt history |
| GET | /api/v1/intents/:id/hold | Get authorization hold status |
| POST | /webhooks/stripe | Receive Stripe webhooks |
| POST | /webhooks/adyen | Receive Adyen notifications |
| POST | /webhooks/paypal | Receive PayPal webhooks |
| GET | /health | Health check |
| GET | /metrics | Prometheus metrics |

---

## User Stories

### US-01: Create a payment intent

> As an API consumer, I want to create a payment intent specifying the provider so that I can begin the payment process.

**Acceptance Criteria:**
- Payment intent created with amount, currency, customer, and provider
- Intent status is CREATED or REQUIRES_METHOD
- Idempotency key prevents duplicate intents
- Intent ID returned for subsequent operations

### US-02: Attach payment method to intent

> As an API consumer, I want to attach a payment method to an intent so that I can proceed to authorization.

**Acceptance Criteria:**
- Payment method can be attached via PUT request
- Intent status transitions to REQUIRES_AUTH
- Payment method can be updated until authorization
- Invalid payment methods rejected with clear error

### US-03: Receive Stripe webhook

> As the system, I want to receive and process Stripe webhooks so that payment status is updated.

**Acceptance Criteria:**
- Stripe signature verified before processing
- Event mapped to canonical model
- Existing workflow signaled with canonical event
- Return 200 to Stripe within timeout

### US-04: Receive Adyen notification

> As the system, I want to receive and process Adyen notifications so that payment status is updated.

**Acceptance Criteria:**
- HMAC signature verified before processing
- Notification mapped to canonical model
- Existing workflow signaled with canonical event
- Return "[accepted]" to Adyen

### US-05: Automatic retry on soft decline

> As a business, I want failed payments automatically retried so that I recover more revenue.

**Acceptance Criteria:**
- Provider decline codes mapped to canonical codes
- Soft declines classified correctly
- New PaymentAttempt record created for each retry
- Retry timing follows fixed intervals (4h, 12h, 24h, 48h)
- Maximum attempts enforced

---

## See Also

- [Non-Functional Requirements](non-functional.md) - Performance, reliability, security
- [Domain Model](../architecture/domain-model.md) - Entity definitions
- [Finalized Decisions](../decisions/finalized-decisions.md) - Implementation choices
