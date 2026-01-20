# Payment Processing Service Documentation

This directory contains all documentation for the Payment Processing Service project. The documentation follows a waterfall-hybrid approach where design and documentation are completed before implementation.

## Documentation Structure

```
docs/
├── _reference.md           # Quick navigation to all documents
├── glossary.md             # Terms and definitions
├── readme.md               # This file
├── research/               # Background research and industry patterns
├── api/                    # API documentation
│   ├── internal/           # Our REST API endpoints
│   └── webhooks/           # Provider webhook endpoints we receive
├── simulations/            # Webhook simulators and testing tools
├── decisions/              # Architectural Decision Records (ADRs)
│   └── pending-decisions.md # Decisions awaiting input
├── requirements/           # Functional and non-functional requirements
└── schema/                 # Database schema documentation
```

## Quick Links

| Document | Purpose |
|----------|---------|
| [Reference Index](_reference.md) | Navigate to any document by task or topic |
| [Glossary](glossary.md) | Understand payment and technical terminology |
| [Pending Decisions](decisions/pending-decisions.md) | **Decisions awaiting your input** |

## Document Index

### Reference
- [Glossary](glossary.md) - Terms and definitions for payment, technical, and project concepts

### Research
- [Payment Systems Research](research/payment-systems-research.md) - Industry patterns from Stripe, Square, Adyen

### API Documentation
- [Internal API Overview](api/readme.md) - Our REST API endpoints
  - [Payment Intents](api/internal/intents.md) - Intent creation, authorization, capture
  - [Accounts](api/internal/accounts.md) - Account and balance queries
  - [Health](api/internal/health.md) - Health and metrics endpoints
- Webhook Receivers
  - [Stripe Webhooks](api/webhooks/stripe.md) - Stripe webhook handling
  - [Adyen Webhooks](api/webhooks/adyen.md) - Adyen notification handling
  - [PayPal Webhooks](api/webhooks/paypal.md) - PayPal webhook handling

### Simulations
- [Simulation Overview](simulations/readme.md) - Testing with webhook simulators

### Architecture Decisions
- [Decision Log](decisions/readme.md) - Index of all ADRs with summaries
- [Pending Decisions](decisions/pending-decisions.md) - **17 decisions awaiting input**
- [ADR-001: Temporal Workflow Engine](decisions/001-temporal-workflow-engine.md)
- [ADR-002: Multi-Provider Adapter Pattern](decisions/002-multi-provider-adapters.md)
- [ADR-003: Transactional Outbox Pattern](decisions/003-transactional-outbox.md)
- [ADR-004: Double-Entry Bookkeeping](decisions/004-double-entry-bookkeeping.md)

### Requirements
- [Service Requirements](requirements/service-requirements.md) - Full requirements document
- [Technical Specification](requirements/technical-spec.md) - Technical implementation spec

### Database Schema
- [Schema Overview](schema/readme.md) - Database design documentation
- [Core Tables](schema/core-tables.md) - Payment intents, attempts, methods
- [Ledger Tables](schema/ledger-tables.md) - Double-entry bookkeeping schema
- [Outbox and Audit](schema/outbox-audit.md) - Event publishing and audit trails

## Development Process

This project follows a waterfall-hybrid methodology:

1. **Research Phase** - Study industry patterns and best practices
2. **Requirements Phase** - Define functional and non-functional requirements
3. **Design Phase** - Document architecture decisions and technical specifications
4. **Decision Phase** - Review and answer [pending decisions](decisions/pending-decisions.md)
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
| 1.3 | Jan 2026 | Restructured ADRs, added pending decisions and glossary |
