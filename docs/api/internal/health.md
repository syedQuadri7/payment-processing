# Health and Metrics Endpoints

Endpoints for health checks, liveness/readiness probes, and observability metrics.

## Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/health` | Overall service health status |
| GET | `/health/live` | Liveness probe for container orchestration |
| GET | `/health/ready` | Readiness probe for load balancer routing |
| GET | `/metrics` | Prometheus-format metrics |

---

## Health Check

Returns the overall health status of the service and its dependencies.

### Dependencies Checked

| Dependency | What is Verified |
|------------|------------------|
| Database | Connection and query execution |
| Temporal | Connection to Temporal server |
| Kafka | Connection to Kafka brokers |

### Health Statuses

| Status | HTTP Code | Meaning |
|--------|-----------|---------|
| `healthy` | 200 | All dependencies operational |
| `degraded` | 200 | Some dependencies failing, service operational |
| `unhealthy` | 503 | Critical dependencies failing |

### Response Fields

| Field | Description |
|-------|-------------|
| `status` | Overall health status |
| `version` | Service version |
| `checks` | Individual dependency check results |
| `timestamp` | When health was evaluated |

---

## Liveness Probe

Indicates whether the service process is running. Used by Kubernetes to decide whether to restart the container.

### Behavior

| Response | Meaning | Kubernetes Action |
|----------|---------|-------------------|
| 200 | Service is alive | No action |
| 503 | Service is dead | Restart container |

### Design Principle

Liveness should only fail if the process is truly stuck. It should NOT fail due to dependency issues (use readiness for that).

---

## Readiness Probe

Indicates whether the service can accept traffic. Used by Kubernetes to decide whether to route traffic to the pod.

### Behavior

| Response | Meaning | Kubernetes Action |
|----------|---------|-------------------|
| 200 | Ready for traffic | Include in load balancer |
| 503 | Not ready | Exclude from load balancer |

### What Makes Service Not Ready

- Database connection unavailable
- Temporal server unreachable
- Service still initializing

---

## Prometheus Metrics

Exposes metrics in Prometheus format for monitoring and alerting.

### Required Metrics

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| `payment_intents_total` | Counter | provider, status | Track payment volume |
| `payment_authorization_duration_seconds` | Histogram | provider | Authorization latency |
| `payment_decline_total` | Counter | provider, canonical_code | Decline patterns |
| `webhook_received_total` | Counter | provider, event_type | Webhook volume |
| `webhook_processing_duration_seconds` | Histogram | provider | Webhook handling latency |
| `clearing_account_balance` | Gauge | account | Clearing account monitoring |
| `temporal_workflow_active` | Gauge | workflow_type | Active workflow count |

### Alerting Thresholds

| Metric | Warning Threshold | Critical Threshold |
|--------|-------------------|-------------------|
| Authorization latency (p95) | > 500ms | > 2s |
| Error rate | > 1% | > 5% |
| Clearing balance age | > 12h | > 24h |
| Webhook processing latency (p95) | > 100ms | > 500ms |

### Metric Labels

| Label | Values | Purpose |
|-------|--------|---------|
| `provider` | STRIPE, ADYEN, PAYPAL | Segment by payment provider |
| `status` | created, authorized, captured, failed | Track state distribution |
| `canonical_code` | INSUFFICIENT_FUNDS, CARD_EXPIRED, etc. | Decline analysis |
| `event_type` | Provider-specific event names | Webhook analysis |
| `workflow_type` | PaymentWorkflow, RecoveryWorkflow | Workflow monitoring |
