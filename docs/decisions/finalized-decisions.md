# Finalized Decisions

Decisions that have been resolved with simple defaults appropriate for a learning project. Each decision notes what production systems typically do differently.

---

## FD-001: Retry Timing Strategy

**Originally:** PD-004

### Decision

Use fixed intervals for retry timing: 4 hours, 12 hours, 24 hours, 48 hours.

### Simple Default

```
Attempt 1: Initial try
Attempt 2: 4 hours after failure
Attempt 3: 12 hours after attempt 2
Attempt 4: 24 hours after attempt 3
Attempt 5: 48 hours after attempt 4 (final)
```

Maximum dunning window: ~4 days

### Why This Works for Learning

- Simple to implement and understand
- Demonstrates Temporal durable timers
- Shows the retry pattern without complex scheduling

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Payday alignment** | Retry on 1st/15th or Fridays when funds more likely |
| **ML optimization** | Learn optimal retry times from historical success |
| **Decline-specific timing** | Different schedules for insufficient funds vs. generic decline |
| **Merchant configuration** | Let merchants customize retry schedules |
| **Customer timezone** | Retry at appropriate times for customer's location |

### Configuration

```go
var RetryIntervals = []time.Duration{
    4 * time.Hour,
    12 * time.Hour,
    24 * time.Hour,
    48 * time.Hour,
}
```

---

## FD-002: Webhook Delivery Guarantees

**Originally:** PD-005

### Decision

Kafka only - internal consumers read from Kafka topics. No external webhook delivery endpoints.

### Simple Default

Events flow from outbox via CDC to Kafka. Internal services consume from Kafka topics.

```
DB Transaction → Outbox Table → Debezium CDC → Kafka → Internal Consumers
```

### Why This Works for Learning

- Demonstrates the transactional outbox pattern
- Shows CDC-based event publishing
- Kafka provides replay capability for debugging

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **External webhooks** | HTTP endpoints for merchant event delivery |
| **Retry with backoff** | Automatic retry of failed webhook deliveries |
| **Webhook signatures** | HMAC signing for webhook authentication |
| **Event filtering** | Subscribe to specific event types only |
| **Multiple endpoints** | Multiple webhook URLs per merchant |
| **Polling fallback** | API to fetch missed events |

### Note on External Webhooks

If external webhook delivery is needed later, it would be a separate service that:

1. Consumes from Kafka
2. Maintains webhook configurations per merchant
3. Delivers events with retry and signature verification
4. Tracks delivery status

---

## FD-003: Idempotency Key Management

**Originally:** PD-006

### Decision

Client-provided idempotency keys with 24-hour expiration, scoped per merchant.

### Simple Default

| Setting | Value |
|---------|-------|
| Key source | Client-provided (required) |
| Key format | Any string up to 100 characters |
| Expiration | 24 hours from first use |
| Scope | Per merchant (merchant_id + idempotency_key) |
| Collision handling | Return original response |

### Why This Works for Learning

- Demonstrates idempotency pattern clearly
- Simple to implement and test
- 24 hours is sufficient for most retry scenarios

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Hybrid keys** | Accept client keys or generate server-side |
| **Longer expiration** | 48-72 hours for complex integrations |
| **Request hash** | Detect same key with different parameters |
| **Collision errors** | Return error instead of original response |
| **Idempotency replay** | Store and replay full response |

### Implementation

```go
type IdempotencyRecord struct {
    Key         string    // Client-provided key
    MerchantID  string    // Scoping
    Response    []byte    // Cached response
    StatusCode  int       // Original HTTP status
    CreatedAt   time.Time // For expiration
}

// Check: WHERE key = $1 AND merchant_id = $2 AND created_at > NOW() - INTERVAL '24 hours'
```

---

## FD-004: Authorization Hold Expiration

**Originally:** PD-008

### Decision

Use provider defaults for hold expiration. Void holds if capture is not received before expiration.

### Simple Default

