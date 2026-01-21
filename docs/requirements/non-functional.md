# Non-Functional Requirements

Performance, reliability, scalability, and quality attributes.

---

## Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-PERF-01 | Payment intent creation API response time | < 200ms (p95) |
| NFR-PERF-02 | Payment status query response time | < 100ms (p95) |
| NFR-PERF-03 | Workflow startup latency | < 500ms |
| NFR-PERF-04 | Concurrent payment processing capacity | 100+ simultaneous workflows |
| NFR-PERF-05 | CDC event latency (DB to Kafka) | < 100ms (p95) |
| NFR-PERF-06 | Ledger entry creation latency | < 50ms (p95) |
| NFR-PERF-07 | Webhook processing (verify + normalize) | < 50ms (p95) |

---

## Reliability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-REL-01 | Payment processing must survive worker restarts | 100% |
| NFR-REL-02 | Duplicate payment prevention via idempotency | 100% |
| NFR-REL-03 | Ledger consistency (balanced entries) | 100% |
| NFR-REL-04 | Workflow state durability | Survives any single component failure |
| NFR-REL-05 | Event delivery guarantee | At-least-once with idempotent consumers |
| NFR-REL-06 | Zero data loss on component failure | Required |
| NFR-REL-07 | Webhook acknowledgment to providers | Return 200 only after safe processing |

---

## Scalability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-SCA-01 | Horizontal worker scaling | Workers can be added without code changes |
| NFR-SCA-02 | Database connection pooling | PgBouncer in transaction mode |
| NFR-SCA-03 | Stateless API servers | Multiple instances behind load balancer |
| NFR-SCA-04 | Kafka partition scaling | Support partition increase without issues |
| NFR-SCA-05 | Per-provider webhook scaling | Each adapter can scale independently |

---

## Maintainability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-MNT-01 | Code documentation | Public functions documented with comments |
| NFR-MNT-02 | Test coverage (workflow code) | Minimum 70% |
| NFR-MNT-03 | Test coverage (adapter code) | Minimum 80% |
| NFR-MNT-04 | Configuration externalization | All environment-specific values via config |
| NFR-MNT-05 | Structured logging | JSON-formatted logs with correlation IDs |
| NFR-MNT-06 | Provider isolation | Adding new provider requires no core changes |

---

## Security

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-SEC-01 | Sensitive data handling | No PCI data stored; use tokenized payment methods |
| NFR-SEC-02 | Database credentials | Environment variables, never hardcoded |
| NFR-SEC-03 | Webhook verification | Provider-specific signature validation required |
| NFR-SEC-04 | API key protection | Keys not logged or exposed in responses |
| NFR-SEC-05 | Row-level locking | Prevent race conditions on balance updates |
| NFR-SEC-06 | Provider secrets isolation | Each provider's credentials stored separately |

---

## Observability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-OBS-01 | Structured logging | All operations logged with context |
| NFR-OBS-02 | Temporal UI integration | Workflows visible and inspectable |
| NFR-OBS-03 | Health check endpoints | API and worker health status |
| NFR-OBS-04 | Metrics exposure | Prometheus-compatible metrics |
| NFR-OBS-05 | Clearing account monitoring | Alert on non-zero clearing balances |
| NFR-OBS-06 | Latency percentile tracking | p50, p95, p99 for all operations |
| NFR-OBS-07 | Per-provider metrics | Track success/failure rates by provider |

---

## Metrics by Provider

| Metric | Labels | Purpose |
|--------|--------|---------|
| payment_authorization_total | provider, status | Volume by provider |
| payment_authorization_duration_seconds | provider | Latency by provider |
| payment_decline_total | provider, canonical_code | Decline patterns |
| webhook_received_total | provider, event_type | Webhook volume |
| webhook_processing_duration_seconds | provider | Webhook latency |

---

## Alerting Rules

| Alert | Condition | Severity |
|-------|-----------|----------|
| Provider High Error Rate | error_rate{provider=X} > 5% for 5m | Warning |
| Provider Down | success_rate{provider=X} < 1% for 2m | Critical |
| Unmapped Decline Code | unmapped_decline_total increases | Info |
| Webhook Signature Failure | signature_failure_total > 10 in 5m | Warning |
| Clearing Account Non-Zero | balance > threshold for 24h | Warning |

---

## Success Criteria

### Functional Completeness

| Criterion | Measurement |
|-----------|-------------|
| Multi-provider webhooks work | All three adapters normalize correctly |
| Canonical model used internally | No provider-specific types in workflows |
| Intent/Method separation | Payment method can be changed without new intent |
| Authorization holds work | Pending balance reflects active holds |
| Capture records ledger entries | Balanced double-entry entries created |
| Clearing accounts monitored | Non-zero balances generate alerts |
| Retry uses canonical codes | Decline classification provider-agnostic |
| CDC publishes canonical events | Events appear in Kafka within 100ms |

### Architectural Compliance

| Criterion | Measurement |
|-----------|-------------|
| Normalize at edge | Workflows never see provider types |
| No dual-writes | All events through outbox only |
| Linear state machines | No circular PaymentAttempt states |
| Idempotent operations | Duplicate requests return same result |
| Race conditions prevented | Concurrent webhook/API handled safely |

### Code Quality

| Criterion | Target |
|-----------|--------|
| Test coverage (workflow) | >= 70% |
| Test coverage (adapter) | >= 80% |
| Test coverage (activity) | >= 70% |
| Linting passes | Zero warnings |
| No hardcoded credentials | All via environment/config |

---

## Error Handling Strategy

| Category | Approach |
|----------|----------|
| Temporal unavailable | API returns 503, retry later; workflows are durable |
| Database unavailable | Standard error handling, fail fast |
| Unknown webhook payment ID | Log, return 200 to provider, investigate |
| Late-arriving webhooks | Accept if idempotent, log if state contradicts |
| Stuck workflows | Use Temporal UI for manual intervention |

See [FD-015 Failure Handling](../decisions/finalized-decisions.md#fd-015-failure-handling) for details.

---

## See Also

- [Functional Requirements](functional.md) - What the system does
- [System Design](../architecture/system-design.md) - Architecture overview
- [Finalized Decisions](../decisions/finalized-decisions.md) - Implementation choices
