# Decisions

This directory contains all architectural and design decisions for the payment processing service.

## Decision Categories

### Finalized Decisions

Decisions resolved with **simple defaults** appropriate for a learning project. Each notes what production systems do differently.

See [Finalized Decisions](finalized-decisions.md) - 19 decisions:

| ID | Decision | Simple Default |
|----|----------|---------------|
| FD-001 | Retry Timing | Fixed intervals (4h, 12h, 24h, 48h) |
| FD-002 | Webhook Delivery | Kafka only (internal consumers) |
| FD-003 | Idempotency Keys | Client-provided, 24h expiration |
| FD-004 | Hold Expiration | Provider defaults, void on expire |
| FD-005 | Rate Limiting | Global fixed limits (100 req/min) |
| FD-006 | Sandbox | Provider test modes directly |
| FD-007 | Payment Methods | Provider tokens only |
| FD-008 | Disputes | Notification only, no evidence |
| FD-009 | Audit Logs | 90-day retention in database |
| FD-010 | Notifications | Events only via Kafka |
| FD-011 | Temporal Workflows | Single cluster, basic config |
| FD-012 | Multi-Provider Adapters | Static code mappings |
| FD-013 | Transactional Outbox | CDC to Kafka (or polling) |
| FD-014 | Double-Entry Ledger | Entries on capture/refund only |
| FD-015 | Failure Handling | Log and manual intervention |
| FD-016 | API Versioning | Date-based (Stripe-style) |
| FD-017 | API Authentication | Key + secret pairs |
| FD-018 | Timeout Handling | Fail and retry |
| FD-019 | Partial Captures | Single partial (Stripe-style) |

### Deferred Decisions

Decisions that are **beyond the scope** of this learning project. Documented to understand what production systems need.

See [Deferred Decisions](deferred-decisions.md) - 7 decisions:

| ID | Decision | Why Deferred |
|----|----------|--------------|
| DD-001 | Database Sharding | Scale beyond learning scope |
| DD-002 | Multi-Currency | Adds complexity without new patterns |
| DD-003 | Smart Routing | Requires historical data and ML |
| DD-004 | Reporting/Analytics | Consumer of data, not core processing |
| DD-005 | Merchant KYC | Compliance domain, not payments |
| DD-006 | SLAs | Operational commitment, not architecture |
| DD-007 | Data Residency | Multi-region infrastructure |

### Pending Decisions

All decisions resolved. See [Pending Decisions](pending-decisions.md) for template to add new decisions.

---

## Quick Links

- [Finalized Decisions](finalized-decisions.md) - 19 decisions with simple defaults
- [Deferred Decisions](deferred-decisions.md) - 7 enterprise-scale decisions
- [Pending Decisions](pending-decisions.md) - Template for new decisions