| Setting | Value |
|---------|-------|
| Hold period | Provider default (typically 7 days) |
| Expiration handling | Automatic void |
| Re-authorization | Not implemented |
| Extension | Not implemented |

### Why This Works for Learning

- Provider defaults are reasonable for most use cases
- Demonstrates hold tracking and voiding
- Avoids complexity of re-authorization flows

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Proactive re-auth** | Re-authorize before hold expires |
| **Hold extension** | Use provider APIs to extend holds |
| **Configurable periods** | Per-merchant hold duration settings |
| **Expiration alerts** | Notify merchants of approaching expiration |
| **Network-specific rules** | Different handling for Visa vs. Mastercard |

### Hold Lifecycle

```
Authorization → Hold Created (expires_at from provider)
                    ↓
        ┌───────────┴───────────┐
        ↓                       ↓
    Capture                 Expiration
    (normal)               (auto-void)
```

---

## FD-005: Rate Limiting and Throttling

**Originally:** PD-010

### Decision

Global fixed rate limits. All merchants have the same limits.

### Simple Default

| Endpoint Type | Limit |
|---------------|-------|
| Payment creation | 100 requests/minute |
| Status queries | 300 requests/minute |
| Webhook receiving | No limit (provider-controlled) |

### Why This Works for Learning

- Demonstrates rate limiting pattern
- Simple to implement with standard middleware
- Prevents runaway clients from affecting system

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Tiered limits** | Higher limits for premium merchants |
| **Per-endpoint limits** | Different limits per API endpoint |
| **Burst allowance** | Allow temporary bursts above limit |
| **Dynamic limits** | Adjust based on system load |
| **IP-based limits** | Additional DDoS protection |
| **Quota tracking** | Monthly/daily usage quotas |

### Implementation

```go
// Simple token bucket per merchant
rateLimiter := NewTokenBucket(
    rate:     100,           // 100 tokens per minute
    capacity: 100,           // burst capacity
)

if !rateLimiter.Allow(merchantID) {
    return 429, "Rate limit exceeded"
}
```

---

## FD-006: Testing and Sandbox Environment

**Originally:** PD-011

### Decision

Use provider test modes directly. No dedicated sandbox environment.

### Simple Default

| Approach | Description |
|----------|-------------|
| Environment | Single environment with test mode flag |
| Test cards | Use provider test card numbers |
| Test webhooks | Webhook simulator for local testing |
| Test isolation | `test_mode: true` flag on payment intents |

### Why This Works for Learning

- Provider sandboxes are well-maintained
- Webhook simulator covers local testing needs
- No infrastructure duplication required

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Dedicated sandbox** | Completely separate environment |
| **Synthetic providers** | Internal mock provider for testing |
| **Test scenarios** | Pre-built scenarios (happy path, declines, disputes) |
| **Data isolation** | Separate database for test data |
| **Automated testing** | CI/CD integration with sandbox |

### Using Provider Test Modes

| Provider | Test Mode |
|----------|-----------|
| Stripe | Use `sk_test_` API keys |
| Adyen | Use test merchant account |
| PayPal | Use sandbox environment (`sandbox.paypal.com`) |

### Webhook Simulator

For local development and testing, use the webhook simulator:

```bash
cd tools/webhook-simulator
./webhook-simulator send stripe payment_intent.succeeded --amount 10000
```

See [Simulation Documentation](../simulations/readme.md) for details.

---

---

## FD-007: Payment Method Storage

**Originally:** PD-007

### Decision

Store provider tokens only. No internal token abstraction layer.

### Simple Default

| Setting | Value |
|---------|-------|
| Token storage | Provider-issued tokens only |
| Customer data | Last four digits, expiry, card brand (for display) |
| Deletion | Customers can delete payment methods |
| Multi-provider | Same card requires separate token per provider |

### Why This Works for Learning

