# ADR-002: Multi-Provider Adapter Pattern

## Status

Accepted

## Problem

Modern payment systems rarely use a single payment provider. Organizations need multiple providers for geographic coverage, cost optimization, redundancy, and feature requirements. Each provider has different webhook formats, signature verification methods, decline codes, and API response structures.

**The core challenge**: Without careful design, provider-specific logic spreads throughout the codebase, creating tight coupling. This makes it difficult to add new providers and creates maintenance burden when providers change their APIs.

| Challenge | Impact |
|-----------|--------|
| Different webhook formats | Core business logic must understand each provider's JSON structure |
| Different event names | Same business event has different names per provider |
| Different decline codes | Same decline reason has different codes per provider |
| Different signature methods | Verification logic scattered across codebase |

## Solutions Considered

### Solution A: Direct Provider Integration

Each provider's types and logic used directly throughout the codebase.

| Pros | Cons |
|------|------|
| Simple initial implementation | Provider logic spreads everywhere |
| No abstraction overhead | Adding new provider touches many files |
| Direct access to provider features | Testing requires mocking each provider |
| | Provider API changes have wide impact |

### Solution B: Provider Facade

Single facade class that wraps all providers with conditional logic.

| Pros | Cons |
|------|------|
| Single point of access | Large, complex facade class |
| Hides some provider differences | Switch statements grow with each provider |
| | Still exposes some provider-specific types |
| | Difficult to test individual providers |

### Solution C: Adapter Pattern with Canonical Model

Provider-specific adapters at the edge normalize to canonical types. Core system only uses canonical types.

| Pros | Cons |
|------|------|
| Core system is provider-agnostic | Mapping maintenance required |
| Adding provider = adding adapter only | Normalization may discard provider details |
| Clear interface contracts | More files and interfaces initially |
| Easy to test with mock canonical events | |
| Provider changes isolated to adapter | |

## Chosen Solution

**Solution C: Adapter Pattern with Canonical Model**

### Architecture

| Layer | Components | Responsibility |
|-------|------------|----------------|
| Adapter Layer | Stripe Adapter, Adyen Adapter, PayPal Adapter | Normalize provider-specific formats |
| Canonical Model | Canonical Payment Event | Provider-agnostic data structure |
| Core System | Workflows, Activities, Ledger, Outbox | Business logic using only canonical types |

Data flows from provider webhooks through adapters, which transform provider-specific formats into canonical events. The core system processes only canonical events and never directly interacts with provider-specific types.

### Canonical Event Model

| Field | Type | Purpose |
|-------|------|---------|
| EventType | Enum | Canonical event type |
| PaymentID | String | Internal payment intent ID |
| ProviderPaymentID | String | Provider's reference ID |
| Provider | Enum | Which provider (STRIPE, ADYEN, PAYPAL) |
| Amount | Integer | Amount in smallest currency unit |
| Currency | String | ISO 4217 currency code |
| Status | Enum | Canonical payment status |
| FailureCode | Optional | Canonical decline code (if failed) |
| RawPayload | JSON | Original provider payload for debugging |
| ReceivedAt | Timestamp | When event was received |

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
| `DISPUTE_WON` | Dispute resolved in merchant favor |
| `DISPUTE_LOST` | Dispute resolved in customer favor |

### Adapter Responsibilities

Each adapter must handle:

| Responsibility | Description |
|----------------|-------------|
| Signature verification | Use provider-specific method (HMAC, API call, etc.) |
| Event parsing | Parse provider's JSON format |
| Event type mapping | Map provider event to canonical event type |
| Decline code mapping | Map provider decline code to canonical code |
| Payload preservation | Store raw payload for debugging |
| Response formatting | Return provider-expected acknowledgment |

## Why This Solution

| Reason | Explanation |
|--------|-------------|
| **Isolation** | Provider-specific code is contained in adapters. Core business logic never changes when adding or modifying providers. |
| **Testability** | Core system tests use mock canonical events. No need to mock provider APIs for business logic tests. |
| **Maintainability** | When a provider changes their API or adds new event types, only one adapter file changes. |
| **Consistency** | All decline codes are classified using canonical codes. Retry logic and error handling work the same regardless of provider. |
| **Extensibility** | Adding a new provider requires only implementing a new adapter. Core system requires zero changes. |
| **Debugging** | Raw payload preserved allows investigation of provider-specific issues without losing data. |

### Trade-off Acceptance

We accept the trade-offs of this approach:

| Trade-off | Mitigation |
|-----------|------------|
| Mapping maintenance | Store mappings in database for updates without code changes |
| Potential data loss in normalization | Preserve raw payload for debugging and future mapping needs |
| Initial complexity | Clear interface contracts make the pattern easy to follow |

## Adding a New Provider

When adding a new payment provider:

1. Create new adapter module for the provider
2. Implement signature verification per provider docs
3. Create event type mapping table (provider events to canonical)
4. Create decline code mapping table (provider codes to canonical)
5. Implement the adapter interface
6. Register webhook endpoint in API routing
7. Add provider to the provider enum
8. Seed decline code mappings in database

## References

- [Stripe Webhook Events](https://stripe.com/docs/api/events/types)
- [Adyen Notification Types](https://docs.adyen.com/development-resources/webhooks/understand-notifications)
- [PayPal Webhook Events](https://developer.paypal.com/docs/api-basics/notifications/webhooks/event-names/)
