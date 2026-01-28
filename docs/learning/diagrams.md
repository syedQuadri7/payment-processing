# System Diagrams

Visual representations of key concepts in the payment processing system.

---

## 1. PaymentIntent State Machine

The core state machine. Every payment intent follows these paths.

```
                              +------------------+
                              |     CREATED      |
                              +------------------+
                                       |
              +------------------------+------------------------+
              |                        |                        |
              v                        v                        v
    +------------------+     +------------------+      +------------------+
    | REQUIRES_METHOD  |---->|  REQUIRES_AUTH   |      |    CANCELLED     |
    +------------------+     +------------------+      +------------------+
              |                        |                      ^ (terminal)
              |                        |                      |
              +------------+-----------+                      |
                           |                                  |
                           v                                  |
                  +------------------+                        |
                  |    AUTHORIZED    |------------------------+
                  +------------------+
                           |
          +----------------+----------------+
          |                                 |
          v                                 v
+------------------+              +------------------+
|    CAPTURED      |              |     VOIDED       |
+------------------+              +------------------+
     (terminal)                        (terminal)
          |
          v (on failure)
+------------------+              +------------------+
|   RECOVERING     |<-------------|     FAILED       |
+------------------+              +------------------+
          |                            (terminal)
          |
          +---> back to AUTHORIZED (on retry success)
          +---> to FAILED (max retries exceeded)
```

**Terminal States** (no further transitions):
- `CAPTURED` - Payment complete, funds collected
- `VOIDED` - Authorization released, no funds collected
- `CANCELLED` - Intent abandoned before authorization
- `FAILED` - Payment permanently failed

---

## 2. Happy Path: Authorization and Capture

The most common successful payment flow.

```
Customer                    API                      Workflow                   Provider
   |                         |                          |                          |
   |  POST /intents          |                          |                          |
   |------------------------>|                          |                          |
   |                         |  Create PaymentIntent    |                          |
   |                         |  Status: CREATED         |                          |
   |  <----------------------|                          |                          |
   |                         |                          |                          |
   |  PUT /intents/:id/method|                          |                          |
   |------------------------>|                          |                          |
   |                         |  Attach PaymentMethod    |                          |
   |  <----------------------|                          |                          |
   |                         |                          |                          |
   |  POST /intents/:id/auth |                          |                          |
   |------------------------>|                          |                          |
   |                         |  Start Workflow          |                          |
   |                         |------------------------->|                          |
   |                         |                          |  Authorize               |
   |                         |                          |------------------------->|
   |                         |                          |  <-----------------------|
   |                         |                          |  Authorization approved  |
   |                         |                          |                          |
   |                         |                          |  Update DB:              |
   |                         |                          |  - Status: AUTHORIZED    |
   |                         |                          |  - Create AuthHold       |
   |                         |                          |  - Write to Outbox       |
   |  <---------------------------------------------------------|                  |
   |  Status: AUTHORIZED     |                          |                          |
   |                         |                          |                          |
   |  POST /intents/:id/capture                         |                          |
   |------------------------>|                          |                          |
   |                         |  Signal Workflow         |                          |
   |                         |------------------------->|                          |
   |                         |                          |  Capture                 |
   |                         |                          |------------------------->|
   |                         |                          |  <-----------------------|
   |                         |                          |  Capture confirmed       |
   |                         |                          |                          |
   |                         |                          |  Update DB:              |
   |                         |                          |  - Status: CAPTURED      |
   |                         |                          |  - Write Ledger entries  |
   |                         |                          |  - Write to Outbox       |
   |  <---------------------------------------------------------|                  |
   |  Status: CAPTURED       |                          |                          |
```

---

## 3. Authorization Hold Lifecycle

Tracks the "hold" on customer funds between auth and capture.

```
                    +------------------+
                    |      ACTIVE      |
                    |                  |
                    | (funds on hold)  |
                    +------------------+
                             |
        +--------------------+--------------------+
        |                    |                    |
        v                    v                    v
+---------------+    +---------------+    +---------------+
|   CAPTURED    |    |    VOIDED     |    |   EXPIRED     |
| (funds taken) |    | (hold released|    | (hold released|
+---------------+    |  by merchant) |    |  by timeout)  |
                     +---------------+    +---------------+

Timeline:
  |--- Auth approved ---|--- Hold window (typically 7 days) ---|--- Expires ---|
                        ^                                       ^
                        |                                       |
                   Can capture                             Auto-expires
                   or void here                            if not captured
```

---

## 4. Payment Attempt (Linear, Immutable)

Each retry creates a NEW attempt. No loops, no state changes on existing records.

