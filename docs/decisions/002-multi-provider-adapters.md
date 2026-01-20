# ADR-002: Multi-Provider Adapter Pattern

## Status

Accepted

## Context

Modern payment systems rarely use a single payment provider. Organizations use multiple providers for:

| Reason | Example |
|--------|---------|
| Geographic coverage | Adyen for EU, Stripe for US |
| Cost optimization | Route based on interchange rates |
| Redundancy | Failover if primary provider is down |
| Feature requirements | PayPal for buyer protection |
| Legacy integration | Existing contracts with specific providers |

Each provider has different:
- Webhook formats and event names
- Signature verification methods
- Decline codes and their meanings
- API response structures

Without careful design, provider-specific logic spreads throughout the codebase, creating tight coupling and making it difficult to add new providers.

## Decision

We will implement the **Adapter Pattern** with a canonical event model:

1. **Provider-specific adapters** at the edge normalize webhooks to canonical events
2. **Core system** (workflows, activities, ledger) only speaks canonical types
3. **Provider-specific activities** handle outbound API calls, returning canonical results

### Architecture

```
┌─────────────────────────────────────────────────────┐
│                   ADAPTER LAYER                      │
│                                                      │
│   Stripe Adapter    Adyen Adapter    PayPal Adapter │
│        │                 │                │          │
│        └─────────────────┼────────────────┘          │
│                          │                           │
│                          ▼                           │
│               Canonical Payment Event                │
└──────────────────────────┬──────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────┐
│                    CORE SYSTEM                       │
│            (Provider-Agnostic)                       │
│                                                      │
│   Workflows    Activities    Ledger    Outbox       │
└─────────────────────────────────────────────────────┘
```

### Canonical Event Model

```go
type CanonicalPaymentEvent struct {
    EventType         CanonicalEventType
    PaymentID         string
    ProviderPaymentID string
    Provider          Provider
    Amount            int64
    Currency          string
    Status            CanonicalStatus
    FailureCode       *CanonicalDeclineCode
    RawPayload        json.RawMessage
    ReceivedAt        time.Time
}
```

### Canonical Event Types

| Event Type | Meaning |
|------------|---------|
| `AUTHORIZATION_SUCCEEDED` | Auth approved, hold placed |
| `AUTHORIZATION_FAILED` | Auth declined |
| `CAPTURE_SUCCEEDED` | Capture confirmed |
| `CAPTURE_FAILED` | Capture failed |
| `VOID_SUCCEEDED` | Auth voided |
| `REFUND_SUCCEEDED` | Refund processed |
| `DISPUTE_OPENED` | Chargeback initiated |

### Adapter Responsibilities

Each adapter handles:

1. **Signature verification** (provider-specific method)
2. **Event parsing** (provider's JSON format)
3. **Event type mapping** (provider event → canonical event)
4. **Decline code mapping** (provider code → canonical code)
5. **Payload preservation** (store raw for debugging)
6. **Response formatting** (provider-expected acknowledgment)

## Consequences

### Positive

- **Isolated provider logic**: Adding a new provider requires only a new adapter
- **Simplified core system**: Workflows never see provider-specific types
- **Consistent error handling**: All declines classified using canonical codes
- **Easier testing**: Core system can be tested with mock canonical events
- **Clear contracts**: Adapter interface defines exactly what's required

### Negative

- **Mapping maintenance**: Must keep provider → canonical mappings updated
- **Potential data loss**: Normalization may discard provider-specific details
- **Initial complexity**: More files and interfaces than a simple implementation

### Mitigations

- Store raw webhook payload for debugging and future mapping needs
- Log unmapped event types and decline codes for monitoring
- Create comprehensive mapping tables based on provider documentation
- Use database-stored mappings that can be updated without code changes

## Implementation Notes

### File Structure

```
internal/adapter/
├── canonical.go          # Canonical types
├── decline_codes.go      # Canonical decline codes
├── stripe/
│   ├── adapter.go        # Webhook handler
│   ├── mapper.go         # Event/code mapping
│   └── signature.go      # Signature verification
├── adyen/
│   └── ...
└── paypal/
    └── ...
```

### Adding a New Provider

1. Create new adapter directory under `internal/adapter/`
2. Implement signature verification
3. Create event type mapping table
4. Create decline code mapping table
5. Implement the adapter interface
6. Register webhook endpoint
7. Add provider to enum

## References

- [Stripe Webhook Events](https://stripe.com/docs/api/events/types)
- [Adyen Notification Types](https://docs.adyen.com/development-resources/webhooks/understand-notifications)
- [PayPal Webhook Events](https://developer.paypal.com/docs/api-basics/notifications/webhooks/event-names/)
