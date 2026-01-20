# ADR-003: Transactional Outbox Pattern

## Status

Accepted

## Problem

Payment systems need to both update database state and publish events for downstream consumers. When a payment is captured, we must update the payment status in the database AND publish a "payment captured" event.

**The core challenge**: This creates the dual-write problem. If the database commit succeeds but the event publish fails, we have inconsistency: the database shows the payment captured, but downstream systems never receive the event.

| Failure Scenario | Consequence |
|------------------|-------------|
| DB commits, Kafka publish fails | Database says captured, consumers never notified |
| Kafka publishes, DB commit fails | Consumers think captured, but it wasn't |
| Partial failure during either | Unknown state, requires manual reconciliation |

## Solutions Considered

### Solution A: Direct Event Publishing

Publish to Kafka directly after database commit.

| Pros | Cons |
|------|------|
| Simple implementation | Fails if Kafka unavailable after commit |
| Low latency | No atomicity guarantee |
| No additional infrastructure | Lost events if publish fails |
| | Requires manual retry logic |

### Solution B: Polling Publisher

Write events to outbox table. Background job polls table and publishes to Kafka.

| Pros | Cons |
|------|------|
| Atomic write (same transaction) | Polling overhead and latency |
| No lost events | Ordering challenges with multiple pollers |
| Simple to understand | Delivery delay based on poll interval |
| | Table can grow large |

### Solution C: Transactional Outbox with CDC

Write events to outbox table in same transaction. Use Change Data Capture (Debezium) to stream changes to Kafka.

| Pros | Cons |
|------|------|
| Atomic write (same transaction) | Requires Debezium infrastructure |
| Low latency (WAL-based) | Eventual consistency (small delay) |
| Preserves commit order | Consumer idempotency required |
| Minimal database load | Outbox table needs cleanup |
| No polling overhead | |

### Solution D: Saga with Compensation

Publish event first, then update database. If database fails, publish compensation event.

| Pros | Cons |
|------|------|
| Event published immediately | Complex compensation logic |
| Works across different systems | Eventual consistency issues |
| | Consumers see event then compensation |
| | Difficult to reason about |

## Chosen Solution

**Solution C: Transactional Outbox with CDC**

### How It Works

| Step | Action |
|------|--------|
| 1. Begin transaction | Start database transaction |
| 2. Update business state | Update payment status, record ledger entries |
| 3. Write to outbox | Insert event record in outbox table |
| 4. Commit transaction | Atomic commit of all changes |

If any step fails, the entire transaction rolls back. No partial state.

### CDC Pipeline

| Stage | Component | Action |
|-------|-----------|--------|
| Source | PostgreSQL | Writes events to outbox table |
| Capture | Debezium | Reads Write-Ahead Log (WAL) |
| Transport | Kafka | Receives and stores events |
| Consumer | Downstream services | Process events idempotently |

Debezium reads PostgreSQL's WAL and publishes changes to Kafka, preserving commit order.

### Event Content Requirements

Events in the outbox contain canonical data only:

| Field | Required | Description |
|-------|----------|-------------|
| Event ID | Yes | Unique identifier for idempotency |
| Event Type | Yes | Canonical event type (e.g., payment.captured) |
| Entity ID | Yes | ID of the affected entity |
| Amount | If applicable | Amount in smallest currency unit |
| Currency | If applicable | ISO 4217 currency code |
| Provider | If applicable | Payment provider used |
| Timestamp | Yes | When the event occurred |
| Correlation ID | Yes | For distributed tracing |

No provider-specific fields in outbox events.

### Infrastructure Requirements

| Component | Requirement |
|-----------|-------------|
| PostgreSQL | REPLICA IDENTITY FULL on outbox table |
| Debezium | PostgreSQL connector configured for outbox table |
| Kafka | Topic per event type or single topic with routing |
| CDC User | Database user with replication permissions |

## Why This Solution

| Reason | Explanation |
|--------|-------------|
| **Atomicity** | Event and state change are in the same database transaction. They succeed or fail together. |
| **No data loss** | If the transaction commits, the event is guaranteed to be published (CDC reads committed WAL). |
| **Low latency** | CDC typically has < 100ms delay from commit to Kafka. Much faster than polling. |
| **Order preservation** | CDC maintains commit order. Events arrive in the order transactions committed. |
| **Low database load** | CDC reads WAL, not production tables. Minimal impact on query performance. |
| **Replay capability** | Kafka retains events. Consumers can replay from any offset. |

### Trade-off Acceptance

| Trade-off | Mitigation |
|-----------|------------|
| Eventual consistency | CDC latency is typically < 100ms, acceptable for most consumers |
| Infrastructure complexity | Debezium is well-documented; operational runbooks exist |
| Consumer idempotency required | Consumers track processed event IDs; standard pattern |
| Outbox table growth | Periodic cleanup job archives/deletes old records |

### Consumer Idempotency Requirements

All consumers must be idempotent:

| Step | Action |
|------|--------|
| 1. Check | Query if event ID has been processed |
| 2. Skip | If already processed, return success without reprocessing |
| 3. Process | If new, process the event |
| 4. Record | Mark event ID as processed |

## References

- [Transactional Outbox Pattern](https://microservices.io/patterns/data/transactional-outbox.html)
- [Debezium Documentation](https://debezium.io/documentation/)
- [CDC with PostgreSQL](https://debezium.io/documentation/reference/connectors/postgresql.html)