```
Attempt #1                 Attempt #2                 Attempt #3
+------------------+       +------------------+       +------------------+
| ID: att_001      |       | ID: att_002      |       | ID: att_003      |
| Status: FAILED   |       | Status: FAILED   |       | Status: SUCCEEDED|
| Decline: SOFT    |       | Decline: SOFT    |       |                  |
| (insufficient $) |       | (try again later)|       |                  |
+------------------+       +------------------+       +------------------+
         |                          |                          |
         v                          v                          v
    Created Day 1             Created Day 3             Created Day 5


Why linear?
- Full audit trail (every attempt preserved)
- Analytics (what decline codes, how many retries)
- No ambiguous "current" state confusion
- Each attempt is immutable once terminal
```

---

## 5. Transactional Outbox Pattern

Solves the "dual write" problem - what if DB succeeds but message queue fails?

```
WITHOUT Outbox (Dangerous):
+----------------+                +----------------+
|   Database     |                |  Message Queue |
+----------------+                +----------------+
        |                                 |
   1. UPDATE                         2. PUBLISH
   payment_intents                   event
   SET status=X                           |
        |                                 X <-- What if this fails?
        v                                      DB is updated but
   Committed!                                  event never sent!


WITH Outbox (Safe):
+----------------+
|   Database     |    Single Transaction:
+----------------+    +------------------------------------------+
        |             | 1. UPDATE payment_intents SET status=X   |
        |             | 2. INSERT INTO outbox (event_type, ...)  |
        |             +------------------------------------------+
        v                       |
   Both committed               |
   atomically!                  |
                                v
                    +------------------------+
                    |    Outbox Consumer     |
                    | (polls outbox table)   |
                    +------------------------+
                                |
                                v
                    +------------------------+
                    |    Message Queue       |
                    |    (Kafka, etc.)       |
                    +------------------------+

If consumer crashes? No problem - event stays in outbox, gets picked up on restart.
If event published twice? Consumers must be idempotent (processed_events table).
```

---

## 6. Webhook Flow Through Adapters

External provider events normalized at the boundary.

```
Stripe                     Adyen                      PayPal
  |                          |                          |
  | POST /webhooks/stripe    | POST /webhooks/adyen     | POST /webhooks/paypal
  v                          v                          v
+------------------+   +------------------+   +------------------+
| Stripe Adapter   |   | Adyen Adapter    |   | PayPal Adapter   |
|                  |   |                  |   |                  |
| 1. Verify sig    |   | 1. Verify HMAC   |   | 1. Verify w/API  |
| 2. Parse event   |   | 2. Parse event   |   | 2. Parse event   |
| 3. Map decline   |   | 3. Map decline   |   | 3. Map decline   |
+------------------+   +------------------+   +------------------+
         |                      |                      |
         v                      v                      v
    +----------------------------------------------------------+
    |                   Canonical Event                         |
    |                                                           |
    |  Type: AUTHORIZATION_SUCCEEDED | CAPTURE_FAILED | etc.    |
    |  DeclineCode: INSUFFICIENT_FUNDS | CARD_EXPIRED | etc.    |
    |  PaymentIntentID: pi_xxx                                  |
    +----------------------------------------------------------+
                                |
                                v
                    +------------------------+
                    |   Temporal Workflow    |
                    |   (receives signal)    |
                    +------------------------+


Why normalize at the edge?
- Core logic is provider-agnostic
- Add new provider = add adapter, core unchanged
- Test with fake adapters
- Single source of truth for event handling
```

---

## 7. Decline Code Classification

How provider-specific declines map to retry behavior.

```
Provider Response                    Canonical Code              Action
-----------------                    --------------              ------

Stripe: "insufficient_funds"    -->  INSUFFICIENT_FUNDS     -->  RETRY (soft)
Adyen:  "Refused:Not enough"    -->  INSUFFICIENT_FUNDS     -->  RETRY (soft)
PayPal: "INSUFFICIENT_FUNDS"    -->  INSUFFICIENT_FUNDS     -->  RETRY (soft)

Stripe: "card_declined"         -->  GENERIC_DECLINE        -->  RETRY (soft)
Stripe: "expired_card"          -->  CARD_EXPIRED           -->  NO RETRY (hard)
Stripe: "fraudulent"            -->  FRAUD_SUSPICION        -->  NO RETRY (fraud)


+------------------+     +------------------+     +------------------+
|   SOFT DECLINE   |     |   HARD DECLINE   |     |  FRAUD DECLINE   |
|                  |     |                  |     |                  |
| - Insufficient $ |     | - Card expired   |     | - Fraud detected |
| - Over limit     |     | - Invalid number |     | - Stolen card    |
| - Try again      |     | - Invalid CVC    |     | - Lost card      |
| - Processor err  |     | - Account closed |     | - Pickup card    |
|                  |     |                  |     |                  |
| ACTION: Retry    |     | ACTION: Stop     |     | ACTION: Stop +   |
| with backoff     |     | inform customer  |     | flag for review  |
+------------------+     +------------------+     +------------------+
```

