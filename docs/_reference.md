# Documentation Reference

Quick navigation to all documentation in this project.

## Getting Started

| Document | Purpose |
|----------|---------|
| [Documentation Overview](readme.md) | Start here - project overview and development process |
| [Glossary](glossary.md) | Terms and definitions for payment and technical concepts |
| [Service Requirements](requirements/service-requirements.md) | What we're building and why |
| [Technical Specification](requirements/technical-spec.md) | How we're building it |

## Research

| Document | Purpose |
|----------|---------|
| [Payment Systems Research](research/payment-systems-research.md) | Industry patterns from Stripe, Square, Adyen - foundational reading for understanding our architectural choices |

## Architecture Decisions

Why we made the choices we did. Read these to understand the reasoning behind the system design.

| Document | Decision |
|----------|----------|
| [ADR-001](decisions/001-temporal-workflow-engine.md) | Why Temporal for workflow orchestration |
| [ADR-002](decisions/002-multi-provider-adapters.md) | How we handle multiple payment providers |
| [ADR-003](decisions/003-transactional-outbox.md) | How we publish events reliably |
| [ADR-004](decisions/004-double-entry-bookkeeping.md) | How we track money movement |
| [Decision Log](decisions/readme.md) | Index of all ADRs with summaries |
| [Pending Decisions](decisions/pending-decisions.md) | Decisions awaiting input - **review and answer** |

## API Documentation

Reference for implementing and testing endpoints.

| Document | Covers |
|----------|--------|
| [API Overview](api/readme.md) | Authentication, error formats, idempotency |
| [Payment Intents](api/internal/intents.md) | Create, authorize, capture payments |
| [Accounts](api/internal/accounts.md) | Balance queries and ledger entries |
| [Health & Metrics](api/internal/health.md) | Health checks and Prometheus metrics |

### Webhook Receivers

How we receive and process notifications from payment providers.

| Document | Provider |
|----------|----------|
| [Stripe Webhooks](api/webhooks/stripe.md) | Stripe signature verification, event mapping |
| [Adyen Webhooks](api/webhooks/adyen.md) | Adyen HMAC verification, notification handling |
| [PayPal Webhooks](api/webhooks/paypal.md) | PayPal verification API, event mapping |

## Database Schema

Reference for database structure and usage patterns.

| Document | Covers |
|----------|--------|
| [Schema Overview](schema/readme.md) | Database design principles, migration info |
| [Core Tables](schema/core-tables.md) | payment_intents, payment_methods, payment_attempts, authorization_holds, decline_code_mappings |
| [Ledger Tables](schema/ledger-tables.md) | accounts, journal_entries, ledger_entries, clearing accounts |
| [Outbox & Audit](schema/outbox-audit.md) | outbox, audit_log, processed_events |

## Testing

| Document | Purpose |
|----------|---------|
| [Simulations Overview](simulations/readme.md) | Using the webhook simulator for testing |
| [Webhook Simulator README](../tools/webhook-simulator/README.md) | Full simulator documentation |

## By Task

### "I need to understand the system"
1. [Documentation Overview](readme.md)
2. [Glossary](glossary.md) - understand the terminology
3. [Payment Systems Research](research/payment-systems-research.md)
4. [Service Requirements](requirements/service-requirements.md)

### "I need to make a decision"
1. [Pending Decisions](decisions/pending-decisions.md) - decisions awaiting input
2. [Decision Log](decisions/readme.md) - how past decisions were made

### "I need to implement a new feature"
1. [Technical Specification](requirements/technical-spec.md)
2. Relevant ADR in [decisions/](decisions/readme.md)
3. [Schema documentation](schema/readme.md)

### "I need to add a new payment provider"
1. [ADR-002: Multi-Provider Adapters](decisions/002-multi-provider-adapters.md)
2. Existing webhook docs: [Stripe](api/webhooks/stripe.md), [Adyen](api/webhooks/adyen.md), [PayPal](api/webhooks/paypal.md)
3. [Core Tables](schema/core-tables.md) - decline_code_mappings section

### "I need to test webhook handling"
1. [Simulations Overview](simulations/readme.md)
2. [Webhook Simulator README](../tools/webhook-simulator/README.md)

### "I need to debug a payment issue"
1. [Schema: Outbox & Audit](schema/outbox-audit.md) - audit_log queries
2. [Schema: Core Tables](schema/core-tables.md) - payment_attempts for history
3. [Schema: Ledger Tables](schema/ledger-tables.md) - clearing account monitoring

### "I don't understand a term"
1. [Glossary](glossary.md) - payment, technical, and project-specific terms
