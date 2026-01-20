# Outbox and Audit Tables

Database tables for reliable event publishing and audit trails.

## outbox

Implements the transactional outbox pattern for reliable event publishing. Events are written to this table in the same transaction as business data changes, then read by CDC (Change Data Capture) and published to Kafka.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key, used for deduplication |
| `aggregate_type` | String(50) | Yes | Type of entity (PaymentIntent, Account) |
| `aggregate_id` | UUID | Yes | ID of the entity |
| `event_type` | String(50) | Yes | Canonical event type |
| `payload` | JSON | Yes | Canonical event data |
| `created_at` | Timestamp | Yes | Event creation time |

### Event Types

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

### Payload Requirements

Payload must contain only canonical fields:
- Event ID
- Event type
- Entity ID
- Amount and currency (if applicable)
- Provider (if applicable)
- Timestamp
- Correlation ID for tracing

No provider-specific data should be included in the payload.

### CDC Configuration Requirements

- Table requires REPLICA IDENTITY FULL for complete row capture
- Debezium connector reads PostgreSQL WAL
- Event router transformation extracts event_type to Kafka headers
- CDC user needs replication permissions

### Cleanup

Old outbox records should be archived/deleted after confirmed delivery:
- Retention period: 7 days recommended
- Archive to cold storage before deletion
- Cleanup job should run during low-traffic periods

---

## audit_log

Append-only audit trail of all significant changes. Required for regulatory compliance and debugging.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `entity_type` | String(50) | Yes | Type of entity changed |
| `entity_id` | UUID | Yes | ID of entity changed |
| `action` | String(20) | Yes | Type of action |
| `actor_type` | String(20) | Yes | Type of actor |
| `actor_id` | String(100) | No | ID of actor |
| `old_values` | JSON | No | Previous field values |
| `new_values` | JSON | No | New field values |
| `metadata` | JSON | No | Additional context |
| `ip_address` | IP Address | No | Client IP address |
| `user_agent` | Text | No | Client user agent |
| `created_at` | Timestamp | Yes | Audit timestamp |

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
|------|-------------|
| `SYSTEM` | Internal system operation |
| `USER` | Authenticated user action |
| `WEBHOOK` | Provider webhook |
| `API` | API client |
| `WORKFLOW` | Temporal workflow |

### Immutability

This table is append-only:
- No UPDATE operations
- No DELETE operations
- Corrections require new audit entries
- Data retention per compliance requirements

### Query Patterns

The table should support efficient queries for:
- All changes to a specific entity (entity_type + entity_id)
- All actions by a specific actor (actor_type + actor_id)
- Actions within a time range (created_at)
- Status changes (action = 'STATUS_CHANGE')

---

## processed_events

Tracks processed webhook events for idempotency. Prevents duplicate processing when providers send the same webhook multiple times.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `provider` | String(20) | Yes | STRIPE, ADYEN, PAYPAL |
| `event_id` | String(100) | Yes | Provider's event ID |
| `event_type` | String(50) | Yes | Event type received |
| `processed_at` | Timestamp | Yes | When processed |

### Constraints

- Unique constraint on (provider, event_id)

### Usage Pattern

Before processing a webhook:
1. Check if (provider, event_id) exists
2. If exists, return 200 without reprocessing
3. If not exists, process webhook
4. Insert record after successful processing

### Cleanup

Old records can be deleted after retention period:
- Retention period: 30 days recommended
- Providers rarely retry after 7 days, but buffer provides safety
- Cleanup job should run during low-traffic periods

---

## Data Retention Summary

| Table | Retention | Archive Strategy |
|-------|-----------|------------------|
| `outbox` | 7 days | Archive to cold storage |
| `audit_log` | Per compliance (typically 7 years) | Partition by time, archive old partitions |
| `processed_events` | 30 days | Delete without archive |