- Demonstrates token storage pattern
- Avoids complexity of token abstraction layer
- Provider tokens are well-tested and PCI-compliant

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Internal token layer** | Abstract tokens that map to provider tokens |
| **Network tokenization** | Visa/Mastercard card-on-file tokens |
| **Multi-provider tokens** | Single card usable across providers |
| **Vault service** | Dedicated secure storage for sensitive data |

---

## FD-008: Dispute and Chargeback Handling

**Originally:** PD-009

### Decision

Notification only. Record disputes and publish events, but no evidence handling.

### Simple Default

| Setting | Value |
|---------|-------|
| Dispute recording | Store dispute ID, amount, reason, status |
| Event publishing | `DISPUTE_OPENED`, `DISPUTE_WON`, `DISPUTE_LOST` |
| Evidence handling | Not implemented (provider portal used) |
| Balance impact | No immediate balance change; update after resolution |

### Why This Works for Learning

- Demonstrates dispute event flow
- Shows state tracking for long-running processes
- Evidence handling is provider-specific UI work

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Evidence collection** | API to upload evidence documents |
| **Deadline tracking** | Reminders before response deadlines |
| **Automatic holds** | Pause subscriptions during disputes |
| **Pre-dispute alerts** | Visa VMPI, Mastercard SAFE integration |

---

## FD-009: Audit Log Retention

**Originally:** PD-012

### Decision

90-day retention in the same database. Metadata plus sanitized requests.

### Simple Default

| Setting | Value |
|---------|-------|
| Retention | 90 days |
| Content | Metadata + sanitized request (no raw card data) |
| Storage | Same database, `audit_log` table |
| Access | Direct database queries |

### Why This Works for Learning

- 90 days is sufficient for debugging and development
- Same-database storage simplifies operations
- Demonstrates audit logging pattern

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **7+ year retention** | Compliance requirements |
| **Log aggregation** | Separate storage (Elasticsearch, etc.) |
| **Query API** | Search interface for audit logs |
| **Export** | Compliance audit export functionality |
| **Immutable storage** | Write-once storage for tamper resistance |

---

## FD-010: Notifications

**Originally:** PD-013

### Decision

Events only. Publish to Kafka and let downstream services handle notifications.

### Simple Default

| Setting | Value |
|---------|-------|
| Notification approach | Events only via Kafka |
| Email/SMS | Not implemented |
| In-app notifications | Not implemented |

### Why This Works for Learning

- Demonstrates event-driven architecture
- Notification delivery is a separate domain
- Kafka events provide the data; consumers handle delivery

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Email integration** | Transaction receipts, failure alerts |
| **SMS notifications** | Security alerts, OTP |
| **Push notifications** | Mobile app integration |
| **Merchant templates** | Customizable notification content |
| **Multi-language** | Localized notifications |

---

## FD-011: Temporal Workflow Engine

### Decision

Use Temporal for durable workflow orchestration instead of database state machines or message queues.

### Simple Default

| Setting | Value |
|---------|-------|
| Workflow engine | Temporal |
| Task queue | `payment-processing` |
| Workflow ID | `payment-{idempotency_key}` |
| ID reuse policy | Reject duplicate |

### Why This Works for Learning

- Durable execution survives process crashes
- Built-in retry with configurable policies
- Signal/query support for external events (webhooks)
- Temporal UI for debugging workflow state

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Multi-cluster** | Temporal clusters across regions for HA |
| **Custom search attributes** | Index workflows by business attributes |
| **Workflow versioning** | Safe deployment of workflow changes |
| **Advanced visibility** | Custom queries for operational dashboards |

### Key Patterns

```go
// Workflows must be deterministic
workflow.Now(ctx)       // Use this, not time.Now()
workflow.Sleep(ctx, d)  // Durable timer

// Side effects belong in activities only
workflow.ExecuteActivity(ctx, activity, input)
```

---

## FD-012: Multi-Provider Adapter Pattern

### Decision

Normalize all provider webhooks to canonical events at the edge. Core system only uses canonical types.

### Simple Default

