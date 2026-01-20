# Health and Metrics Endpoints

Endpoints for health checks and observability.

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |
| `GET` | `/health/live` | Liveness probe |
| `GET` | `/health/ready` | Readiness probe |
| `GET` | `/metrics` | Prometheus metrics |

---

## Health Check

Returns overall service health status.

### Request

```
GET /health
```

### Example Request

```bash
curl http://localhost:8080/health
```

### Example Response (Healthy)

```json
{
  "status": "healthy",
  "version": "1.2.0",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 2
    },
    "temporal": {
      "status": "healthy",
      "latency_ms": 5
    },
    "kafka": {
      "status": "healthy",
      "latency_ms": 3
    }
  },
  "timestamp": "2026-01-17T10:30:00Z"
}
```

### Example Response (Degraded)

```json
{
  "status": "degraded",
  "version": "1.2.0",
  "checks": {
    "database": {
      "status": "healthy",
      "latency_ms": 2
    },
    "temporal": {
      "status": "unhealthy",
      "error": "connection timeout",
      "latency_ms": 5000
    },
    "kafka": {
      "status": "healthy",
      "latency_ms": 3
    }
  },
  "timestamp": "2026-01-17T10:30:00Z"
}
```

### Response Codes

| Code | Status | Description |
|------|--------|-------------|
| `200` | `healthy` | All checks passing |
| `200` | `degraded` | Some checks failing, service operational |
| `503` | `unhealthy` | Critical checks failing |

---

## Liveness Probe

Indicates if the service is running. Used by Kubernetes for restart decisions.

### Request

```
GET /health/live
```

### Example Response

```json
{
  "status": "alive",
  "timestamp": "2026-01-17T10:30:00Z"
}
```

### Response Codes

| Code | Description |
|------|-------------|
| `200` | Service is alive |
| `503` | Service is dead (should restart) |

---

## Readiness Probe

Indicates if the service can accept traffic. Used by Kubernetes for traffic routing.

### Request

```
GET /health/ready
```

### Example Response

```json
{
  "status": "ready",
  "checks": {
    "database": "ready",
    "temporal": "ready"
  },
  "timestamp": "2026-01-17T10:30:00Z"
}
```

### Response Codes

| Code | Description |
|------|-------------|
| `200` | Ready to accept traffic |
| `503` | Not ready (exclude from load balancer) |

---

## Prometheus Metrics

Exposes metrics in Prometheus format.

### Request

```
GET /metrics
```

### Example Response

```
# HELP payment_intents_total Total payment intents created
# TYPE payment_intents_total counter
payment_intents_total{provider="STRIPE",status="created"} 1523
payment_intents_total{provider="STRIPE",status="captured"} 1487
payment_intents_total{provider="ADYEN",status="created"} 892
payment_intents_total{provider="ADYEN",status="captured"} 867

# HELP payment_authorization_duration_seconds Authorization request duration
# TYPE payment_authorization_duration_seconds histogram
payment_authorization_duration_seconds_bucket{provider="STRIPE",le="0.1"} 1200
payment_authorization_duration_seconds_bucket{provider="STRIPE",le="0.5"} 1480
payment_authorization_duration_seconds_bucket{provider="STRIPE",le="1"} 1520
payment_authorization_duration_seconds_sum{provider="STRIPE"} 152.34
payment_authorization_duration_seconds_count{provider="STRIPE"} 1523

# HELP payment_decline_total Total payment declines by code
# TYPE payment_decline_total counter
payment_decline_total{provider="STRIPE",canonical_code="INSUFFICIENT_FUNDS"} 23
payment_decline_total{provider="STRIPE",canonical_code="CARD_EXPIRED"} 8
payment_decline_total{provider="ADYEN",canonical_code="GENERIC_DECLINE"} 15

# HELP webhook_received_total Total webhooks received
# TYPE webhook_received_total counter
webhook_received_total{provider="STRIPE",event_type="charge.captured"} 1487
webhook_received_total{provider="STRIPE",event_type="charge.failed"} 36
webhook_received_total{provider="ADYEN",event_type="AUTHORISATION"} 892

# HELP clearing_account_balance Current clearing account balance
# TYPE clearing_account_balance gauge
clearing_account_balance{account="payment_clearing"} 1500.00
clearing_account_balance{account="settlement_clearing"} 25000.00
clearing_account_balance{account="fee_clearing"} 0.00

# HELP temporal_workflow_active Active Temporal workflows
# TYPE temporal_workflow_active gauge
temporal_workflow_active{workflow_type="PaymentWorkflow"} 45
temporal_workflow_active{workflow_type="RecoveryWorkflow"} 3
```

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `payment_intents_total` | Counter | Total intents by provider and status |
| `payment_authorization_duration_seconds` | Histogram | Auth latency by provider |
| `payment_decline_total` | Counter | Declines by canonical code |
| `webhook_received_total` | Counter | Webhooks by provider and type |
| `clearing_account_balance` | Gauge | Current clearing balances |
| `temporal_workflow_active` | Gauge | Active workflow count |

### Alerting Thresholds

| Metric | Warning | Critical |
|--------|---------|----------|
| Authorization latency p95 | > 500ms | > 2s |
| Error rate | > 1% | > 5% |
| Clearing balance age | > 12h | > 24h |
| Webhook processing latency | > 100ms | > 500ms |
