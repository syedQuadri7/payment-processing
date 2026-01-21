# Payment Processing Service Documentation

This directory contains all documentation for the Payment Processing Service project. The documentation follows a waterfall-hybrid approach where design and documentation are completed before implementation.

## Learning Scope

This is a **learning project** focused on understanding production-grade payment processing patterns. It intentionally simplifies certain aspects to focus on core concepts.

### What's Implemented

| Pattern | Description | Status |
|---------|-------------|--------|
| Temporal Workflows | Durable execution for payment lifecycle | Implemented |
| Multi-Provider Adapters | Normalize webhooks to canonical events | Implemented |
| Transactional Outbox | Reliable event publishing via CDC | Implemented |
| Double-Entry Bookkeeping | Mathematical correctness for money movement | Implemented |
| Idempotency | Exactly-once via at-least-once + idempotent consumers | Implemented |

### What's Intentionally Simplified

| Feature | Learning Implementation | Production Would Add |
|---------|------------------------|---------------------|
| Rate limiting | Global fixed limits | Tiered per-merchant |
| Retry timing | Fixed intervals | ML-optimized, payday-aligned |
| Webhook delivery | Internal Kafka consumers | External HTTP endpoints |
| Sandbox | Provider test modes | Dedicated sandbox environment |
| Dispute handling | Notification only | Evidence collection |
| Notifications | Events only | Email/SMS/push |

See [Finalized Decisions](decisions/finalized-decisions.md) for details on each simplification.

### What's Beyond Scope

| Feature | Why Deferred |
|---------|--------------|
| Database sharding | Scale beyond learning needs |
| Multi-currency | Complexity without new patterns |
| Smart routing | Requires ML/historical data |
| Merchant KYC | Compliance domain |

See [Deferred Decisions](decisions/deferred-decisions.md) for what production systems implement.

### Two Infrastructure Paths

| Path | Use When |
|------|----------|
| **Simple** | Getting started, focusing on payment patterns |
| **Production** | Learning CDC, Kafka, infrastructure patterns |

See [System Design](architecture/system-design.md) for details.

---

## Documentation Structure

```
docs/
+-- readme.md               # This file
+-- overview.md             # Project overview and learning objectives
+-- glossary.md             # Terms and definitions
+-- architecture/           # System design documentation
|   +-- system-design.md    # High-level architecture and data flow
|   +-- domain-model.md     # Entities, relationships, state machines
+-- requirements/           # What the system does
|   +-- functional.md       # Functional requirements (FR-xxx)
|   +-- non-functional.md   # Non-functional requirements (NFR-xxx)
+-- decisions/              # Architecture decisions
|   +-- readme.md           # Decision index
|   +-- finalized-decisions.md  # 19 decisions with simple defaults
|   +-- deferred-decisions.md   # 7 enterprise-scale decisions
|   +-- pending-decisions.md    # Template for new decisions
+-- api/                    # API documentation
|   +-- internal/           # REST API endpoints
|   +-- webhooks/           # Provider webhook receivers
+-- schema/                 # Database schema documentation
+-- research/               # Industry patterns and background
+-- simulations/            # Testing tools
```

## Quick Links

| Document | Purpose |
|----------|---------|
| [Project Overview](overview.md) | What this project is and why |
| [Glossary](glossary.md) | Payment and technical terminology |
| [System Design](architecture/system-design.md) | Architecture and data flow |
| [Decision Log](decisions/readme.md) | All decisions and their status |

## Document Index

### Overview
- [Project Overview](overview.md) - Goals, scope, technology stack

### Architecture
- [System Design](architecture/system-design.md) - High-level architecture, layers, data flow
- [Domain Model](architecture/domain-model.md) - Entities, relationships, state machines

### Requirements
- [Functional Requirements](requirements/functional.md) - What the system does (FR-xxx)
- [Non-Functional Requirements](requirements/non-functional.md) - Performance, reliability, security (NFR-xxx)

### Decisions
- [Decision Index](decisions/readme.md) - Overview of all decisions
- [Finalized Decisions](decisions/finalized-decisions.md) - 19 decisions with simple defaults
- [Deferred Decisions](decisions/deferred-decisions.md) - 7 enterprise-scale decisions
- [Pending Decisions](decisions/pending-decisions.md) - Template for new decisions

### Reference
- [Glossary](glossary.md) - Terms and definitions

### API Documentation
- [Internal API Overview](api/readme.md) - Our REST API endpoints
  - [Payment Intents](api/internal/intents.md) - Intent creation, authorization, capture
  - [Accounts](api/internal/accounts.md) - Account and balance queries
  - [Health](api/internal/health.md) - Health and metrics endpoints
- Webhook Receivers
  - [Stripe Webhooks](api/webhooks/stripe.md) - Stripe webhook handling
  - [Adyen Webhooks](api/webhooks/adyen.md) - Adyen notification handling
  - [PayPal Webhooks](api/webhooks/paypal.md) - PayPal webhook handling

### Database Schema
- [Schema Overview](schema/readme.md) - Database design documentation
- [Core Tables](schema/core-tables.md) - Payment intents, attempts, methods
- [Ledger Tables](schema/ledger-tables.md) - Double-entry bookkeeping schema
- [Outbox and Audit](schema/outbox-audit.md) - Event publishing and audit trails

### Research
- [Payment Systems Research](research/payment-systems-research.md) - Industry patterns from Stripe, Square, Adyen

### Simulations
- [Simulation Overview](simulations/readme.md) - Testing with webhook simulators

## Development Process

This project follows a waterfall-hybrid methodology:

1. **Research Phase** - Study industry patterns and best practices
2. **Requirements Phase** - Define functional and non-functional requirements
3. **Design Phase** - Document architecture decisions and technical specifications
4. **Decision Phase** - Review and finalize decisions
5. **Implementation Phase** - Build according to specifications
6. **Testing Phase** - Validate against requirements

Documentation should be updated as decisions are made and before code is written.

## Key Principles

From studying production payment systems at Stripe, Square, and Adyen:

| Principle | Description |
|-----------|-------------|
| **Payments are promises** | Authorization is a promise; settlement is money movement |
| **Separate Intent, Method, Order** | Avoid coupling what is paid, how, and what's purchased |
| **Transactional Outbox** | Never dual-write to database and message broker |
| **Linear State Machines** | Avoid circular states; new attempts are new records |
| **Normalize at the Edge** | Provider-specific logic lives in adapters only |
| **Idempotency First** | Exactly-once via at-least-once + idempotent consumers |

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | Dec 2025 | Initial documentation structure |
| 1.1 | Jan 2026 | Added production-grade patterns |
| 1.2 | Jan 2026 | Added multi-provider adapter architecture |
| 1.3 | Jan 2026 | Restructured decisions, added glossary |
| 1.4 | Jan 2026 | Reorganized docs: separated architecture, requirements |