| Setting | Value |
|---------|-------|
| Adapter location | Edge (webhook endpoints) |
| Internal model | Canonical events and decline codes |
| Provider logic | Isolated in adapter and provider-specific activities |

### Why This Works for Learning

- Core system is provider-agnostic
- Adding a provider means adding an adapter, not changing core
- Canonical decline codes enable consistent retry logic

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Dynamic mapping** | Database-driven code mappings (hot reload) |
| **Adapter metrics** | Per-provider success rates, latency |
| **Circuit breakers** | Automatic provider failover |
| **A/B routing** | Test new adapters with traffic percentage |

### Canonical Event Flow

```
Provider Webhook → Adapter → Canonical Event → Signal Workflow → Process
```

---

## FD-013: Transactional Outbox Pattern

### Decision

Write events to an outbox table in the same transaction as business data. Use CDC (Debezium) to publish to Kafka.

### Simple Default

| Setting | Value |
|---------|-------|
| Event storage | `outbox` table in same database |
| Publishing | Debezium CDC to Kafka |
| Event format | JSON with version field |

### Why This Works for Learning

- Solves dual-write problem without distributed transactions
- Events are guaranteed to be published if transaction commits
- Kafka provides replay capability

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Avro + Schema Registry** | Schema evolution with compatibility |
| **Outbox cleanup** | Periodic deletion of published events |
| **Multiple topics** | Route events by type to different topics |
| **Exactly-once Kafka** | Idempotent producer configuration |

### Simple Path Alternative

For getting started without CDC infrastructure, poll the outbox table directly:

```go
// Simple polling consumer
SELECT * FROM outbox WHERE published_at IS NULL ORDER BY created_at LIMIT 100
// Process events, then mark as published
```

---

## FD-014: Double-Entry Bookkeeping

### Decision

Track all money movement with balanced debit/credit entries. Use clearing accounts to track in-flight transactions.

### Simple Default

| Setting | Value |
|---------|-------|
| Entry creation | On capture and refund only (not authorization) |
| Balance tracking | `ledger_balance`, `pending_balance`, `available_balance` |
| Clearing accounts | Authorization clearing, settlement clearing |

### Why This Works for Learning

- Mathematical proof of correctness (debits = credits)
- Self-auditing via clearing account monitoring
- Clear audit trail for all money movement

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Auth ledger entries** | Optional entries for detailed audit |
| **Multi-currency ledgers** | Separate entries per currency |
| **Real-time reconciliation** | Continuous balance verification |
| **Subledgers** | Separate ledgers for different account types |

### Key Principle

Authorization is a promise, not money movement. Ledger entries are created only when money actually moves (capture, refund, settlement).

---

## FD-015: Failure Handling Patterns

### Decision

Handle failures with simple, well-documented patterns. Prefer logging and manual intervention over complex automated recovery for rare edge cases.

### Simple Default

| Failure Mode | Handling |
|--------------|----------|
| Temporal unavailable | Return HTTP 503, client retries |
| Database unavailable | Return HTTP 503, activity retries |
| Unknown webhook payment ID | Log warning, return 200 |
| Late-arriving webhook | Log and ignore (idempotent) |
| Stuck workflow | Manual intervention via Temporal UI |

### Why This Works for Learning

- Simple error handling that's easy to understand
- Logging provides visibility for debugging
- Temporal's durability handles most recovery automatically

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Circuit breakers** | Fail fast during outages |
| **Dead letter queues** | Capture failed events for reprocessing |
| **Automated escalation** | Workflows that trigger on stuck payments |
| **Reconciliation jobs** | Automated detection and resolution |

### Unknown Payment Webhook Handling

```go
if err == ErrPaymentNotFound {
    log.Warn("webhook for unknown payment", "provider_id", id)
    return 200  // Acknowledge to prevent retries
}
```

---

## Summary Table

