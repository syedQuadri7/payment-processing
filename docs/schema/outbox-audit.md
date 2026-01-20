# Outbox and Audit Tables

Database tables for event publishing and audit trails.

## outbox

Implements the transactional outbox pattern for reliable event publishing.

```sql
CREATE TABLE outbox (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type    VARCHAR(50) NOT NULL,
    aggregate_id      UUID NOT NULL,
    event_type        VARCHAR(50) NOT NULL,
    payload           JSONB NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Required for Debezium CDC to capture full row data
ALTER TABLE outbox REPLICA IDENTITY FULL;

CREATE INDEX idx_outbox_created ON outbox(created_at);
CREATE INDEX idx_outbox_aggregate ON outbox(aggregate_type, aggregate_id);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key, used for deduplication |
| `aggregate_type` | VARCHAR(50) | Type of entity (PaymentIntent, Account) |
| `aggregate_id` | UUID | ID of the entity |
| `event_type` | VARCHAR(50) | Canonical event type |
| `payload` | JSONB | Canonical event data |
| `created_at` | TIMESTAMPTZ | Event creation time |

### Event Types (Canonical)

| Event Type | Description |
|------------|-------------|
| `payment.created` | Payment intent created |
| `payment.authorized` | Authorization succeeded |
| `payment.authorization_failed` | Authorization declined |
| `payment.captured` | Capture succeeded |
| `payment.capture_failed` | Capture failed |
| `payment.voided` | Authorization voided |
| `payment.refunded` | Refund processed |
| `payment.dispute_opened` | Chargeback initiated |
| `account.balance_updated` | Account balance changed |

### Payload Format (Canonical)

```json
{
  "event_id": "evt_abc123",
  "event_type": "payment.captured",
  "payment_id": "pi_xyz789",
  "amount": 10000,
  "currency": "USD",
  "provider": "STRIPE",
  "captured_at": "2026-01-17T10:40:00Z",
  "correlation_id": "req_abc123"
}
```

**Important**: Payload contains only canonical fields. No provider-specific data.

### CDC Configuration

Debezium connector reads outbox table changes from PostgreSQL WAL:

```json
{
  "name": "payment-outbox-connector",
  "config": {
    "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
    "database.hostname": "postgres",
    "database.port": "5432",
    "database.user": "debezium",
    "database.password": "${DB_PASSWORD}",
    "database.dbname": "payments",
    "table.include.list": "public.outbox",
    "transforms": "outbox",
    "transforms.outbox.type": "io.debezium.transforms.outbox.EventRouter",
    "transforms.outbox.table.fields.additional.placement": "event_type:header:eventType"
  }
}
```

### Cleanup Strategy

Old outbox records can be archived/deleted after confirmation of Kafka delivery:

```sql
-- Archive outbox records older than 7 days
INSERT INTO outbox_archive
SELECT * FROM outbox WHERE created_at < NOW() - INTERVAL '7 days';

