# Payment Processing Service
## Requirements and Project Document

**Version:** 1.2  
**Last Updated:** January 2026  
**Project Type:** Learning Project / Portfolio Piece

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | Dec 2025 | Initial requirements |
| 1.1 | Jan 2026 | Added production-grade patterns (Intent/Method separation, transactional outbox, clearing accounts) |
| 1.2 | Jan 2026 | Added multi-provider adapter architecture, canonical event model |

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Business Context](#2-business-context)
3. [Goals and Objectives](#3-goals-and-objectives)
4. [Domain Model](#4-domain-model)
5. [Functional Requirements](#5-functional-requirements)
6. [Non-Functional Requirements](#6-non-functional-requirements)
7. [System Architecture](#7-system-architecture)
8. [Data Models](#8-data-models)
9. [User Stories](#9-user-stories)
10. [API Specifications](#10-api-specifications)
11. [Project Timeline](#11-project-timeline)
12. [Success Criteria](#12-success-criteria)
13. [Risks and Mitigations](#13-risks-and-mitigations)
14. [Future Enhancements](#14-future-enhancements)

---

## 1. Project Overview

### 1.1 Executive Summary

This project involves building a Payment Processing Service that demonstrates production-grade patterns derived from industry leaders including Stripe, Square/Block, and Adyen. The system combines intelligent payment retry logic with core banking transaction patterns relevant to credit unions and financial institutions.

**Fundamental Architectural Insight:** Payments are promises about money movement, not money movement itself. Every design decision flows from understanding the distinction between authorization (the promise) and settlement (the actual transfer).

**Multi-Provider Design:** The system ingests webhooks from multiple third-party payment providers (Stripe, Adyen, PayPal, etc.), normalizes them to a canonical model at the edge, and processes all payments through a unified workflow engine. Provider-specific logic exists only at system boundaries.

The service handles the complete payment lifecycle: intent creation, authorization, capture, recovery from failures, and settlement recording. It supports multiple payment types (card, ACH, internal transfers) and implements intelligent retry scheduling for recoverable failures.

### 1.2 Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| **Language** | Golang | Performance, strong typing, excellent concurrency support |
| **Workflow Engine** | Temporal | Durable execution, built-in retry handling, workflow state persistence |
| **Primary Database** | PostgreSQL | ACID compliance, robust for financial data, CDC support via logical replication |
| **Connection Pooler** | PgBouncer | Transaction pooling mode, reduces connection overhead |
| **Event Streaming** | Kafka with Debezium CDC | Transactional outbox pattern, schema registry support, event replay capability |
| **Schema Registry** | Confluent Schema Registry | Avro schemas with backward compatibility enforcement |
| **Payment Processors** | Stripe, Adyen, PayPal (test mode) | Industry-standard APIs, demonstrate multi-provider support |
| **API Framework** | Echo or Fiber | Lightweight, high-performance HTTP routing |
| **Containerization** | Docker Compose | Local development environment |

### 1.3 Key Architectural Principles

These principles are derived from studying production payment systems at scale:

| Principle | Description |
|-----------|-------------|
| **Exactly-once via at-least-once + idempotency** | True exactly-once delivery is impossible; achieve it through idempotent consumers |
| **Separate Intent, Method, and Order** | Avoid coupling what is paid, how it's paid, and what's purchased |
| **Authorization before Capture** | Model the promise (hold) separately from the claim (capture) |
| **Transactional Outbox** | Never dual-write to database and message broker |
| **Linear State Machines** | Avoid circular states; new attempts are new records |
| **Clearing Account Monitoring** | Non-zero clearing balances indicate unresolved issues |
| **Normalize at the Edge** | Provider-specific logic lives in adapters; core system speaks canonical model |

### 1.4 Target Audience

This project demonstrates skills relevant to:

- Data Engineer positions at credit unions (e.g., Vancity)
- Backend Engineer roles at fintech companies
- Platform Engineer positions at financial institutions
- Software Engineer roles focused on payment systems

---

## 2. Business Context

### 2.1 Problem Statement

Payment failures represent a significant challenge for businesses relying on recurring revenue:

- **Involuntary churn** accounts for approximately 50% of all subscriber churn
- Failed payments cost subscription businesses 10-35% of annual revenue
- Traditional retry systems use static schedules ignoring failure type nuances
- Financial institutions process millions of transactions requiring robust, auditable systems
- Race conditions between webhooks and API responses cause duplicate charges in poorly designed systems
- **Multi-provider complexity**: Each payment processor uses different schemas, event types, decline codes, and authentication methods

### 2.2 Industry Background

**Production System Insights:**

Stripe processes over 5 million queries per second using 5,000+ collections across 2,000+ database shards. Their key learning: building card-first abstractions creates technical debt—payment methods have fundamentally different lifecycles.

Square's Books ledger service manages approximately 20TB of data with a three-person team using Google Cloud Spanner, demonstrating how proper architecture reduces operational burden.

Adyen's stateless PAL (Payments Acceptance Layer) allows any instance to handle any payment without routing constraints, achieving linear scaling.

**Multi-Provider Reality:**

Modern payment systems rarely use a single provider:

| Reason | Example |
|--------|---------|
| Geographic coverage | Adyen for EU, Stripe for US |
| Cost optimization | Route based on interchange rates |
| Redundancy | Failover if primary provider is down |
| Feature requirements | PayPal for buyer protection, Stripe for subscriptions |
| Legacy integration | Existing contracts with specific providers |

**Butter Payments Approach:**

Intelligent retry timing recovers 166% more revenue than traditional dunning by analyzing 128+ data points per transaction to determine optimal retry timing.

**Credit Union Requirements:**

Financial institutions like Vancity require:

- Complete audit trails for regulatory compliance
- Double-entry bookkeeping with clearing account monitoring
- Support for scheduled and recurring payments
- Real-time and batch processing capabilities
- Event-driven architecture for downstream analytics
- Mathematical proof of correctness through balanced ledgers

### 2.3 Stakeholders

| Stakeholder | Interest |
|-------------|----------|
| **Portfolio Reviewer** | Evaluating technical competence and domain knowledge |
| **Hiring Manager** | Assessing fit for data/backend engineering roles |
| **Technical Interviewer** | Understanding architectural decisions and trade-offs |
| **Self (Developer)** | Learning Temporal, Golang, and payment domain patterns |

---

## 3. Goals and Objectives

### 3.1 Primary Goals

1. **Demonstrate Temporal Mastery**
   - Implement long-running workflows spanning hours or days
   - Use signals for external event handling (including provider webhooks)
   - Implement queries for workflow state inspection
   - Configure activity retry policies with proper error classification

2. **Implement Domain-Accurate Payment Processing**
   - Separate PaymentIntent, PaymentMethod, and Order concepts
   - Model authorization holds separately from capture
   - Proper decline classification with 2,000+ code support
   - Double-entry bookkeeping with clearing account monitoring

3. **Build Multi-Provider Architecture**
   - Provider-specific adapters for webhook ingestion
   - Canonical event model for internal processing
   - Unified decline code normalization across providers
   - Provider-agnostic workflow logic

4. **Build Production-Grade Patterns**
   - Transactional outbox with CDC for reliable event publishing
   - Idempotency with atomic phases and recovery points
   - Race condition prevention for webhook/API concurrent processing
   - Linear state machines with attempt tracking

5. **Create Portfolio-Ready Deliverable**
   - Well-documented codebase
   - Working local development environment
   - Comprehensive test coverage
   - Clear README with setup instructions

### 3.2 Learning Objectives

| Objective | Skill Demonstrated |
|-----------|-------------------|
| Temporal workflow development | Distributed systems, durable execution |
| Payment domain modeling | Financial services knowledge, industry patterns |
| Multi-provider integration | Adapter pattern, schema normalization |
| Event-driven architecture | Transactional outbox, CDC, schema evolution |
| API design | REST best practices, idempotency |
| Database schema design | Double-entry bookkeeping, audit trails |
| Testing strategies | Workflow testing, integration testing |

### 3.3 Out of Scope

- Production deployment infrastructure (Kubernetes, cloud providers)
- Real payment processing (test mode only)
- User authentication and authorization system
- Frontend/UI development
- Machine learning for retry optimization (rule-based approach used)
- Multi-currency support beyond USD
- International payment regulations
- PCI DSS compliance infrastructure
- Smart routing between providers (single provider per payment)

---

## 4. Domain Model

### 4.1 Core Domain Concepts

The domain model separates concerns to support diverse payment methods and providers without architectural rewrites:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DOMAIN MODEL                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐         │
│  │  PaymentIntent  │    │  PaymentMethod  │    │     Order       │         │
│  │                 │    │                 │    │                 │         │
│  │  WHAT is paid   │    │  HOW it's paid  │    │ WHAT's purchased│         │
│  │                 │    │                 │    │                 │         │
│  │  - Amount       │    │  - Card token   │    │  - Line items   │         │
│  │  - Currency     │    │  - Bank account │    │  - Merchant     │         │
│  │  - Customer     │    │  - Wallet       │    │  - Invoice ref  │         │
│  │  - Provider     │    │  - Type/Network │    │  - Metadata     │         │
│  │  - State        │    │  - Provider     │    │                 │         │
│  └────────┬────────┘    └────────┬────────┘    └────────┬────────┘         │
│           │                      │                      │                   │
│           └──────────────────────┼──────────────────────┘                   │
│                                  │                                          │
│                                  ▼                                          │
│                    ┌─────────────────────────┐                              │
│                    │   PaymentTransaction    │                              │
│                    │                         │                              │
│                    │   Links Intent + Method │                              │
│                    │   Tracks Attempts       │                              │
│                    │   Records Auth/Capture  │                              │
│                    │   Provider-Agnostic     │                              │
│                    └─────────────────────────┘                              │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                      CANONICAL EVENT MODEL                            │  │
│  │                                                                       │  │
│  │  All provider webhooks are normalized to this model before entering  │  │
│  │  the workflow engine. The core system never sees provider-specific   │  │
│  │  terminology.                                                         │  │
│  │                                                                       │  │
│  │  CanonicalPaymentEvent:                                               │  │
│  │    - EventType (AUTHORIZATION_SUCCEEDED, CAPTURE_FAILED, etc.)       │  │
│  │    - PaymentID (internal)                                            │  │
│  │    - ProviderPaymentID (external)                                    │  │
│  │    - Provider (STRIPE, ADYEN, PAYPAL)                                │  │
│  │    - Amount, Currency                                                 │  │
│  │    - Status, FailureCode (normalized)                                │  │
│  │    - RawPayload (preserved for debugging)                            │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 Payment Lifecycle

The payment lifecycle distinguishes between authorization (promise) and capture (claim):

| Phase | Description | Money Movement | Reversible |
|-------|-------------|----------------|------------|
| **Intent Created** | Payment request received | None | Yes |
| **Authorization** | Issuer approves, places hold | Hold on customer funds | Yes (void) |
| **Capture** | Merchant claims authorized funds | Initiates settlement | Limited (refund) |
| **Settlement** | Actual fund transfer | Funds move to acquirer | No |
| **Reconciliation** | Ledger entries recorded | Internal bookkeeping | No |

### 4.3 Authorization Hold Model

Authorization holds are first-class entities, not implicit states:

| Attribute | Description |
|-----------|-------------|
| **Hold ID** | Unique identifier for the authorization |
| **Amount** | Authorized amount (may differ from capture) |
| **Expiration** | When the hold expires (5-30 days by network) |
| **Status** | ACTIVE, CAPTURED, VOIDED, EXPIRED |
| **Capture Window** | Time remaining to capture |
| **Provider** | Which PSP holds the authorization |

### 4.4 Balance Model

Accounts maintain multiple balance states for accuracy:

| Balance Type | Description | Calculation |
|--------------|-------------|-------------|
| **Ledger Balance** | Sum of all settled transactions | Immutable entry sum |
| **Pending Balance** | Authorized but unsettled amounts | Active holds sum |
| **Available Balance** | Funds available for new transactions | Ledger - Pending debits |
| **Reserved Balance** | Funds reserved for scheduled payments | Scheduled payment sum |

### 4.5 Multi-Provider Canonical Model

The canonical event model abstracts provider differences:

**Canonical Event Types:**

| Canonical Type | Stripe Event | Adyen Notification | PayPal Event |
|----------------|--------------|-------------------|--------------|
| AUTHORIZATION_SUCCEEDED | payment_intent.succeeded | AUTHORISATION (success=true) | PAYMENT.AUTHORIZATION.CREATED |
| AUTHORIZATION_FAILED | payment_intent.payment_failed | AUTHORISATION (success=false) | PAYMENT.AUTHORIZATION.VOIDED |
| CAPTURE_SUCCEEDED | charge.captured | CAPTURE | PAYMENT.CAPTURE.COMPLETED |
| CAPTURE_FAILED | charge.failed | CAPTURE_FAILED | PAYMENT.CAPTURE.DENIED |
| REFUND_SUCCEEDED | charge.refunded | REFUND | PAYMENT.CAPTURE.REFUNDED |
| DISPUTE_OPENED | charge.dispute.created | CHARGEBACK | CUSTOMER.DISPUTE.CREATED |

**Canonical Decline Codes:**

| Canonical Code | Stripe | Adyen | PayPal |
|----------------|--------|-------|--------|
| INSUFFICIENT_FUNDS | insufficient_funds | Refused:51 | INSUFFICIENT_FUNDS |
| CARD_EXPIRED | expired_card | Refused:33 | CREDIT_CARD_EXPIRED |
| FRAUD_SUSPICION | fraudulent | Refused:59 | TRANSACTION_REFUSED |
| GENERIC_DECLINE | card_declined | Refused:05 | INSTRUMENT_DECLINED |
| DO_NOT_HONOR | do_not_honor | Refused:57 | DO_NOT_HONOR |

---

## 5. Functional Requirements

### 5.1 Payment Intent Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-INT-01 | System shall accept payment intent creation via REST API | Must Have |
| FR-INT-02 | System shall validate payment intent before processing | Must Have |
| FR-INT-03 | Payment intent shall be independent of payment method | Must Have |
| FR-INT-04 | System shall support updating payment method on existing intent | Must Have |
| FR-INT-05 | System shall support payment intent cancellation before capture | Must Have |
| FR-INT-06 | System shall use idempotency keys with atomic phases | Must Have |
| FR-INT-07 | System shall support attaching metadata to payment intents | Should Have |
| FR-INT-08 | Payment intent shall specify which provider to use | Must Have |

### 5.2 Authorization and Capture

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-AUTH-01 | System shall request authorization from payment processor | Must Have |
| FR-AUTH-02 | System shall track authorization holds as first-class entities | Must Have |
| FR-AUTH-03 | System shall support separate authorization and capture | Must Have |
| FR-AUTH-04 | System shall support immediate capture (auth + capture) | Must Have |
| FR-AUTH-05 | System shall track authorization expiration windows | Should Have |
| FR-AUTH-06 | System shall support partial capture (less than authorized) | Should Have |
| FR-AUTH-07 | System shall support void of uncaptured authorizations | Should Have |
| FR-AUTH-08 | System shall use provider-specific activities for PSP calls | Must Have |

### 5.3 Multi-Provider Adapter Layer

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-ADP-01 | System shall provide dedicated webhook endpoints per provider | Must Have |
| FR-ADP-02 | Each adapter shall verify provider-specific signatures/authentication | Must Have |
| FR-ADP-03 | Adapters shall normalize events to canonical model before processing | Must Have |
| FR-ADP-04 | Adapters shall preserve raw webhook payload for debugging | Must Have |
| FR-ADP-05 | Adapters shall map provider decline codes to canonical codes | Must Have |
| FR-ADP-06 | Adapters shall map provider event types to canonical event types | Must Have |
| FR-ADP-07 | Adapter failures shall return appropriate HTTP status to provider | Must Have |
| FR-ADP-08 | System shall support adding new providers without core changes | Should Have |

**Provider-Specific Verification Requirements:**

| Provider | Verification Method | Requirement |
|----------|-------------------|-------------|
| Stripe | Webhook signature with timestamp | Verify within 5 minutes of timestamp |
| Adyen | HMAC-SHA256 | Verify using shared secret |
| PayPal | Webhook ID verification API | Call PayPal to verify authenticity |

### 5.4 Decline Handling and Recovery

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-DEC-01 | System shall classify declines into categories | Must Have |
| FR-DEC-02 | System shall maintain decline code mapping per provider | Must Have |
| FR-DEC-03 | System shall normalize provider codes to canonical codes | Must Have |
| FR-DEC-04 | System shall automatically retry soft declines with intelligent timing | Must Have |
| FR-DEC-05 | System shall not retry hard declines or fraud flags | Must Have |
| FR-DEC-06 | System shall create new attempt records for each retry (linear state machine) | Must Have |
| FR-DEC-07 | System shall limit retry attempts to configurable maximum (default: 6) | Must Have |
| FR-DEC-08 | System shall align retry timing with paydays for insufficient funds | Should Have |
| FR-DEC-09 | System shall support immediate retry when payment method updated | Should Have |

### 5.5 Ledger and Bookkeeping

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-LED-01 | System shall use double-entry bookkeeping for all transactions | Must Have |
| FR-LED-02 | System shall maintain clearing accounts for in-flight transactions | Must Have |
| FR-LED-03 | System shall ensure every debit has a corresponding credit | Must Have |
| FR-LED-04 | System shall prevent ledger entry modification or deletion | Must Have |
| FR-LED-05 | System shall track ledger, pending, and available balances | Must Have |
| FR-LED-06 | System shall monitor clearing account balances (non-zero indicates issues) | Must Have |
| FR-LED-07 | System shall support balance reconciliation queries | Should Have |
| FR-LED-08 | System shall enforce daily transaction limits when configured | Should Have |

### 5.6 Event Publishing

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-EVT-01 | System shall use transactional outbox pattern for event publishing | Must Have |
| FR-EVT-02 | System shall never dual-write to database and message broker | Must Have |
| FR-EVT-03 | System shall use CDC (Debezium) to relay outbox events to Kafka | Must Have |
| FR-EVT-04 | System shall publish canonical events (not provider-specific) | Must Have |
| FR-EVT-05 | System shall use Avro schemas with backward compatibility | Should Have |
| FR-EVT-06 | System shall support event replay for consumer recovery | Should Have |
| FR-EVT-07 | Event consumers shall be idempotent (track processed message IDs) | Must Have |

### 5.7 Concurrency and Race Conditions

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-CON-01 | System shall handle simultaneous webhook and API responses safely | Must Have |
| FR-CON-02 | System shall use SELECT FOR UPDATE when processing payment updates | Must Have |
| FR-CON-03 | System shall use optimistic locking with version columns for balances | Must Have |
| FR-CON-04 | System shall prevent duplicate charges from concurrent processing | Must Have |
| FR-CON-05 | System shall handle provider webhook retries idempotently | Must Have |

### 5.8 Scheduled Payments

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-SCH-01 | System shall support one-time future-dated payments | Should Have |
| FR-SCH-02 | System shall support recurring payments (weekly, biweekly, monthly) | Should Have |
| FR-SCH-03 | System shall verify account status before executing scheduled payments | Should Have |
| FR-SCH-04 | System shall support cancellation of scheduled payments | Should Have |
| FR-SCH-05 | System shall reserve balance for upcoming scheduled payments | Nice to Have |

### 5.9 Audit and Compliance

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-AUD-01 | System shall log all payment state changes to audit log | Must Have |
| FR-AUD-02 | System shall record actor information for all changes | Must Have |
| FR-AUD-03 | System shall preserve original and new values for updates | Must Have |
| FR-AUD-04 | System shall timestamp all audit entries with microsecond precision | Must Have |
| FR-AUD-05 | System shall support audit log querying by entity, actor, or time range | Should Have |
| FR-AUD-06 | Audit log shall be append-only (no updates or deletes) | Must Have |
| FR-AUD-07 | System shall log provider and correlation IDs for webhook tracing | Must Have |

---

## 6. Non-Functional Requirements

### 6.1 Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-PERF-01 | Payment intent creation API response time | < 200ms (p95) |
| NFR-PERF-02 | Payment status query response time | < 100ms (p95) |
| NFR-PERF-03 | Workflow startup latency | < 500ms |
| NFR-PERF-04 | Concurrent payment processing capacity | 100+ simultaneous workflows |
| NFR-PERF-05 | CDC event latency (DB to Kafka) | < 100ms (p95) |
| NFR-PERF-06 | Ledger entry creation latency | < 50ms (p95) |
| NFR-PERF-07 | Webhook processing (signature verify + normalize) | < 50ms (p95) |

### 6.2 Reliability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-REL-01 | Payment processing must survive worker restarts | 100% |
| NFR-REL-02 | Duplicate payment prevention via idempotency | 100% |
| NFR-REL-03 | Ledger consistency (balanced entries) | 100% |
| NFR-REL-04 | Workflow state durability | Survives any single component failure |
| NFR-REL-05 | Event delivery guarantee | At-least-once with idempotent consumers |
| NFR-REL-06 | Zero data loss on component failure | Required |
| NFR-REL-07 | Webhook delivery acknowledgment to providers | Return 200 only after safe processing |

### 6.3 Scalability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-SCA-01 | Horizontal worker scaling | Workers can be added without code changes |
| NFR-SCA-02 | Database connection pooling | PgBouncer in transaction mode |
| NFR-SCA-03 | Stateless API servers | Multiple instances behind load balancer |
| NFR-SCA-04 | Kafka partition scaling | Support partition increase without rebalance issues |
| NFR-SCA-05 | Per-provider webhook scaling | Each adapter can scale independently |

### 6.4 Maintainability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-MNT-01 | Code documentation | Public functions documented with comments |
| NFR-MNT-02 | Test coverage (workflow code) | Minimum 70% coverage |
| NFR-MNT-03 | Test coverage (adapter code) | Minimum 80% coverage |
| NFR-MNT-04 | Configuration externalization | All environment-specific values via config |
| NFR-MNT-05 | Structured logging | JSON-formatted logs with correlation IDs |
| NFR-MNT-06 | Schema evolution | Backward-compatible Avro schema changes only |
| NFR-MNT-07 | Provider isolation | Adding new provider requires no core changes |

### 6.5 Security

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-SEC-01 | Sensitive data handling | No PCI data stored; use tokenized payment methods |
| NFR-SEC-02 | Database credentials | Environment variables or Vault, never hardcoded |
| NFR-SEC-03 | Webhook verification | Provider-specific signature validation required |
| NFR-SEC-04 | API key protection | Keys not logged or exposed in responses |
| NFR-SEC-05 | Row-level locking | Prevent race conditions on balance updates |
| NFR-SEC-06 | Provider secrets isolation | Each provider's credentials stored separately |

### 6.6 Observability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-OBS-01 | Structured logging | All operations logged with context |
| NFR-OBS-02 | Temporal UI integration | Workflows visible and inspectable |
| NFR-OBS-03 | Health check endpoints | API and worker health status |
| NFR-OBS-04 | Metrics exposure | Prometheus-compatible metrics |
| NFR-OBS-05 | Clearing account monitoring | Alert on non-zero clearing balances |
| NFR-OBS-06 | Latency percentile tracking | p50, p95, p99 for all operations |
| NFR-OBS-07 | Per-provider metrics | Track success/failure rates by provider |

---

## 7. System Architecture

### 7.1 Component Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Payment Processing Service                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                        ADAPTER LAYER                                   │  │
│  │                   (Normalize at the Edge)                              │  │
│  │                                                                        │  │
│  │  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐                  │  │
│  │  │   Stripe    │   │   Adyen     │   │   PayPal    │                  │  │
│  │  │   Adapter   │   │   Adapter   │   │   Adapter   │                  │  │
│  │  │             │   │             │   │             │                  │  │
│  │  │ POST /wh/   │   │ POST /wh/   │   │ POST /wh/   │                  │  │
│  │  │   stripe    │   │   adyen     │   │   paypal    │                  │  │
│  │  │             │   │             │   │             │                  │  │
│  │  │ - Verify    │   │ - Verify    │   │ - Verify    │                  │  │
│  │  │   signature │   │   HMAC      │   │   via API   │                  │  │
│  │  │ - Map events│   │ - Map events│   │ - Map events│                  │  │
│  │  │ - Normalize │   │ - Normalize │   │ - Normalize │                  │  │
│  │  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘                  │  │
│  │         │                 │                 │                          │  │
│  │         └─────────────────┼─────────────────┘                          │  │
│  │                           │                                            │  │
│  │                           ▼                                            │  │
│  │                 ┌───────────────────┐                                  │  │
│  │                 │  Canonical Event  │                                  │  │
│  │                 └───────────────────┘                                  │  │
│  └───────────────────────────┼───────────────────────────────────────────┘  │
│                              │                                              │
│  ┌───────────────────────────┼───────────────────────────────────────────┐  │
│  │                           ▼                                            │  │
│  │  ┌─────────────────────┐                                              │  │
│  │  │     API Server      │     ┌─────────────────────────────────────┐  │  │
│  │  │     (Stateless)     │────▶│         Temporal Workflows          │  │  │
│  │  │                     │     │                                     │  │  │
│  │  │  POST /intents      │     │  PaymentWorkflow                    │  │  │
│  │  │  POST /intents/:id/ │     │    ├─ ValidateIntent                │  │  │
│  │  │       authorize     │     │    ├─ RequestAuthorization          │  │  │
│  │  │  POST /intents/:id/ │     │    │   (provider-specific activity) │  │  │
│  │  │       capture       │     │    ├─ WaitForCapture (or immediate) │  │  │
│  │  │                     │     │    ├─ ProcessCapture                │  │  │
│  │  └─────────────────────┘     │    │   (provider-specific activity) │  │  │
│  │                              │    ├─ RecordLedgerEntries           │  │  │
│  │  ┌─────────────────────┐     │    └─ WriteToOutbox                 │  │  │
│  │  │   Signal Handlers   │────▶│                                     │  │  │
│  │  │                     │     │  RecoveryWorkflow                   │  │  │
│  │  │  - ProviderEvent    │     │    ├─ ClassifyDecline (canonical)   │  │  │
│  │  │  - UpdateMethod     │     │    ├─ CreateAttemptRecord           │  │  │
│  │  │  - CancelIntent     │     │    ├─ CalculateRetryTime            │  │  │
│  │  │  - ForceCapture     │     │    ├─ DurableSleep                  │  │  │
│  │  └─────────────────────┘     │    └─ RetryOrEscalate               │  │  │
│  │                              │                                     │  │  │
│  │  ┌─────────────────────┐     │  ScheduledPaymentWorkflow           │  │  │
│  │  │   Query Handlers    │◀────│    ├─ WaitForScheduledTime          │  │  │
│  │  │                     │     │    ├─ VerifyAccountStatus           │  │  │
│  │  │  - GetIntentStatus  │     │    └─ StartChildPaymentWorkflow     │  │  │
│  │  │  - GetAttempts      │     └─────────────────────────────────────┘  │  │
│  │  │  - GetHoldStatus    │                                              │  │
│  │  └─────────────────────┘                                              │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         ACTIVITY LAYER                                 │  │
│  │                                                                        │  │
│  │  Provider-Specific Activities      Provider-Agnostic Activities       │  │
│  │  ─────────────────────────────     ────────────────────────────       │  │
│  │  StripeAuthorizationActivity       RecordLedgerActivity               │  │
│  │  StripeCapureActivity              WriteOutboxActivity                │  │
│  │  AdyenAuthorizationActivity        ClassifyDeclineActivity            │  │
│  │  AdyenCaptureActivity              CheckAccountStatusActivity         │  │
│  │  PayPalAuthorizationActivity       CalculateRetryTimeActivity         │  │
│  │  PayPalCaptureActivity                                                │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         DATA LAYER                                     │  │
│  │                                                                        │  │
│  │  PostgreSQL                           Kafka                            │  │
│  │  ┌─────────────────────────────┐     ┌─────────────────────────┐      │  │
│  │  │ payment_intents             │     │ payments.authorized     │      │  │
│  │  │ authorization_holds         │     │ payments.captured       │      │  │
│  │  │ payment_attempts            │     │ payments.failed         │      │  │
│  │  │ ledger_entries              │     │ payments.recovering     │      │  │
│  │  │ accounts (with balances)    │     │ accounts.updated        │      │  │
│  │  │ clearing_accounts           │     │                         │      │  │
│  │  │ outbox                      │────▶│ (via Debezium CDC)      │      │  │
│  │  │ audit_log                   │     │                         │      │  │
│  │  │ decline_code_mappings       │     │ (Canonical events only) │      │  │
│  │  └─────────────────────────────┘     └─────────────────────────┘      │  │
│  │                                                                        │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Adapter Layer Detail

The adapter layer normalizes all provider-specific data at the system boundary:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ADAPTER LAYER DETAIL                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Stripe Webhook                                                            │
│   POST /webhooks/stripe                                                     │
│         │                                                                   │
│         ▼                                                                   │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                      StripeAdapter                                   │  │
│   │                                                                      │  │
│   │  1. Verify Signature                                                 │  │
│   │     - Extract Stripe-Signature header                               │  │
│   │     - Verify timestamp within tolerance (5 min)                     │  │
│   │     - Validate HMAC-SHA256 signature                                │  │
│   │     - Reject with 401 if invalid                                    │  │
│   │                                                                      │  │
│   │  2. Parse Event                                                      │  │
│   │     - Unmarshal JSON to Stripe event struct                         │  │
│   │     - Extract event type (e.g., "charge.succeeded")                 │  │
│   │     - Extract relevant data from event object                       │  │
│   │                                                                      │  │
│   │  3. Map to Canonical                                                 │  │
│   │     - "charge.succeeded" → CAPTURE_SUCCEEDED                        │  │
│   │     - "insufficient_funds" → INSUFFICIENT_FUNDS                     │  │
│   │     - Extract amount, currency, IDs                                 │  │
│   │                                                                      │  │
│   │  4. Preserve Raw Payload                                             │  │
│   │     - Store original JSON for debugging                             │  │
│   │     - Include provider timestamp                                    │  │
│   │                                                                      │  │
│   │  5. Signal Workflow or Start New                                     │  │
│   │     - Look up workflow by provider payment ID                       │  │
│   │     - Signal existing workflow with canonical event                 │  │
│   │     - Return 200 to Stripe                                          │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│         │                                                                   │
│         ▼                                                                   │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │                    CanonicalPaymentEvent                             │  │
│   │                                                                      │  │
│   │  {                                                                   │  │
│   │    "event_type": "CAPTURE_SUCCEEDED",                               │  │
│   │    "payment_id": "pay_abc123",                                      │  │
│   │    "provider_payment_id": "ch_xyz789",                              │  │
│   │    "provider": "STRIPE",                                            │  │
│   │    "amount": 10000,                                                 │  │
│   │    "currency": "USD",                                               │  │
│   │    "status": "SUCCEEDED",                                           │  │
│   │    "failure_code": null,                                            │  │
│   │    "raw_payload": { ... },                                          │  │
│   │    "received_at": "2026-01-17T10:30:00Z"                           │  │
│   │  }                                                                   │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.3 Transactional Outbox Pattern

The system never dual-writes to database and Kafka. All events flow through the outbox:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     TRANSACTIONAL OUTBOX PATTERN                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Application                                                               │
│       │                                                                     │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────┐                      │
│   │              Single Transaction                  │                      │
│   │                                                  │                      │
│   │   1. UPDATE payment_intents SET status = ...    │                      │
│   │   2. INSERT INTO ledger_entries (...)           │                      │
│   │   3. INSERT INTO outbox (event_type, payload)   │                      │
│   │      (Canonical event - no provider specifics)  │                      │
│   │                                                  │                      │
│   │   COMMIT                                         │                      │
│   └─────────────────────────────────────────────────┘                      │
│                           │                                                 │
│                           ▼                                                 │
│   ┌─────────────────────────────────────────────────┐                      │
│   │           Debezium CDC Connector                 │                      │
│   │                                                  │                      │
│   │   - Reads PostgreSQL WAL (write-ahead log)      │                      │
│   │   - Captures outbox table changes               │                      │
│   │   - Publishes to Kafka topics                   │                      │
│   │   - Maintains exactly-once semantics            │                      │
│   └─────────────────────────────────────────────────┘                      │
│                           │                                                 │
│                           ▼                                                 │
│   ┌─────────────────────────────────────────────────┐                      │
│   │                 Kafka Topics                     │                      │
│   │                                                  │                      │
│   │   payments.authorized  (canonical events)       │                      │
│   │   payments.captured    (canonical events)       │                      │
│   │   payments.failed      (canonical events)       │                      │
│   │                                                  │                      │
│   └─────────────────────────────────────────────────┘                      │
│                           │                                                 │
│                           ▼                                                 │
│   ┌─────────────────────────────────────────────────┐                      │
│   │          Idempotent Consumers                    │                      │
│   │                                                  │                      │
│   │   - Track processed message IDs                 │                      │
│   │   - Handle duplicates gracefully                │                      │
│   │   - Analytics, notifications, reporting         │                      │
│   │   - Consumers never see provider-specific data  │                      │
│   └─────────────────────────────────────────────────┘                      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.4 Payment Flow Sequence

**Inbound Flow (Provider Webhook):**

1. Provider sends webhook to `/webhooks/{provider}`
2. Adapter verifies signature using provider-specific method
3. Adapter parses event and maps to canonical model
4. Adapter looks up existing workflow by provider payment ID
5. Adapter signals workflow with canonical event
6. Workflow processes event using provider-agnostic logic
7. Return 200 to provider

**Outbound Flow (API-Initiated Payment):**

1. Client creates PaymentIntent via `POST /api/v1/intents` (specifies provider)
2. Workflow starts, selects provider-specific authorization activity
3. Activity calls provider API with idempotency key
4. On success: Create hold, update balances, write outbox
5. CDC relays canonical event to Kafka
6. Client can poll for status or receive webhook

### 7.5 Idempotency with Atomic Phases

Each operation is broken into phases with recovery points:

| Phase | Description | Recovery Point |
|-------|-------------|----------------|
| **started** | Request received, idempotency key stored | Can restart from beginning |
| **validated** | Input validation passed | Skip validation on retry |
| **authorized** | Authorization received from provider | Skip authorization, use stored result |
| **captured** | Capture confirmed | Skip capture, use stored result |
| **ledger_updated** | Ledger entries written | Skip ledger update |
| **event_written** | Outbox event written | Skip event write |
| **completed** | All phases complete | Return cached response |

---

## 8. Data Models

### 8.1 PaymentIntent Entity

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique payment intent identifier |
| idempotency_key | String | Client-provided key preventing duplicates |
| customer_id | String | Customer reference |
| amount | Decimal(19,4) | Payment amount |
| currency | String(3) | ISO 4217 currency code |
| status | Enum | CREATED, REQUIRES_METHOD, REQUIRES_AUTH, AUTHORIZED, CAPTURED, FAILED, CANCELLED |
| capture_method | Enum | AUTOMATIC, MANUAL |
| provider | Enum | STRIPE, ADYEN, PAYPAL |
| provider_payment_id | String | Provider's payment/charge ID |
| payment_method_id | UUID | Reference to attached payment method |
| metadata | JSONB | Custom key-value data |
| created_at | Timestamp | Intent creation time |
| updated_at | Timestamp | Last modification time |

### 8.2 AuthorizationHold Entity

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique hold identifier |
| payment_intent_id | UUID | Parent payment intent |
| amount | Decimal(19,4) | Authorized amount |
| currency | String(3) | ISO 4217 currency code |
| status | Enum | ACTIVE, CAPTURED, VOIDED, EXPIRED |
| provider | Enum | STRIPE, ADYEN, PAYPAL |
| processor_auth_code | String | Provider's authorization code |
| expires_at | Timestamp | When the hold expires |
| captured_amount | Decimal(19,4) | Amount captured (may be partial) |
| captured_at | Timestamp | Capture timestamp |
| created_at | Timestamp | Authorization timestamp |

### 8.3 PaymentAttempt Entity (Linear State Machine)

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique attempt identifier |
| payment_intent_id | UUID | Parent payment intent |
| attempt_number | Integer | Sequential attempt number |
| status | Enum | PENDING, PROCESSING, SUCCEEDED, FAILED |
| provider | Enum | STRIPE, ADYEN, PAYPAL |
| provider_response_code | String | Raw provider response code |
| canonical_decline_code | String | Normalized decline code |
| decline_type | Enum | SOFT, HARD, FRAUD, TEMPORARY |
| processor_txn_id | String | Provider's transaction ID |
| idempotency_key | String | Attempt-specific idempotency key |
| created_at | Timestamp | Attempt start time |
| completed_at | Timestamp | Attempt completion time |

### 8.4 Account Entity (Multi-Balance)

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique account identifier |
| type | Enum | CHECKING, SAVINGS, LOAN, CLEARING |
| owner_id | String | Customer identifier |
| ledger_balance | Decimal(19,4) | Sum of settled transactions |
| pending_balance | Decimal(19,4) | Authorized but unsettled |
| available_balance | Decimal(19,4) | Ledger minus pending debits |
| reserved_balance | Decimal(19,4) | Reserved for scheduled payments |
| currency | String(3) | Account currency |
| status | Enum | ACTIVE, FROZEN, CLOSED |
| daily_limit | Decimal(19,4) | Maximum daily outflow |
| version | Integer | Optimistic locking version |
| created_at | Timestamp | Account creation time |
| updated_at | Timestamp | Last balance update time |

### 8.5 Decline Code Mapping Entity

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique mapping identifier |
| provider | Enum | STRIPE, ADYEN, PAYPAL |
| provider_code | String | Raw provider decline code |
| canonical_code | String | Normalized internal code |
| decline_type | Enum | SOFT, HARD, FRAUD, TEMPORARY |
| description | String | Human-readable description |
| retry_eligible | Boolean | Whether retry is permitted |
| suggested_action | String | Recommended customer action |

### 8.6 Canonical Event Types

| Event Type | Description |
|------------|-------------|
| AUTHORIZATION_SUCCEEDED | Authorization approved, hold placed |
| AUTHORIZATION_FAILED | Authorization declined |
| CAPTURE_SUCCEEDED | Capture confirmed |
| CAPTURE_FAILED | Capture failed |
| VOID_SUCCEEDED | Authorization voided |
| REFUND_SUCCEEDED | Refund processed |
| REFUND_FAILED | Refund failed |
| DISPUTE_OPENED | Chargeback initiated |
| DISPUTE_CLOSED | Dispute resolved |

### 8.7 Clearing Accounts

Clearing accounts track in-flight transactions for monitoring:

| Account | Purpose | Expected Balance |
|---------|---------|------------------|
| **payment_clearing** | Tracks funds between authorization and capture | Near-zero (temporary holds) |
| **settlement_clearing** | Tracks funds awaiting bank settlement | Varies by settlement cycle |
| **fee_clearing** | Tracks collected fees awaiting disbursement | Near-zero |

**Monitoring Rule:** Non-zero clearing account balances exceeding 24 hours indicate unresolved issues requiring investigation.

---

## 9. User Stories

### 9.1 Payment Intent Creation

**US-01: Create a payment intent**
> As an API consumer, I want to create a payment intent specifying the provider so that I can begin the payment process.

Acceptance Criteria:
- Payment intent created with amount, currency, customer, and provider
- Intent status is CREATED or REQUIRES_METHOD
- Idempotency key prevents duplicate intents
- Intent ID returned for subsequent operations

**US-02: Attach payment method to intent**
> As an API consumer, I want to attach a payment method to an intent so that I can proceed to authorization.

Acceptance Criteria:
- Payment method can be attached via PUT request
- Intent status transitions to REQUIRES_AUTH
- Payment method can be updated until authorization
- Invalid payment methods rejected with clear error

### 9.2 Multi-Provider Webhooks

**US-03: Receive Stripe webhook**
> As the system, I want to receive and process Stripe webhooks so that payment status is updated.

Acceptance Criteria:
- Stripe signature verified before processing
- Event mapped to canonical model
- Existing workflow signaled with canonical event
- Return 200 to Stripe within timeout

**US-04: Receive Adyen notification**
> As the system, I want to receive and process Adyen notifications so that payment status is updated.

Acceptance Criteria:
- HMAC signature verified before processing
- Notification mapped to canonical model
- Existing workflow signaled with canonical event
- Return "[accepted]" to Adyen

### 9.3 Decline Recovery

**US-05: Automatic retry on soft decline**
> As a business, I want failed payments automatically retried at intelligent times regardless of provider so that I recover more revenue.

Acceptance Criteria:
- Provider decline codes mapped to canonical codes
- Soft declines classified correctly using canonical code
- New PaymentAttempt record created for each retry
- Retry timing considers decline type (not provider)
- Maximum attempts enforced

---

## 10. API Specifications

### 10.1 Endpoints Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/v1/intents | Create a payment intent |
| GET | /api/v1/intents/:id | Get payment intent status |
| PUT | /api/v1/intents/:id/method | Attach payment method |
| POST | /api/v1/intents/:id/authorize | Request authorization |
| POST | /api/v1/intents/:id/capture | Capture authorized funds |
| POST | /api/v1/intents/:id/cancel | Cancel payment intent |
| GET | /api/v1/intents/:id/attempts | Get attempt history |
| GET | /api/v1/intents/:id/hold | Get authorization hold status |
| POST | /api/v1/scheduled | Create scheduled payment |
| GET | /api/v1/scheduled/:id | Get schedule status |
| DELETE | /api/v1/scheduled/:id | Cancel schedule |
| GET | /api/v1/accounts/:id | Get account with balances |
| GET | /api/v1/accounts/:id/ledger | Get ledger entries |
| POST | /webhooks/stripe | Receive Stripe webhooks |
| POST | /webhooks/adyen | Receive Adyen notifications |
| POST | /webhooks/paypal | Receive PayPal webhooks |
| GET | /health | Health check |
| GET | /metrics | Prometheus metrics |

### 10.2 Create Intent Request

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| amount | string | Yes | Decimal amount (e.g., "100.00") |
| currency | string | Yes | ISO 4217 code |
| customer_id | string | Yes | Customer reference |
| provider | string | Yes | STRIPE, ADYEN, or PAYPAL |
| capture_method | string | No | "automatic" or "manual" |
| payment_method_id | string | No | Pre-attach method |
| metadata | object | No | Custom key-value pairs |

### 10.3 Error Responses

| Status Code | Error Type | When Used |
|-------------|------------|-----------|
| 400 | validation_error | Invalid input format or values |
| 401 | authentication_error | Invalid webhook signature |
| 404 | not_found | Resource does not exist |
| 409 | conflict | Idempotency conflict or invalid state transition |
| 422 | business_rule_violation | Insufficient funds, limit exceeded |
| 500 | internal_error | System error |

---

## 11. Project Timeline

### Phase 1: Foundation (Week 1-2)

**Objectives:**
- Establish project structure with domain model separation
- Set up infrastructure with CDC pipeline
- Implement core database schema including decline mappings

**Deliverables:**
- Go module with Intent/Method/Attempt separation
- Docker Compose with Temporal, PostgreSQL, Kafka, Debezium
- Database migrations including outbox table and decline mappings
- Basic API server with health check
- Temporal worker connecting to server
- Debezium CDC connector configured

**Milestone:** CDC pipeline operational; outbox events appearing in Kafka

### Phase 2: Adapter Layer (Week 3-4)

**Objectives:**
- Implement provider-specific adapters
- Define canonical event model
- Build decline code mapping system

**Deliverables:**
- StripeAdapter with signature verification
- AdyenAdapter with HMAC verification
- Canonical event type definitions
- Decline code mapping tables (50+ codes per provider)
- Adapter unit tests with provider fixtures

**Milestone:** Adapters normalize test webhooks to canonical events

### Phase 3: Authorization Flow (Week 5-6)

**Objectives:**
- Implement PaymentWorkflow with authorization
- Build provider-specific authorization activities
- Implement multi-balance account model

**Deliverables:**
- PaymentIntent creation and validation
- Provider-specific authorization activities
- AuthorizationHold entity management
- Multi-balance tracking (ledger, pending, available)
- Query handlers for workflow state

**Milestone:** Authorizations creating holds via any provider

### Phase 4: Capture and Ledger (Week 7-8)

**Objectives:**
- Implement capture flow with ledger entries
- Build double-entry bookkeeping with clearing accounts
- Implement transactional outbox integration

**Deliverables:**
- Provider-specific capture activities
- Double-entry ledger with clearing accounts
- Clearing account balance monitoring
- Outbox event writing in same transaction
- Canonical event publication via CDC

**Milestone:** Complete auth/capture flow with balanced ledger entries

### Phase 5: Recovery and Production Readiness (Week 9-10)

**Objectives:**
- Implement decline classification using canonical codes
- Build retry workflow
- Achieve test coverage targets

**Deliverables:**
- ClassifyDecline activity using canonical codes
- RecoveryWorkflow with intelligent retry
- Signal handlers for method update and cancellation
- Webhook handlers with race condition protection
- Unit and integration tests
- API and architecture documentation

**Milestone:** All tests pass; documentation complete; demo ready

---

## 12. Success Criteria

### 12.1 Functional Completeness

| Criterion | Measurement |
|-----------|-------------|
| Multi-provider webhooks work | All three adapters normalize correctly |
| Canonical model used internally | No provider-specific types in workflows |
| Intent/Method separation | Payment method can be changed without new intent |
| Authorization holds work | Pending balance reflects active holds |
| Capture records ledger entries | Balanced double-entry entries created |
| Clearing accounts monitored | Non-zero balances generate alerts |
| Retry uses canonical codes | Decline classification provider-agnostic |
| CDC publishes canonical events | Events appear in Kafka within 100ms |

### 12.2 Architectural Compliance

| Criterion | Measurement |
|-----------|-------------|
| Normalize at edge | Workflows never see provider types |
| No dual-writes | All events through outbox only |
| Linear state machines | No circular PaymentAttempt states |
| Idempotent operations | Duplicate requests return same result |
| Race conditions prevented | Concurrent webhook/API handled safely |

### 12.3 Code Quality

| Criterion | Target |
|-----------|--------|
| Test coverage (workflow code) | >= 70% |
| Test coverage (adapter code) | >= 80% |
| Test coverage (activity code) | >= 70% |
| Linting passes | Zero warnings |
| No hardcoded credentials | All via environment/Vault |

---

## 13. Risks and Mitigations

### 13.1 Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| CDC complexity | Medium | High | Start with simple Debezium config; expand gradually |
| Temporal learning curve | Medium | Medium | Use official tutorials; start with simple workflows |
| Provider API differences | Medium | Medium | Thorough adapter testing with real webhook samples |
| Decline code mapping gaps | Medium | Medium | Start with common codes; log unmapped codes |
| Race condition bugs | High | High | Use SELECT FOR UPDATE consistently; test concurrency |

### 13.2 Schedule Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Scope creep | Medium | High | Strict adherence to out-of-scope list |
| CDC setup delays | Medium | Medium | Have simple polling fallback initially |
| Provider documentation gaps | Medium | Low | Use test mode webhooks extensively |

---

## 14. Future Enhancements

### 14.1 Near-Term

- Additional providers (Square, Braintree)
- Multi-currency support with exchange rates
- Partial refund workflow
- 3D Secure authentication flow

### 14.2 Medium-Term

- Smart routing between providers (cost optimization)
- ML-based retry optimization
- Fraud scoring integration
- Real-time analytics dashboard

### 14.3 Long-Term

- Multi-tenant architecture
- Cross-border payment support
- Regulatory reporting automation
- Provider failover automation

---

## Appendix A: Decline Code Categories

| Category | Examples | Retry Eligible | Typical Resolution |
|----------|----------|----------------|-------------------|
| **Soft - Funds** | INSUFFICIENT_FUNDS, OVER_LIMIT | Yes | Wait for payday |
| **Soft - Temporary** | TRY_AGAIN, PROCESSING_ERROR | Yes | Retry in hours |
| **Soft - Generic** | DO_NOT_HONOR, GENERIC_DECLINE | Yes | Retry with timing variation |
| **Hard - Card** | CARD_EXPIRED, INVALID_NUMBER | No | Request new card |
| **Hard - Account** | ACCOUNT_CLOSED, RESTRICTED | No | Contact customer |
| **Fraud** | FRAUD_SUSPICION, STOLEN_CARD | No | Flag for review |
| **Temporary - System** | RATE_LIMIT, TIMEOUT | Yes (activity-level) | Exponential backoff |

---

## Appendix B: Glossary

| Term | Definition |
|------|------------|
| **Adapter** | Component that normalizes provider-specific webhooks to canonical model |
| **Authorization Hold** | A temporary hold on customer funds pending capture |
| **Canonical Model** | Provider-agnostic internal representation of payment events |
| **Capture** | The action of claiming previously authorized funds |
| **CDC** | Change Data Capture - streaming database changes to event bus |
| **Clearing Account** | Internal account tracking in-flight transactions |
| **Idempotency Key** | Client-provided key ensuring duplicate requests are safe |
| **Linear State Machine** | State machine where states only transition forward |
| **Outbox Pattern** | Writing events to database table for reliable delivery |
| **PaymentAttempt** | Single attempt to process a payment (immutable record) |
| **PaymentIntent** | The abstract concept of a payment to be made |
| **PaymentMethod** | The instrument used to make a payment |
| **Provider** | Third-party payment processor (Stripe, Adyen, PayPal) |
| **Settlement** | Actual transfer of funds between financial institutions |

---

*Document prepared for portfolio project demonstrating production-grade payment processing patterns.*