| Decision | Simple Default | Production Enhancement |
|----------|---------------|----------------------|
| FD-001 Retry Timing | Fixed intervals (4h, 12h, 24h, 48h) | Payday-aligned, ML-optimized |
| FD-002 Webhook Delivery | Kafka only (internal consumers) | Add external webhook endpoints |
| FD-003 Idempotency Keys | Client-provided, 24h expiration | Hybrid with server fallback |
| FD-004 Hold Expiration | Provider defaults, void on expire | Proactive re-authorization |
| FD-005 Rate Limiting | Global fixed limits (100 req/min) | Tiered per merchant |
| FD-006 Sandbox | Provider test modes directly | Dedicated sandbox environment |
| FD-007 Payment Methods | Provider tokens only | Internal token abstraction |
| FD-008 Disputes | Notification only, no evidence | Evidence collection API |
| FD-009 Audit Logs | 90-day retention in database | 7+ years, dedicated storage |
| FD-010 Notifications | Events only via Kafka | Email/SMS/push integration |
| FD-011 Temporal Workflows | Single cluster, basic config | Multi-cluster, versioning |
| FD-012 Multi-Provider Adapters | Static code mappings | Dynamic mappings, circuit breakers |
| FD-013 Transactional Outbox | CDC to Kafka (or polling) | Avro schemas, exactly-once |
| FD-014 Double-Entry Ledger | Entries on capture/refund only | Full audit trail, multi-currency |
| FD-015 Failure Handling | Log and manual intervention | Automated recovery, dead letter queues |

---

## FD-016: API Versioning (Stripe-Style)

### Decision

Use date-based API versioning with header pinning, like Stripe.

### Simple Default

| Setting | Value |
|---------|-------|
| Version format | Date-based: `2026-01-20` |
| Version header | `API-Version` |
| Default behavior | Latest version if no header |
| Pinning | Merchant account stores pinned version |

### How It Works

```
Client Request:
  GET /api/intents/pi_123
  API-Version: 2026-01-20

Server:
  1. Check header for version (or use merchant's pinned version)
  2. Process request
  3. Transform response to match requested version
```

### Why This Works for Learning

- Demonstrates a production pattern used by Stripe, Twilio, etc.
- Forces thinking about backward compatibility
- Version transformers are interesting to implement

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Changelog per version** | Document changes between versions |
| **Deprecation warnings** | Headers warning of old version usage |
| **Version sunset** | Force upgrade after deprecation period |
| **Per-endpoint versioning** | Different versions for different endpoints |

### Implementation Approach

```go
// Middleware extracts version
func VersionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        version := r.Header.Get("API-Version")
        if version == "" {
            version = getMerchantPinnedVersion(r)
        }
        if version == "" {
            version = CurrentVersion
        }
        ctx := context.WithValue(r.Context(), "api-version", version)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Response transformer based on version
func TransformResponse(version string, response any) any {
    if version < "2026-01-15" {
        // Apply backward-compatible transformations
    }
    return response
}
```

---

## FD-017: API Authentication (Key + Secret)

### Decision

Use API key pairs: publishable key (client-side) and secret key (server-side), like Stripe.

### Simple Default

| Key Type | Prefix | Use Case | Permissions |
|----------|--------|----------|-------------|
| Secret key | `sk_` | Server-side | Full API access |
| Publishable key | `pk_` | Client-side | Limited (create tokens only) |
| Test keys | `sk_test_`, `pk_test_` | Development | Same permissions, test mode |

### How It Works

```
Server-side (secret key):
  POST /api/intents
  Authorization: Bearer sk_live_abc123
  → Full access to create, capture, refund

Client-side (publishable key):
  POST /api/tokens
  Authorization: Bearer pk_live_xyz789
  → Can only tokenize card details
```

### Why This Works for Learning

- Demonstrates the publishable/secret key pattern
- Forces thinking about what operations are safe client-side
- Common pattern across Stripe, Twilio, etc.

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Restricted keys** | Custom scopes (read-only, specific endpoints) |
| **Key rotation** | Generate new keys without downtime |
| **Key metadata** | Name, created date, last used |
| **IP restrictions** | Limit key usage to specific IPs |