DELETE FROM outbox WHERE created_at < NOW() - INTERVAL '7 days';
```

---

## audit_log

Append-only audit trail of all significant changes. Required for compliance.

```sql
CREATE TABLE audit_log (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type       VARCHAR(50) NOT NULL,
    entity_id         UUID NOT NULL,
    action            VARCHAR(20) NOT NULL,
    actor_type        VARCHAR(20) NOT NULL,
    actor_id          VARCHAR(100),
    old_values        JSONB,
    new_values        JSONB,
    metadata          JSONB,
    ip_address        INET,
    user_agent        TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit_log(actor_type, actor_id);
CREATE INDEX idx_audit_action ON audit_log(action);
CREATE INDEX idx_audit_created ON audit_log(created_at);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `entity_type` | VARCHAR(50) | Type of entity changed |
| `entity_id` | UUID | ID of entity changed |
| `action` | VARCHAR(20) | CREATE, UPDATE, DELETE |
| `actor_type` | VARCHAR(20) | SYSTEM, USER, WEBHOOK, API |
| `actor_id` | VARCHAR(100) | ID of actor (user ID, API key, etc.) |
| `old_values` | JSONB | Previous field values |
| `new_values` | JSONB | New field values |
| `metadata` | JSONB | Additional context |
| `ip_address` | INET | Client IP address |
| `user_agent` | TEXT | Client user agent |
| `created_at` | TIMESTAMPTZ | Audit timestamp |

### Entity Types

| Type | Description |
|------|-------------|
| `PAYMENT_INTENT` | Payment intent changes |
| `PAYMENT_METHOD` | Payment method changes |
| `ACCOUNT` | Account balance/status changes |
| `AUTHORIZATION_HOLD` | Hold lifecycle events |

### Action Types

| Action | Description |
|--------|-------------|
| `CREATE` | Entity created |
| `UPDATE` | Entity modified |
| `DELETE` | Entity deleted |
| `STATUS_CHANGE` | Status transition |

### Actor Types

| Type | Description |
|--------|-------------|
| `SYSTEM` | Internal system operation |
| `USER` | Authenticated user action |
| `WEBHOOK` | Provider webhook |
| `API` | API client |
| `WORKFLOW` | Temporal workflow |

### Example Audit Entries

**Payment Intent Created:**
```json
{
  "entity_type": "PAYMENT_INTENT",
  "entity_id": "pi_abc123",
  "action": "CREATE",
  "actor_type": "API",
  "actor_id": "apikey_xyz",
  "new_values": {
    "amount": 10000,
    "currency": "USD",
    "status": "CREATED",
    "provider": "STRIPE"
  },
  "metadata": {
    "idempotency_key": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

**Status Change from Webhook:**
```json
{
  "entity_type": "PAYMENT_INTENT",
  "entity_id": "pi_abc123",
  "action": "STATUS_CHANGE",
  "actor_type": "WEBHOOK",
  "actor_id": "stripe",
  "old_values": {
    "status": "REQUIRES_AUTH"
  },
  "new_values": {
    "status": "AUTHORIZED"
  },
  "metadata": {
    "webhook_event_id": "evt_stripe_123",
    "provider_payment_id": "pi_stripe_xyz"
  }
}
```

### Querying Audit Log

```sql
-- All changes to a specific payment
SELECT * FROM audit_log
WHERE entity_type = 'PAYMENT_INTENT' AND entity_id = :payment_id
ORDER BY created_at;

-- All actions by a specific actor
SELECT * FROM audit_log
WHERE actor_type = 'USER' AND actor_id = :user_id
ORDER BY created_at DESC
LIMIT 100;

-- Status changes in time range
SELECT * FROM audit_log
WHERE action = 'STATUS_CHANGE'
  AND created_at BETWEEN :start_date AND :end_date
ORDER BY created_at;
```

---

## processed_events

Tracks processed webhook events for idempotency.

```sql
CREATE TABLE processed_events (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider          VARCHAR(20) NOT NULL,
    event_id          VARCHAR(100) NOT NULL,
    event_type        VARCHAR(50) NOT NULL,
    processed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_processed_event UNIQUE (provider, event_id)
);

CREATE INDEX idx_processed_provider ON processed_events(provider);
CREATE INDEX idx_processed_at ON processed_events(processed_at);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `provider` | VARCHAR(20) | STRIPE, ADYEN, PAYPAL |
| `event_id` | VARCHAR(100) | Provider's event ID |
| `event_type` | VARCHAR(50) | Event type received |
| `processed_at` | TIMESTAMPTZ | When processed |

### Usage

```go
func (r *EventRepo) IsProcessed(ctx context.Context, provider, eventID string) (bool, error) {
    var exists bool
    err := r.db.QueryRow(ctx,
        "SELECT EXISTS(SELECT 1 FROM processed_events WHERE provider = $1 AND event_id = $2)",
        provider, eventID,
    ).Scan(&exists)
    return exists, err
}

func (r *EventRepo) MarkProcessed(ctx context.Context, provider, eventID, eventType string) error {
    _, err := r.db.Exec(ctx,
        "INSERT INTO processed_events (provider, event_id, event_type) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
        provider, eventID, eventType,
    )
    return err
}
```

### Cleanup

Old processed event records can be cleaned up:

```sql
-- Remove records older than 30 days
DELETE FROM processed_events WHERE processed_at < NOW() - INTERVAL '30 days';
```
