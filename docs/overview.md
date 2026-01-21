# Payment Processing Service

A learning project demonstrating production-grade payment processing patterns with Go and Temporal.

---

## Project Overview

This service implements durable workflow orchestration for payment operations, combining intelligent retry logic with core banking transaction patterns. It demonstrates patterns derived from studying Stripe, Square/Block, Adyen, and major financial institutions.

### Core Insight

Payments are promises about money movement, not money movement itself. Every design decision flows from understanding the distinction between authorization (the promise) and settlement (the actual transfer).

### What This Project Demonstrates

| Pattern | Description |
|---------|-------------|
| Durable Workflows | Temporal-based orchestration surviving crashes and restarts |
| Multi-Provider Support | Adapters for Stripe, Adyen, PayPal with canonical normalization |
| Intelligent Retry | Decline classification with context-aware scheduling |
| Double-Entry Bookkeeping | Mathematically provable correctness |
| Transactional Outbox | CDC-based event publishing preventing dual-write issues |
| Idempotent Operations | Atomic phases with recovery points |

---

## Learning Objectives

| Objective | Skills Demonstrated |
|-----------|---------------------|
| Temporal workflow development | Distributed systems, durable execution, signals, queries |
| Payment domain modeling | Financial services knowledge, industry patterns |
| Multi-provider integration | Adapter pattern, schema normalization |
| Event-driven architecture | Transactional outbox, CDC, schema evolution |
| API design | REST best practices, idempotency |
| Database schema design | Double-entry bookkeeping, audit trails |

---

## Business Context

### The Problem

Payment failures represent a significant challenge for businesses relying on recurring revenue:

- Involuntary churn accounts for approximately 50% of all subscriber churn
- Failed payments cost subscription businesses 10-35% of annual revenue
- Traditional retry systems use static schedules ignoring failure type nuances
- Race conditions between webhooks and API responses cause duplicate charges
- Multi-provider complexity: Each processor uses different schemas, event types, and decline codes

### Industry Background

Modern payment systems rarely use a single provider:

| Reason | Example |
|--------|---------|
| Geographic coverage | Adyen for EU, Stripe for US |
| Cost optimization | Route based on interchange rates |
| Redundancy | Failover if primary provider is down |
| Feature requirements | PayPal for buyer protection |

---

## What's In Scope

### Implemented Patterns

- Payment intent lifecycle (create, authorize, capture, void, refund)
- Multi-provider webhook ingestion with signature verification
- Canonical event model for provider-agnostic processing
- Decline code normalization and classification
- Recovery workflows with intelligent retry scheduling
- Double-entry ledger with clearing account monitoring
- Transactional outbox with CDC to Kafka

### Intentionally Simplified

See [Finalized Decisions](decisions/finalized-decisions.md) for details on each simplification.

| Feature | Learning Implementation | Production Would Add |
|---------|------------------------|---------------------|
| Rate limiting | Global fixed limits | Tiered per-merchant limits |
| Retry timing | Fixed intervals | ML-optimized, payday-aligned |
| Webhook delivery | Internal Kafka consumers | External HTTP endpoints |
| Idempotency | 24-hour client-provided keys | Longer expiration, hybrid keys |
| Sandbox | Provider test modes | Dedicated sandbox environment |
| Partial captures | Single partial capture | Multiple partial captures |

---

## What's Out of Scope

These are documented in [Deferred Decisions](decisions/deferred-decisions.md):

- Production deployment infrastructure (Kubernetes, cloud providers)
- Real payment processing (test mode only)
- User authentication and authorization system
- Frontend/UI development
- Machine learning for retry optimization
- Multi-currency support beyond USD
- International payment regulations
- PCI DSS compliance infrastructure
- Smart routing between providers
- Database sharding

---

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Language | Go 1.24 | Performance, strong typing, concurrency |
| Workflow Engine | Temporal | Durable execution, retry handling |
| Database | PostgreSQL | ACID compliance, CDC support |
| Connection Pooler | PgBouncer | Transaction pooling |
| Event Streaming | Kafka | Event replay, schema registry |
| CDC | Debezium | Database change capture |

### Simple vs Production Path

Two infrastructure paths are available. See [Technical Spec](architecture/system-design.md) for details.

**Simple Path** (recommended for getting started):
- Poll outbox table directly (no Kafka/Debezium setup)
- JSON events with version field
- Direct PostgreSQL connection

**Production Path** (available in docker-compose):
- CDC with Debezium to Kafka
- Avro schemas with Schema Registry
- PostgreSQL with PgBouncer

Both paths work with the same codebase.

---

## Target Audience

This project demonstrates skills relevant to:

- Data Engineer positions at credit unions
- Backend Engineer roles at fintech companies
- Platform Engineer positions at financial institutions
- Software Engineer roles focused on payment systems

---

## Documentation Index

| Document | Description |
|----------|-------------|
| [Architecture Overview](architecture/system-design.md) | System design and layer responsibilities |
| [Domain Model](architecture/domain-model.md) | Entities, relationships, and state machines |
| [Functional Requirements](requirements/functional.md) | What the system does |
| [Non-Functional Requirements](requirements/non-functional.md) | Performance, reliability, security |
| [Finalized Decisions](decisions/finalized-decisions.md) | 19 decisions with simple defaults |
| [Deferred Decisions](decisions/deferred-decisions.md) | 7 enterprise-scale decisions |
| [Glossary](glossary.md) | Terms and definitions |

---

*A learning project demonstrating production-grade payment processing patterns.*