---

## 8. Double-Entry Ledger

Every money movement has equal debits and credits.

```
Authorization (hold funds):
+------------------------+------------------------+
|        DEBIT           |        CREDIT          |
+------------------------+------------------------+
| Customer Liability     | Payment Clearing       |
| +$100.00               | +$100.00               |
+------------------------+------------------------+
                    Sum = $0 (balanced!)


Capture (claim funds):
+------------------------+------------------------+
|        DEBIT           |        CREDIT          |
+------------------------+------------------------+
| Payment Clearing       | Merchant Receivable    |
| +$100.00               | +$100.00               |
+------------------------+------------------------+
                    Sum = $0 (balanced!)


Void (release hold):
+------------------------+------------------------+
|        DEBIT           |        CREDIT          |
+------------------------+------------------------+
| Payment Clearing       | Customer Liability     |
| +$100.00               | +$100.00               |
+------------------------+------------------------+
                    Sum = $0 (balanced!)


Why double-entry?
- Mathematical proof of correctness (debits = credits always)
- Catches bugs (unbalanced entry = something wrong)
- Full audit trail
- Easy reconciliation
```

---

## 9. System Architecture Overview

How the services fit together.

```
                                  External
                                  Providers
                                     |
                                     | Webhooks
                                     v
+------------------------------------------------------------------+
|                                                                   |
|    +------------------+                    +------------------+   |
|    |   payment-api    |                    | provider-sim     |   |
|    |                  |                    | (testing only)   |   |
|    | - REST endpoints |                    |                  |   |
|    | - Webhook ingest |                    | - Fake Stripe    |   |
|    | - Auth/Rate limit|                    | - Fake Adyen     |   |
|    +------------------+                    | - Fake PayPal    |   |
|            |                               +------------------+   |
|            | Start workflow /                                     |
|            | Send signal                                          |
|            v                                                      |
|    +------------------+                                           |
|    | payment-worker   |                                           |
|    |                  |                                           |
|    | - Temporal       |                                           |
|    |   workflows      |                                           |
|    | - Activities     |                                           |
|    +------------------+                                           |
|            |                                                      |
|            | Read/Write                                           |
|            v                                                      |
|    +------------------+         +------------------+              |
|    |   PostgreSQL     |         |    Temporal      |              |
|    |                  |         |    Server        |              |
|    | - payment_intents|         |                  |              |
|    | - ledger entries |         | - Workflow state |              |
|    | - outbox         |         | - Task queues    |              |
|    | - audit_log      |         | - History        |              |
|    +------------------+         +------------------+              |
|                                                                   |
+------------------------------------------------------------------+

shared/
  domain/      <-- Core types (used by all services)
  adapter/     <-- Webhook handlers
  repository/  <-- Database access
```

---

## 10. Request Flow: POST /intents/:id/authorize

Trace a single API call through the system.

```
HTTP Request
     |
     v
+--------------------+
| Middleware Stack   |
| - RequestID        |
| - Correlation      |
| - Auth             |
| - RateLimit        |
| - Idempotency      |
+--------------------+
     |
     v
+--------------------+
| Handler:           |
| AuthorizeIntent()  |
+--------------------+
     |
     | 1. Validate request
     | 2. Load PaymentIntent from DB
     | 3. Check CanAuthorize()
     |
     v
+--------------------+
| Temporal Client    |
| StartWorkflow() or |
| SignalWorkflow()   |
+--------------------+
     |
     | (async - returns immediately)
     |
     v
+--------------------+      +--------------------+
| HTTP Response      |      | Workflow executes  |
| 202 Accepted       |      | in background...   |
+--------------------+      +--------------------+
                                    |
                                    v
                            +--------------------+
                            | Activity:          |
                            | CallProvider()     |
                            +--------------------+
                                    |
                                    v
                            +--------------------+
                            | Activity:          |
                            | UpdatePaymentIntent|
                            | WriteLedgerEntry   |
                            | WriteOutboxEvent   |
                            +--------------------+
                                    |
                                    | (single DB transaction)
                                    v
                            +--------------------+
                            | PostgreSQL         |
                            +--------------------+
```
