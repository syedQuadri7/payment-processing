# System Design

High-level architecture and design principles for the Payment Processing Service.

---

## Architectural Principles

These principles are derived from studying systems processing billions of transactions.

### Principle 1: Separate Intent, Method, and Order

PaymentIntent, PaymentMethod, and Order are distinct entities. Payment methods can be attached, updated, or swapped without creating new intents.

### Principle 2: Authorization Before Capture

Authorization places a hold on customer funds. Capture claims those funds. These are separate operations with different lifecycles.

### Principle 3: Transactional Outbox

Events are written to an outbox table within the same database transaction. CDC relays events to Kafka asynchronously. Never dual-write.

### Principle 4: Linear State Machines

PaymentAttempt records are immutable. Each retry creates a new attempt with its own linear state progression.

### Principle 5: Clearing Account Monitoring

Non-zero clearing account balances exceeding thresholds trigger alerts indicating unresolved issues.

### Principle 6: Idempotency with Atomic Phases

Operations are broken into phases with recovery points. Each phase handles either local database operations OR external API calls, never both.

### Principle 7: Normalize at the Edge

Provider-specific logic exists only at system boundaries. The core system speaks a canonical language.

---

## High-Level Architecture

```
                    ADAPTER LAYER (Edge)
    +--------------------------------------------------+
    |  Stripe       Adyen        PayPal                |
    |  Webhook      Webhook      Webhook               |
    |     |            |            |                  |
    |     +------------+------------+                  |
    |                  |                               |
    |                  v                               |
    |          Canonical Event                         |
    +------------------+-------------------------------+
                       |
    +------------------+-------------------------------+
    |                  v           CORE LAYER          |
    |  +---------------------+  +-------------------+  |
    |  |     API Server      |  | Temporal Workflows|  |
    |  |                     +->|                   |  |
    |  |  POST /intents      |  |  PaymentWorkflow  |  |
    |  |  POST /authorize    |  |  RecoveryWorkflow |  |
    |  +---------------------+  +-------------------+  |
    +--------------------------------------------------+
                       |
    +------------------+-------------------------------+
    |                  v           DATA LAYER          |
    |  PostgreSQL              Kafka                   |
    |  +- payment_intents      +- payments.authorized  |
    |  +- authorization_holds  +- payments.captured    |
    |  +- payment_attempts     +- payments.failed      |
    |  +- ledger_entries                               |
    |  +- outbox ------------> (via Debezium CDC)      |
    +--------------------------------------------------+
```

---

## Layer Responsibilities

| Layer | Responsibility | Provider-Aware? |
|-------|----------------|-----------------|
| **Adapter Layer** | Receive webhooks, verify signatures, normalize to canonical | Yes (by design) |
| **API Layer** | REST endpoints, request validation, workflow orchestration | No |
| **Workflow Layer** | Business logic, state management, signal handling | No |
| **Activity Layer (PSP)** | Make outbound calls to payment providers | Yes (isolated) |
| **Activity Layer (Internal)** | Ledger, outbox, decline classification | No |
| **Data Layer** | Persistence, event publishing | No |

---

## Data Flow Patterns

### Inbound (Provider to System)

Provider sends webhook to adapter. Adapter verifies signature, parses event, maps to canonical model, and signals the workflow.

```
Provider Webhook -> Adapter -> Canonical Event -> Signal Workflow -> Process
```

### Outbound (System to Provider)

Workflow executes provider-specific activity. Activity calls provider API and returns canonical result.

```
Workflow -> Provider-Specific Activity -> Provider API -> Response -> Continue
```

### Events (System to Consumers)

State changes and outbox events are written in the same transaction. CDC captures changes and publishes to Kafka.

```
DB Transaction (state + outbox) -> CDC -> Kafka -> Consumers
```

---

## Adapter Layer

Each provider adapter handles:

| Responsibility | Description |
|----------------|-------------|
| Signature Verification | Provider-specific authentication |
| Event Parsing | Unmarshal provider's format |
| Event Mapping | Translate to canonical event type |
| Decline Code Mapping | Translate to canonical decline code |
| Payload Preservation | Store raw payload for debugging |
| Workflow Signaling | Send canonical event to workflow |
| Response Formatting | Return provider-expected acknowledgment |

### Provider Verification Methods