### Key Storage

```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY,
    merchant_id UUID NOT NULL REFERENCES merchants(id),
    key_hash VARCHAR(64) NOT NULL,  -- Store hash, not plaintext
    key_prefix VARCHAR(20) NOT NULL, -- sk_live_, pk_test_, etc.
    key_type VARCHAR(20) NOT NULL,   -- secret, publishable
    test_mode BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ
);
```

---

## FD-018: Request Timeout Handling

### Decision

Treat timeouts as failures and let Temporal's activity retry handle recovery.

### Simple Default

| Setting | Value |
|---------|-------|
| Provider API timeout | 30 seconds |
| On timeout | Return error, activity retries |
| Idempotency | Activity uses idempotency key with provider |

### How It Works

```go
func (a *StripeAuthActivity) Execute(ctx context.Context, input AuthInput) (*AuthResult, error) {
    // Set timeout for provider call
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // Call provider with idempotency key
    result, err := a.stripe.Authorize(ctx, input, input.IdempotencyKey)
    if err != nil {
        // Timeout or other error - return error, Temporal will retry
        return nil, fmt.Errorf("stripe auth failed: %w", err)
    }

    return result, nil
}
```

### Why This Works for Learning

- Simple error handling
- Temporal's retry policy handles transient failures
- Idempotency keys prevent duplicate charges on retry

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Status queries** | On timeout, query provider for transaction status |
| **Reconciliation** | Background job matches provider records with ours |
| **Circuit breakers** | Stop calling provider if repeated timeouts |
| **Timeout metrics** | Track timeout rates per provider |

---

## Summary Table

| Decision | Simple Default | Production Enhancement |
|----------|---------------|----------------------|
| FD-001 Retry Timing | Fixed intervals (4h, 12h, 24h, 48h) | Payday-aligned, ML-optimized |
| FD-002 Webhook Delivery | Kafka only (internal consumers) | Add external webhook endpoints |
| FD-003 Idempotency Keys | Client-provided, 24h expiration | Hybrid with server fallback |
| FD-004 Hold Expiration | Provider defaults, void on expire | Proactive re-authorization |
| FD-005 Rate Limiting | Global fixed limits (100 req/min) | Tiered per merchant |
| FD-006 Sandbox | Provider test modes directly | Dedicated sandbox environment |
| FD-007 Payment Methods | Provider tokens only | Internal token abstraction |
| FD-008 Disputes | Notification only, no evidence | Evidence collection API |
| FD-009 Audit Logs | 90-day retention in database | 7+ years, dedicated storage |
| FD-010 Notifications | Events only via Kafka | Email/SMS/push integration |
| FD-011 Temporal Workflows | Single cluster, basic config | Multi-cluster, versioning |
| FD-012 Multi-Provider Adapters | Static code mappings | Dynamic mappings, circuit breakers |
| FD-013 Transactional Outbox | CDC to Kafka (or polling) | Avro schemas, exactly-once |
| FD-014 Double-Entry Ledger | Entries on capture/refund only | Full audit trail, multi-currency |
| FD-015 Failure Handling | Log and manual intervention | Automated recovery, dead letter queues |
| FD-016 API Versioning | Date-based (Stripe-style) | Version sunset, per-endpoint versions |
| FD-017 API Authentication | Key + secret pairs | Restricted keys, IP restrictions |
| FD-018 Timeout Handling | Fail and retry | Status queries, reconciliation |

---

## FD-019: Partial Captures (Stripe-Style)

### Decision

Support single partial capture. Capture amount can be less than or equal to authorized amount. Remainder is automatically released.

### Simple Default

| Setting | Value |
|---------|-------|
| Capture amount | Must be <= authorized amount |
| Multiple captures | Not supported (single capture only) |
| Remainder handling | Auto-released after capture |
| Over-capture | Not supported |

