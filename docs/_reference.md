# Documentation Reference

Quick navigation to all documentation in this project.

## Getting Started

| Document | Purpose |
|----------|---------|
| [Documentation Overview](readme.md) | Start here - project overview and development process |
| [Project Overview](overview.md) | Goals, scope, and technology stack |
| [Glossary](glossary.md) | Terms and definitions for payment and technical concepts |
| [Implementation Plan](implementation-plan.md) | Phased implementation roadmap with task breakdown |

## Architecture

| Document | Purpose |
|----------|---------|
| [System Design](architecture/system-design.md) | High-level architecture, layers, data flow |
| [Domain Model](architecture/domain-model.md) | Entities, relationships, state machines |

## Requirements

| Document | Purpose |
|----------|---------|
| [Functional Requirements](requirements/functional.md) | What the system does (FR-xxx) |
| [Non-Functional Requirements](requirements/non-functional.md) | Performance, reliability, security (NFR-xxx) |

## Research

| Document | Purpose |
|----------|---------|
| [Payment Systems Research](research/payment-systems-research.md) | Industry patterns from Stripe, Square, Adyen |

## Architecture Decisions

Why we made the choices we did. Read these to understand the reasoning behind the system design.

| Document | Purpose |
|----------|---------|
| [Decision Index](decisions/readme.md) | Overview of all decisions |
| [Finalized Decisions](decisions/finalized-decisions.md) | 19 decisions with simple defaults for learning |
| [Deferred Decisions](decisions/deferred-decisions.md) | 7 enterprise-scale decisions (documented only) |

Key architectural decisions in finalized-decisions.md:
- FD-011: Temporal Workflows
- FD-012: Multi-Provider Adapters
- FD-013: Transactional Outbox
- FD-014: Double-Entry Ledger
- FD-015: Failure Handling
- FD-016: API Versioning (Stripe-style)
- FD-017: API Authentication
- FD-018: Timeout Handling
- FD-019: Partial Captures

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
2. [Project Overview](overview.md) - goals and scope
3. [Glossary](glossary.md) - understand the terminology
4. [System Design](architecture/system-design.md) - architecture overview
5. [Payment Systems Research](research/payment-systems-research.md)

### "I need to understand a decision"
1. [Decision Log](decisions/readme.md) - index of all decisions
2. [Finalized Decisions](decisions/finalized-decisions.md) - simple defaults chosen
3. [Deferred Decisions](decisions/deferred-decisions.md) - what production systems do

### "I need to implement a new feature"
1. [System Design](architecture/system-design.md)
2. [Domain Model](architecture/domain-model.md)
3. Relevant decision in [finalized-decisions.md](decisions/finalized-decisions.md)
4. [Schema documentation](schema/readme.md)

### "I need to add a new payment provider"
1. [FD-012: Multi-Provider Adapters](decisions/finalized-decisions.md) in finalized decisions
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