| Provider | Method |
|----------|--------|
| Stripe | HMAC-SHA256 with timestamp (reject if > 5 min old) |
| Adyen | HMAC-SHA256 using shared key |
| PayPal | Call PayPal API to verify webhook authenticity |

---

## Canonical Event Model

All provider webhooks are normalized to a canonical event before entering the workflow engine.

### Canonical Event Types

| Type | Description |
|------|-------------|
| AUTHORIZATION_SUCCEEDED | Authorization approved, hold placed |
| AUTHORIZATION_FAILED | Authorization declined |
| CAPTURE_SUCCEEDED | Capture confirmed |
| CAPTURE_FAILED | Capture failed |
| VOID_SUCCEEDED | Authorization voided |
| REFUND_SUCCEEDED | Refund processed |
| DISPUTE_OPENED | Chargeback initiated |

### Canonical Decline Codes

| Category | Examples |
|----------|----------|
| Soft (retry eligible) | INSUFFICIENT_FUNDS, OVER_LIMIT, GENERIC_DECLINE |
| Hard (not retryable) | CARD_EXPIRED, INVALID_NUMBER, ACCOUNT_CLOSED |
| Fraud | FRAUD_SUSPICION, STOLEN_CARD, LOST_CARD |

See [Glossary](../glossary.md) for full definitions.

---

## Transactional Outbox Pattern

Events are written to an outbox table within the same database transaction as business data.

```
Application
    |
    v
+-----------------------------------------------+
|              Single Transaction                |
|                                                |
|   1. UPDATE payment_intents SET status = ...  |
|   2. INSERT INTO ledger_entries (...)         |
|   3. INSERT INTO outbox (event_type, payload) |
|                                                |
|   COMMIT                                       |
+-----------------------------------------------+
    |
    v (Debezium reads WAL)
+-----------------------------------------------+
|           CDC Connector                        |
|                                                |
|   - Reads PostgreSQL write-ahead log          |
|   - Captures outbox table changes             |
|   - Publishes to Kafka topics                 |
+-----------------------------------------------+
    |
    v
+-----------------------------------------------+
|             Kafka Topics                       |
|                                                |
|   payments.authorized  (canonical events)     |
|   payments.captured    (canonical events)     |
|   payments.failed      (canonical events)     |
+-----------------------------------------------+
    |
    v
+-----------------------------------------------+
|          Idempotent Consumers                  |
|                                                |
|   - Track processed message IDs               |
|   - Handle duplicates gracefully              |
|   - Consumers never see provider-specific data|
+-----------------------------------------------+
```

---

## Simple vs Production Infrastructure

### Simple Path (Recommended for Getting Started)

Start here to focus on payment processing patterns without infrastructure complexity.

| Component | Simple Approach |
|-----------|-----------------|
| Event Publishing | Poll outbox table directly |
| Event Format | JSON with version field |
| Database | Direct PostgreSQL |
| Configuration | Environment variables |

### Production Path

Use when learning CDC patterns or simulating production infrastructure.

| Component | Production Approach |
|-----------|---------------------|
| Event Publishing | CDC with Debezium to Kafka |
| Event Format | Avro + Schema Registry |
| Database | PostgreSQL + PgBouncer |
| Configuration | Vault integration |

### Switching Paths

Both paths work with the same codebase:
- Environment variable: `EVENT_PUBLISHER=polling` vs `EVENT_PUBLISHER=kafka`
- Docker-compose profile: `--profile simple` vs `--profile production`

The core payment processing code is identical. Only the event publishing mechanism changes.

---

## Anti-Patterns to Avoid

| Anti-Pattern | Risk | Mitigation |
|--------------|------|------------|
| Card-first abstractions | Other payment methods don't fit | Separate Intent from Method |
| Provider-specific core logic | Tight coupling | Canonical model, adapter pattern |
| Circular state machines | Audit confusion | Linear states with attempt records |
| Dual-writes | Data loss, inconsistency | Transactional outbox pattern |
| Synchronous PSP handling only | Race conditions | Handle webhook + API concurrently |

---

## See Also

- [Domain Model](domain-model.md) - Entity relationships and state machines
- [Functional Requirements](../requirements/functional.md) - What the system does
- [Finalized Decisions](../decisions/finalized-decisions.md) - Implementation choices