### How It Works

```
Authorization: $100.00
    ↓
Capture: $75.00 (partial)
    ↓
Result: $75.00 captured, $25.00 released
```

### Why This Works for Learning

- Demonstrates that authorization != capture amount
- Shows hold release for unused authorization
- Matches Stripe's model (most common pattern)
- Simpler than tracking multiple partial captures

### What Production Systems Do Differently

| Enhancement | Description |
|-------------|-------------|
| **Multiple partials** | Adyen/PayPal style: multiple captures until auth exhausted |
| **Remaining auth tracking** | Track how much authorization remains |
| **final_capture flag** | Explicitly mark last capture (PayPal pattern) |
| **Over-capture** | Allow capture > auth for tips (limited use cases) |

### Implementation

```go
func (w *PaymentWorkflow) Capture(ctx workflow.Context, amount decimal.Decimal) error {
    if amount.GreaterThan(w.state.AuthorizedAmount) {
        return ErrCaptureExceedsAuthorization
    }
    if w.state.CapturedAmount.GreaterThan(decimal.Zero) {
        return ErrAlreadyCaptured  // Single capture only
    }

    // Capture the requested amount
    w.state.CapturedAmount = amount

    // Release remainder if partial
    if amount.LessThan(w.state.AuthorizedAmount) {
        remainder := w.state.AuthorizedAmount.Sub(amount)
        // Void the remainder (releases hold)
    }

    return nil
}
```

### Ledger Impact

For a $100 auth with $75 capture:

```
Journal Entry: "Capture payment PI-123"
  DEBIT   Customer Account     $75.00
  CREDIT  Settlement Clearing  $75.00

Journal Entry: "Release partial auth PI-123"
  (No ledger entry - auth hold was not in ledger)
  (Only update pending_balance on customer account)
```

---

## Summary Table

| Decision | Simple Default | Production Enhancement |
|----------|---------------|----------------------|
| FD-001 Retry Timing | Fixed intervals (4h, 12h, 24h, 48h) | Payday-aligned, ML-optimized |
| FD-002 Webhook Delivery | Kafka only (internal consumers) | Add external webhook endpoints |
| FD-003 Idempotency Keys | Client-provided, 24h expiration | Hybrid with server fallback |
| FD-004 Hold Expiration | Provider defaults, void on expire | Proactive re-authorization |
| FD-005 Rate Limiting | Global fixed limits (100 req/min) | Tiered per merchant |
| FD-006 Sandbox | Provider test modes directly | Dedicated sandbox environment |
| FD-007 Payment Methods | Provider tokens only | Internal token abstraction |
| FD-008 Disputes | Notification only, no evidence | Evidence collection API |
| FD-009 Audit Logs | 90-day retention in database | 7+ years, dedicated storage |
| FD-010 Notifications | Events only via Kafka | Email/SMS/push integration |
| FD-011 Temporal Workflows | Single cluster, basic config | Multi-cluster, versioning |
| FD-012 Multi-Provider Adapters | Static code mappings | Dynamic mappings, circuit breakers |
| FD-013 Transactional Outbox | CDC to Kafka (or polling) | Avro schemas, exactly-once |
| FD-014 Double-Entry Ledger | Entries on capture/refund only | Full audit trail, multi-currency |
| FD-015 Failure Handling | Log and manual intervention | Automated recovery, dead letter queues |
| FD-016 API Versioning | Date-based (Stripe-style) | Version sunset, per-endpoint versions |
| FD-017 API Authentication | Key + secret pairs | Restricted keys, IP restrictions |
| FD-018 Timeout Handling | Fail and retry | Status queries, reconciliation |
| FD-019 Partial Captures | Single partial (Stripe-style) | Multiple partials, over-capture |

---

## See Also

- [Deferred Decisions](deferred-decisions.md) - Enterprise-scale decisions beyond learning scope
- [Pending Decisions](pending-decisions.md) - Template for future decisions
