# ADR-003: Transactional Outbox Pattern

## Status

Accepted

## Context

Payment systems need to both update database state and publish events for downstream consumers. This creates the **dual-write problem**:

```
// DANGEROUS: Dual-write pattern
tx.Exec("UPDATE payments SET status = 'captured'")
tx.Commit()
kafka.Publish(PaymentCapturedEvent{...})  // What if this fails?
```

If the database commit succeeds but Kafka publish fails:
- Database shows payment captured
- Downstream systems never receive the event
- Data inconsistency between systems

Distributed transactions (2PC) are theoretically possible but:
- Complex to implement correctly
- Reduce system availability
- Often not supported across different systems
- Performance overhead

## Decision

We will use the **Transactional Outbox Pattern**:

1. Write events to an `outbox` table in the same database transaction as the business data
2. Use Change Data Capture (CDC) via Debezium to stream outbox changes to Kafka
3. Consumers must be idempotent (CDC guarantees at-least-once delivery)

### Implementation

```sql
-- Single atomic transaction
BEGIN;

-- Update business state
UPDATE payment_intents SET status = 'captured', captured_at = NOW()
WHERE id = $1;

-- Record ledger entries
INSERT INTO ledger_entries (account_id, amount, direction, ...)
VALUES ...;

-- Write event to outbox (same transaction)
INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload)
VALUES ('PaymentIntent', $1, 'payment.captured', $2);

COMMIT;
```

### Outbox Table Schema

```sql
CREATE TABLE outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type  VARCHAR(50) NOT NULL,
    aggregate_id    UUID NOT NULL,
    event_type      VARCHAR(50) NOT NULL,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Required for CDC to capture full row data
ALTER TABLE outbox REPLICA IDENTITY FULL;
```

### CDC Pipeline

```
PostgreSQL → Debezium → Kafka → Consumers
    │                     │
    └─ WAL changes ──────┘
```

Debezium reads PostgreSQL's Write-Ahead Log (WAL) and publishes changes to Kafka topics.

### Event Content

Events in the outbox contain **canonical data only**:

```json
{
  "event_type": "payment.captured",
  "payment_id": "pi_abc123",
  "amount": 10000,
  "currency": "USD",
  "provider": "STRIPE",
  "captured_at": "2026-01-17T10:40:00Z"
}
```

No provider-specific fields are included.

## Consequences

### Positive

- **Atomicity guaranteed**: Event and state change in same transaction
- **No data loss**: Database transaction succeeds or fails atomically
- **Low database load**: CDC reads WAL, minimal impact on production queries
- **Event ordering**: CDC preserves commit order
- **Replay capability**: Kafka retains events for configured retention period

### Negative

- **Eventual consistency**: Small delay between DB commit and Kafka delivery
- **Infrastructure complexity**: Requires Debezium connector and Kafka
- **Consumer idempotency required**: At-least-once delivery means duplicates possible
- **Outbox table growth**: Must periodically clean up processed events

### Mitigations

- CDC latency typically < 100ms, acceptable for most consumers
- Consumers track processed message IDs to handle duplicates
- Run periodic cleanup job to archive/delete old outbox records
- Monitor CDC lag as key operational metric

## Consumer Idempotency

All consumers must be idempotent:

```go
func (c *Consumer) Handle(event PaymentCapturedEvent) error {
    // Check if already processed
    if c.repo.EventProcessed(event.ID) {
        return nil // Already handled
    }

    // Process event
    err := c.processEvent(event)
    if err != nil {
        return err
    }

    // Mark as processed
    c.repo.MarkProcessed(event.ID)
    return nil
}
```

## Alternatives Considered

### 1. Direct Kafka Publishing

- **Approach**: Publish to Kafka directly after DB commit
- **Problem**: Fails if Kafka is unavailable after commit
- **Rejected**: Violates atomicity requirement

### 2. Polling Publisher

- **Approach**: Background job polls outbox table and publishes
- **Problem**: Polling overhead, ordering challenges
- **Rejected**: CDC is more efficient and preserves ordering

### 3. Saga with Compensation

- **Approach**: Publish first, compensate if DB fails
- **Problem**: Complex compensation logic, eventual consistency issues
- **Rejected**: Outbox is simpler for this use case

## References

- [Transactional Outbox Pattern](https://microservices.io/patterns/data/transactional-outbox.html)
- [Debezium Documentation](https://debezium.io/documentation/)
- [CDC with PostgreSQL](https://debezium.io/documentation/reference/connectors/postgresql.html)
