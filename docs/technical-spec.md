# Payment Processing Service
## Technical Specification

**Version:** 1.1  
**A Learning Project with Golang and Temporal**

Designed for Credit Union / Banking Domain Experience  
Incorporating Production Patterns from Stripe, Square, and Adyen

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architectural Principles](#2-architectural-principles)
3. [System Architecture](#3-system-architecture)
4. [Domain Model Design](#4-domain-model-design)
5. [Temporal Workflow Design](#5-temporal-workflow-design)
6. [Activity Design](#6-activity-design)
7. [Database Schema](#7-database-schema)
8. [Event System Design](#8-event-system-design)
9. [API Design](#9-api-design)
10. [Idempotency Implementation](#10-idempotency-implementation)
11. [Concurrency and Race Conditions](#11-concurrency-and-race-conditions)
12. [Observability Design](#12-observability-design)
13. [Project Structure](#13-project-structure)
14. [Implementation Roadmap](#14-implementation-roadmap)
15. [Testing Strategy](#15-testing-strategy)
16. [Infrastructure Configuration](#16-infrastructure-configuration)

---

## 1. Executive Summary

### 1.1 Project Overview

This specification describes a Payment Processing Service demonstrating production-grade patterns derived from studying Stripe, Square/Block, Adyen, and major financial institutions. The service implements durable workflow orchestration using Temporal, enabling payment operations that may span hours or days.

**Core Architectural Truth:** Payments are promises about money movement, not money movement itself. Every design decision flows from understanding the distinction between authorization (the promise) and settlement (the actual transfer).

**Key Insight:** Exactly-once payment processing is achieved through at-least-once delivery combined with idempotent consumers. True exactly-once delivery is theoretically impossible in distributed systems.

### 1.2 Learning Objectives

- Master Temporal workflow patterns including signals, queries, and durable timers
- Implement the separation of Intent, Method, and Order
- Build authorization/capture flow with hold tracking
- Implement transactional outbox pattern with CDC
- Design double-entry bookkeeping with clearing account monitoring
- Practice idempotency with atomic phases

### 1.3 Technology Decisions

| Component | Choice | Rationale |
|-----------|--------|-----------|
| **Language** | Golang | Performance, strong typing, Temporal SDK maturity |
| **Workflow Engine** | Temporal | Durable execution, battle-tested at Stripe, Uber, Netflix |
| **Primary Database** | PostgreSQL | ACID compliance, logical replication for CDC |
| **Connection Pooler** | PgBouncer | Transaction pooling mode, reduces connection overhead |
| **Event Streaming** | Kafka | Event replay capability, schema registry support |
| **CDC** | Debezium | Captures WAL changes with minimal database load |
| **Schema Registry** | Confluent | Avro with backward compatibility enforcement |
| **Payment Processor** | Stripe (test mode) | Industry-standard API, excellent documentation |

---

## 2. Architectural Principles

### 2.1 Principles from Production Systems

These principles are derived from studying systems processing billions of transactions:

#### Principle 1: Separate Intent, Method, and Order

Stripe's engineering team acknowledged: "We built abstractions designed for the simplest payment method—cards. It's as if we were trying to build a spaceship by adding parts to a car."

**Application:** PaymentIntent, PaymentMethod, and Order are distinct entities. Payment methods can be attached, updated, or swapped without creating new intents.

#### Principle 2: Authorization Before Capture

Authorization places a hold on customer funds. Capture claims those funds. These are separate operations with different lifecycles.

**Application:** AuthorizationHold is a first-class entity with its own state, expiration tracking, and capture window.

#### Principle 3: Transactional Outbox

The dual-write problem—needing to update a database AND publish an event atomically—has no solution through distributed transactions in practice.

**Application:** Events are written to an outbox table within the same database transaction. Debezium CDC relays events to Kafka asynchronously.

#### Principle 4: Linear State Machines

Circular state machines (FAILED → PENDING → FAILED) indicate design flaws. Each cycle represents a new attempt that should be tracked separately.

**Application:** PaymentAttempt records are immutable. Each retry creates a new attempt with its own linear state progression.

#### Principle 5: Clearing Account Monitoring

Stripe uses a "water flow" analogy: money flows through pipes into reservoirs. At steady state, intermediate clearing accounts should be near-zero.

**Application:** Non-zero clearing account balances exceeding thresholds trigger alerts indicating unresolved issues.

#### Principle 6: Idempotency with Atomic Phases

Operations are broken into phases with recovery points. Each phase handles either local database operations OR external API calls, never both.

**Application:** Idempotency records store the current phase, enabling reliable resumption from any failure point.

### 2.2 Anti-Patterns to Avoid

| Anti-Pattern | Risk | Mitigation |
|--------------|------|------------|
| **Card-first abstractions** | Other payment methods don't fit | Separate Intent from Method |
| **Circular state machines** | Audit confusion, debugging difficulty | Linear states with attempt records |
| **Dual-writes** | Data loss, inconsistency | Transactional outbox pattern |
| **Synchronous PSP handling only** | Race conditions, duplicate charges | Handle webhook + API concurrently |
| **Distributed monolith** | Tight coupling despite services | Clear API boundaries |
| **Soft outages ignored** | Latency degradation undetected | Percentile-based alerting |

---

## 3. System Architecture

### 3.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         PAYMENT PROCESSING SERVICE                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                          API LAYER (Stateless)                        │  │
│  │                                                                       │  │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │  │
│  │   │   Intents   │  │   Methods   │  │  Accounts   │  │  Webhooks  │  │  │
│  │   │   Handler   │  │   Handler   │  │   Handler   │  │  Handler   │  │  │
│  │   └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └─────┬──────┘  │  │
│  │          │                │                │               │         │  │
│  └──────────┼────────────────┼────────────────┼───────────────┼─────────┘  │
│             │                │                │               │            │
│             ▼                ▼                ▼               ▼            │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                      TEMPORAL CLIENT                                  │  │
│  │                                                                       │  │
│  │   StartWorkflow  │  SignalWorkflow  │  QueryWorkflow                 │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│             │                                                              │
│             ▼                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                      TEMPORAL SERVER                                  │  │
│  │                                                                       │  │
│  │   Workflow State  │  Task Queues  │  Timers  │  Event History        │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│             │                                                              │
│             ▼                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                      TEMPORAL WORKERS                                 │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────┐    ┌─────────────────────────┐         │  │
│  │   │    PaymentWorkflow      │    │    RecoveryWorkflow     │         │  │
│  │   │                         │    │                         │         │  │
│  │   │  ├─ ValidateIntent      │    │  ├─ ClassifyDecline     │         │  │
│  │   │  ├─ RequestAuth         │    │  ├─ CreateAttempt       │         │  │
│  │   │  ├─ TrackHold           │    │  ├─ CalculateRetry      │         │  │
│  │   │  ├─ ProcessCapture      │    │  ├─ DurableSleep        │         │  │
│  │   │  ├─ RecordLedger        │    │  └─ RetryOrEscalate     │         │  │
│  │   │  └─ WriteOutbox         │    │                         │         │  │
│  │   └─────────────────────────┘    └─────────────────────────┘         │  │
│  │                                                                       │  │
│  │   ┌─────────────────────────┐                                        │  │
│  │   │  ScheduledWorkflow      │                                        │  │
│  │   │                         │                                        │  │
│  │   │  ├─ WaitForTime         │                                        │  │
│  │   │  ├─ VerifyAccount       │                                        │  │
│  │   │  └─ StartChildWorkflow  │                                        │  │
│  │   └─────────────────────────┘                                        │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│             │                                                              │
│             ▼                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         DATA LAYER                                    │  │
│  │                                                                       │  │
│  │   ┌────────────────────────────────────────────────────────────────┐ │  │
│  │   │                      PostgreSQL                                 │ │  │
│  │   │                                                                 │ │  │
│  │   │  payment_intents  │  authorization_holds  │  payment_attempts  │ │  │
│  │   │  payment_methods  │  accounts             │  ledger_entries    │ │  │
│  │   │  clearing_accounts│  outbox               │  audit_log         │ │  │
│  │   │  idempotency_keys │  decline_codes        │                    │ │  │
│  │   └────────────────────────────────────────────────────────────────┘ │  │
│  │             │                                                        │  │
│  │             │ (WAL / Logical Replication)                            │  │
│  │             ▼                                                        │  │
│  │   ┌────────────────────────────────────────────────────────────────┐ │  │
│  │   │                    Debezium CDC                                 │ │  │
│  │   │                                                                 │ │  │
│  │   │  Captures outbox inserts  │  Transforms to CloudEvents         │ │  │
│  │   └────────────────────────────────────────────────────────────────┘ │  │
│  │             │                                                        │  │
│  │             ▼                                                        │  │
│  │   ┌────────────────────────────────────────────────────────────────┐ │  │
│  │   │                         Kafka                                   │ │  │
│  │   │                                                                 │ │  │
│  │   │  payments.authorized  │  payments.captured  │  payments.failed │ │  │
│  │   │  accounts.updated     │  ledger.entries                        │ │  │
│  │   └────────────────────────────────────────────────────────────────┘ │  │
│  │                                                                       │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                      EXTERNAL INTEGRATIONS                            │  │
│  │                                                                       │  │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                  │  │
│  │   │   Stripe    │  │ Circuit     │  │  Metrics    │                  │  │
│  │   │   API       │  │ Breaker     │  │ (Prometheus)│                  │  │
│  │   └─────────────┘  └─────────────┘  └─────────────┘                  │  │
│  │                                                                       │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Component Responsibilities

| Component | Responsibility | Scaling Strategy |
|-----------|----------------|------------------|
| **API Server** | HTTP endpoints, request validation, workflow orchestration | Horizontal, stateless |
| **Temporal Server** | Workflow state, task scheduling, timer management | Cluster mode (Docker for dev) |
| **Temporal Workers** | Execute workflow and activity code | Horizontal, KEDA-based |
| **PostgreSQL** | Persistent storage, ACID transactions | Vertical + read replicas |
| **PgBouncer** | Connection pooling in transaction mode | Sidecar per worker |
| **Debezium** | CDC from outbox table to Kafka | Single instance (HA optional) |
| **Kafka** | Event streaming, consumer decoupling | Partition-based |
| **Schema Registry** | Avro schema management, compatibility | Single instance |

### 3.3 Payment Flow Sequence

**Authorization Flow:**

```
┌────────┐     ┌─────────┐     ┌──────────┐     ┌────────┐     ┌─────────┐
│ Client │     │   API   │     │ Temporal │     │ Worker │     │ Stripe  │
└───┬────┘     └────┬────┘     └────┬─────┘     └───┬────┘     └────┬────┘
    │               │               │              │               │
    │ POST /intents │               │              │               │
    │──────────────▶│               │              │               │
    │               │               │              │               │
    │               │ StartWorkflow │              │               │
    │               │──────────────▶│              │               │
    │               │               │              │               │
    │               │               │ ScheduleTask │               │
    │               │               │─────────────▶│               │
    │               │               │              │               │
    │   202 Accepted│               │              │               │
    │◀──────────────│               │              │               │
    │               │               │              │               │
    │ POST /intents/:id/authorize   │              │               │
    │──────────────▶│               │              │               │
    │               │               │              │               │
    │               │ SignalWorkflow│              │               │
    │               │──────────────▶│              │               │
    │               │               │              │               │
    │               │               │ WakeWorkflow │               │
    │               │               │─────────────▶│               │
    │               │               │              │               │
    │               │               │              │ CreateCharge  │
    │               │               │              │──────────────▶│
    │               │               │              │               │
    │               │               │              │   AuthCode    │
    │               │               │              │◀──────────────│
    │               │               │              │               │
    │               │               │              │ [DB Transaction]
    │               │               │              │ - Create Hold │
    │               │               │              │ - Update Balance
    │               │               │              │ - Write Outbox│
    │               │               │              │               │
    │   200 OK (hold details)       │              │               │
    │◀──────────────│◀──────────────│◀─────────────│               │
    │               │               │              │               │
```

**Capture Flow:**

```
    │ POST /intents/:id/capture     │              │               │
    │──────────────▶│               │              │               │
    │               │               │              │               │
    │               │ SignalWorkflow│              │               │
    │               │──────────────▶│              │               │
    │               │               │              │               │
    │               │               │              │ CaptureCharge │
    │               │               │              │──────────────▶│
    │               │               │              │               │
    │               │               │              │   Confirmed   │
    │               │               │              │◀──────────────│
    │               │               │              │               │
    │               │               │              │ [DB Transaction]
    │               │               │              │ - Update Hold │
    │               │               │              │ - Ledger Entries
    │               │               │              │ - Write Outbox│
    │               │               │              │               │
    │   200 OK (captured)           │              │               │
    │◀──────────────│◀──────────────│◀─────────────│               │
```

---

## 4. Domain Model Design

### 4.1 Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           DOMAIN MODEL                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────────┐         ┌─────────────────┐                          │
│   │ PaymentIntent   │         │ PaymentMethod   │                          │
│   ├─────────────────┤         ├─────────────────┤                          │
│   │ id              │         │ id              │                          │
│   │ idempotency_key │◀────────│ customer_id     │                          │
│   │ customer_id     │         │ type            │                          │
│   │ amount          │         │ token           │                          │
│   │ currency        │         │ last_four       │                          │
│   │ status          │         │ expiry_month    │                          │
│   │ capture_method  │         │ expiry_year     │                          │
│   │ payment_method_id────────▶│ is_default      │                          │
│   │ metadata        │         │ created_at      │                          │
│   │ created_at      │         └─────────────────┘                          │
│   │ updated_at      │                                                       │
│   └────────┬────────┘                                                       │
│            │                                                                │
│            │ 1:1 (when authorized)                                          │
│            ▼                                                                │
│   ┌─────────────────┐                                                       │
│   │AuthorizationHold│                                                       │
│   ├─────────────────┤                                                       │
│   │ id              │                                                       │
│   │ intent_id       │                                                       │
│   │ amount          │                                                       │
│   │ status          │  [ACTIVE, CAPTURED, VOIDED, EXPIRED]                 │
│   │ auth_code       │                                                       │
│   │ expires_at      │                                                       │
│   │ captured_amount │                                                       │
│   │ captured_at     │                                                       │
│   │ created_at      │                                                       │
│   └─────────────────┘                                                       │
│            │                                                                │
│            │ 1:N                                                            │
│            ▼                                                                │
│   ┌─────────────────┐                                                       │
│   │ PaymentAttempt  │  (Linear State Machine - Immutable)                  │
│   ├─────────────────┤                                                       │
│   │ id              │                                                       │
│   │ intent_id       │                                                       │
│   │ attempt_number  │                                                       │
│   │ status          │  [PENDING, PROCESSING, SUCCEEDED, FAILED]            │
│   │ decline_code    │                                                       │
│   │ decline_type    │                                                       │
│   │ processor_txn_id│                                                       │
│   │ idempotency_key │                                                       │
│   │ created_at      │                                                       │
│   │ completed_at    │                                                       │
│   └─────────────────┘                                                       │
│                                                                             │
│   ┌─────────────────┐         ┌─────────────────┐                          │
│   │    Account      │         │  LedgerEntry    │                          │
│   ├─────────────────┤         ├─────────────────┤                          │
│   │ id              │◀────────│ account_id      │                          │
│   │ type            │         │ journal_entry_id│                          │
│   │ owner_id        │         │ intent_id       │                          │
│   │ ledger_balance  │         │ entry_type      │  [DEBIT, CREDIT]         │
│   │ pending_balance │         │ amount          │                          │
│   │ available_bal   │         │ balance_after   │                          │
│   │ reserved_bal    │         │ description     │                          │
│   │ currency        │         │ created_at      │                          │
│   │ status          │         └─────────────────┘                          │
│   │ daily_limit     │                                                       │
│   │ version         │  (Optimistic Locking)                                │
│   │ created_at      │                                                       │
│   │ updated_at      │                                                       │
│   └─────────────────┘                                                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 PaymentIntent State Machine

```
                              ┌─────────────────┐
                              │     CREATED     │
                              │                 │
                              │ Intent exists,  │
                              │ no method yet   │
                              └────────┬────────┘
                                       │
                                       │ attach_method
                                       ▼
                              ┌─────────────────┐
                              │ REQUIRES_AUTH   │
                              │                 │
                              │ Method attached,│
                              │ awaiting auth   │
                              └────────┬────────┘
                                       │
                           ┌───────────┼───────────┐
                           │           │           │
                           │ authorize │           │ cancel
                           ▼           │           ▼
                  ┌─────────────────┐  │  ┌─────────────────┐
                  │   AUTHORIZED    │  │  │   CANCELLED     │
                  │                 │  │  │                 │
                  │ Hold placed,    │  │  │ User or system  │
                  │ await capture   │  │  │ cancellation    │
                  └────────┬────────┘  │  └─────────────────┘
                           │           │
               ┌───────────┼───────────┤
               │           │           │
               │ capture   │ void      │ decline (soft)
               ▼           ▼           ▼
      ┌─────────────┐ ┌──────────┐ ┌─────────────────┐
      │  CAPTURED   │ │ VOIDED   │ │   RECOVERING    │
      │             │ │          │ │                 │
      │ Funds       │ │ Hold     │ │ Soft decline,   │
      │ claimed     │ │ released │ │ retry scheduled │
      └─────────────┘ └──────────┘ └────────┬────────┘
                                            │
                                ┌───────────┼───────────┐
                                │           │           │
                                │ retry     │ exhaust   │ cancel
                                │ success   │ retries   │
                                ▼           ▼           ▼
                       ┌────────────┐ ┌──────────┐ ┌──────────┐
                       │ AUTHORIZED │ │  FAILED  │ │CANCELLED │
                       └────────────┘ └──────────┘ └──────────┘
```

### 4.3 PaymentAttempt State Machine (Linear)

Each attempt is immutable and follows a strictly linear progression:

```
     PENDING ────▶ PROCESSING ────▶ SUCCEEDED
                        │
                        └────────▶ FAILED

     (No backward transitions. New retry = new attempt record)
```

### 4.4 Account Balance Calculation

| Balance Type | Calculation | Update Trigger |
|--------------|-------------|----------------|
| **ledger_balance** | SUM(ledger_entries) | Ledger entry insert |
| **pending_balance** | SUM(active holds) | Hold create/capture/void |
| **available_balance** | ledger_balance - pending_debits | Balance recalculation |
| **reserved_balance** | SUM(scheduled payment amounts) | Schedule create/complete |

### 4.5 Clearing Account Design

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      CLEARING ACCOUNT FLOW                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Customer                Payment              Merchant                      │
│   Account                 Clearing             Account                       │
│                                                                             │
│   ┌─────────┐           ┌─────────┐           ┌─────────┐                  │
│   │ $1,000  │           │   $0    │           │  $500   │                  │
│   └────┬────┘           └────┬────┘           └────┬────┘                  │
│        │                     │                     │                        │
│        │  Authorization ($100)                     │                        │
│        │ (pending balance increases)               │                        │
│        │                     │                     │                        │
│        │                     │                     │                        │
│        │  Capture                                  │                        │
│        │────────────────────▶│                     │                        │
│        │  DEBIT $100         │ CREDIT $100        │                        │
│        │                     │                     │                        │
│   ┌─────────┐           ┌─────────┐               │                        │
│   │  $900   │           │  $100   │               │                        │
│   └─────────┘           └────┬────┘               │                        │
│                              │                     │                        │
│                              │  Settlement         │                        │
│                              │────────────────────▶│                        │
│                              │  DEBIT $100         │ CREDIT $100           │
│                              │                     │                        │
│                         ┌─────────┐           ┌─────────┐                  │
│                         │   $0    │           │  $600   │                  │
│                         └─────────┘           └─────────┘                  │
│                              │                                              │
│                              │                                              │
│   Monitoring Rule: If clearing balance > $0 for > 24 hours                 │
│                    → Alert: Unresolved transaction                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.6 Decline Classification Design

The system maintains a decline code mapping table supporting 2,000+ processor codes:

| Category | Characteristics | Retry Eligible | Strategy |
|----------|-----------------|----------------|----------|
| **SOFT_FUNDS** | Temporary lack of funds | Yes | Align with paydays |
| **SOFT_GENERIC** | Vague decline, may succeed | Yes | Time variation |
| **SOFT_TEMPORARY** | Processor issues | Yes | Short backoff |
| **HARD_CARD** | Card-level permanent issue | No | Request new method |
| **HARD_ACCOUNT** | Account closed/restricted | No | Contact customer |
| **FRAUD** | Fraud indicators | No | Flag for review |
| **TEMPORARY** | System/rate limits | Activity retry | Exponential backoff |

---

## 5. Temporal Workflow Design

### 5.1 PaymentWorkflow Overview

The PaymentWorkflow orchestrates the complete payment lifecycle with support for authorization/capture separation:

**Workflow Characteristics:**
- **Execution Timeout:** 30 days (maximum dunning window)
- **Task Queue:** "payments"
- **ID Pattern:** `payment-{idempotency_key}`
- **ID Reuse Policy:** REJECT_DUPLICATE

**State Management:**
- Workflow state tracked via internal struct
- Query handlers expose state for API reads
- Signal handlers enable external event injection

**Signal Definitions:**

| Signal | Purpose | Payload |
|--------|---------|---------|
| `authorize` | Trigger authorization | AuthorizeRequest |
| `capture` | Trigger capture | CaptureRequest |
| `cancel` | Cancel intent | CancelReason |
| `update-method` | Change payment method | PaymentMethod |
| `void` | Void authorization | VoidReason |

**Query Definitions:**

| Query | Returns | Use Case |
|-------|---------|----------|
| `get-status` | PaymentState | API status endpoint |
| `get-attempts` | []AttemptRecord | Attempt history |
| `get-hold` | HoldStatus | Authorization hold details |

### 5.2 PaymentWorkflow State Machine

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      PAYMENT WORKFLOW LOGIC                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   START                                                                     │
│     │                                                                       │
│     ▼                                                                       │
│   ┌─────────────────────────────────────────────┐                          │
│   │         Initialize State                     │                          │
│   │         Register Query Handlers              │                          │
│   │         Get Signal Channels                  │                          │
│   └────────────────────┬────────────────────────┘                          │
│                        │                                                    │
│                        ▼                                                    │
│   ┌─────────────────────────────────────────────┐                          │
│   │         AWAIT SIGNAL                         │                          │
│   │                                              │                          │
│   │   Selector:                                  │                          │
│   │   - authorize signal → goto AUTHORIZATION   │                          │
│   │   - cancel signal → goto CANCELLED          │                          │
│   │   - update-method → update & continue       │                          │
│   └────────────────────┬────────────────────────┘                          │
│                        │                                                    │
│            ┌───────────┴───────────┐                                       │
│            │                       │                                        │
│            ▼                       ▼                                        │
│   ┌─────────────────┐     ┌─────────────────┐                              │
│   │  AUTHORIZATION  │     │   CANCELLED     │                              │
│   │                 │     │                 │                              │
│   │ ExecuteActivity:│     │ Record cancel   │                              │
│   │ RequestAuth     │     │ Write outbox    │                              │
│   └────────┬────────┘     │ RETURN          │                              │
│            │              └─────────────────┘                              │
│            │                                                                │
│   ┌────────┴────────┬─────────────────┐                                    │
│   │                 │                 │                                     │
│   ▼                 ▼                 ▼                                     │
│ SUCCESS          SOFT DECLINE      HARD DECLINE                            │
│   │                 │                 │                                     │
│   │                 │                 ▼                                     │
│   │                 │         ┌─────────────────┐                          │
│   │                 │         │     FAILED      │                          │
│   │                 │         │                 │                          │
│   │                 │         │ Write outbox    │                          │
│   │                 │         │ RETURN          │                          │
│   │                 │         └─────────────────┘                          │
│   │                 │                                                       │
│   │                 ▼                                                       │
│   │         ┌─────────────────┐                                            │
│   │         │   RECOVERING    │                                            │
│   │         │                 │                                            │
│   │         │ Start Recovery  │─────────────────────┐                      │
│   │         │ Child Workflow  │                     │                      │
│   │         └────────┬────────┘                     │                      │
│   │                  │                              │                      │
│   │                  ├──────── retry success ───────┤                      │
│   │                  ├──────── exhausted ──────────▶│ FAILED               │
│   │                  └──────── cancelled ──────────▶│ CANCELLED            │
│   │                                                 │                      │
│   ▼                                                                        │
│ ┌─────────────────────────────────────────────┐                           │
│ │         AUTHORIZED (Hold Created)            │                           │
│ │                                              │                           │
│ │ If capture_method == AUTOMATIC:             │                           │
│ │   → immediate capture                        │                           │
│ │ Else:                                        │                           │
│ │   → await capture signal                     │                           │
│ └────────────────────┬────────────────────────┘                           │
│                      │                                                     │
│                      ▼                                                     │
│ ┌─────────────────────────────────────────────┐                           │
│ │         AWAIT CAPTURE SIGNAL                 │                           │
│ │                                              │                           │
│ │   Selector:                                  │                           │
│ │   - capture signal → goto CAPTURE           │                           │
│ │   - void signal → goto VOIDED               │                           │
│ │   - timer (hold expiry) → goto EXPIRED      │                           │
│ └────────────────────┬────────────────────────┘                           │
│                      │                                                     │
│            ┌─────────┴─────────┐                                          │
│            ▼                   ▼                                           │
│   ┌─────────────────┐ ┌─────────────────┐                                 │
│   │    CAPTURE      │ │     VOIDED      │                                 │
│   │                 │ │                 │                                 │
│   │ ExecuteActivity:│ │ Release hold    │                                 │
│   │ ProcessCapture  │ │ Write outbox    │                                 │
│   │ RecordLedger    │ │ RETURN          │                                 │
│   │ WriteOutbox     │ └─────────────────┘                                 │
│   └────────┬────────┘                                                     │
│            │                                                               │
│            ▼                                                               │
│   ┌─────────────────┐                                                     │
│   │    CAPTURED     │                                                     │
│   │                 │                                                     │
│   │ Payment complete│                                                     │
│   │ RETURN success  │                                                     │
│   └─────────────────┘                                                     │
│                                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 RecoveryWorkflow Design

The RecoveryWorkflow handles intelligent retry scheduling as a child workflow:

**Workflow Characteristics:**
- **Parent:** PaymentWorkflow
- **Execution Timeout:** 30 days
- **ID Pattern:** `recovery-{payment_id}`

**Retry Timing Logic:**

| Decline Type | Attempt 1 | Attempt 2 | Attempt 3 | Attempt 4 | Attempt 5 |
|--------------|-----------|-----------|-----------|-----------|-----------|
| insufficient_funds | Next payday | +1 payday | +2 paydays | +3 paydays | +4 paydays |
| generic_decline | 4 hours | 12 hours | 24 hours | 48 hours | 7 days |
| do_not_honor | 6 hours | 24 hours | 48 hours | 5 days | 7 days |
| rate_limit | 1 minute | 2 minutes | 4 minutes | 8 minutes | 16 minutes |

**Payday Alignment Logic:**
- Default paydays: 1st and 15th of month
- Retry scheduled for 6 AM local time (after direct deposits clear)
- Weekend paydays shifted to following Monday
- Configurable per customer if payday data available

### 5.4 ScheduledPaymentWorkflow Design

Handles future-dated and recurring payments:

**Workflow Characteristics:**
- **Execution Timeout:** End date or 10 years
- **ID Pattern:** `scheduled-{schedule_id}`

**Recurrence Patterns:**

| Frequency | Calculation |
|-----------|-------------|
| ONCE | Execute at start_date |
| WEEKLY | start_date + (n * 7 days) |
| BIWEEKLY | start_date + (n * 14 days) |
| MONTHLY | Same day each month (adjusted for month length) |

**Pre-execution Checks:**
1. Verify source account is ACTIVE
2. Check available_balance covers amount
3. Verify no account holds/freezes
4. If checks fail: skip this occurrence, log, continue schedule

---

## 6. Activity Design

### 6.1 Activity Design Principles

All activities follow these principles:

1. **Idempotency:** Activities must be safe to retry
2. **Single Responsibility:** One external interaction per activity
3. **Non-Determinism Isolation:** All randomness, time, external calls in activities
4. **Error Classification:** Return appropriate error types for retry policy

### 6.2 Activity Definitions

| Activity | Responsibility | External System | Idempotency Strategy |
|----------|----------------|-----------------|---------------------|
| ValidateIntent | Validate intent data, check accounts | PostgreSQL | Read-only |
| RequestAuthorization | Request auth from processor | Stripe | Idempotency key |
| ProcessCapture | Capture authorized funds | Stripe | Idempotency key |
| CreateHold | Create authorization hold record | PostgreSQL | Unique constraint |
| UpdateHold | Update hold status | PostgreSQL | Version check |
| RecordLedgerEntries | Write double-entry ledger | PostgreSQL | Payment ID check |
| WriteOutbox | Write event to outbox table | PostgreSQL | Part of transaction |
| ClassifyDecline | Map processor code to category | PostgreSQL | Read-only |
| CalculateRetryTime | Determine optimal retry time | Internal | Deterministic |
| CheckAccountStatus | Verify account is active | PostgreSQL | Read-only |

### 6.3 Activity Retry Configuration

| Activity Type | Initial Interval | Backoff | Max Attempts | Non-Retryable Errors |
|---------------|------------------|---------|--------------|---------------------|
| **Database Read** | 100ms | 2.0 | 5 | None |
| **Database Write** | 100ms | 2.0 | 5 | UniqueViolation |
| **Payment Processor** | 1s | 2.0 | 3 | HardDecline, Fraud |
| **Ledger Update** | 100ms | 2.0 | 5 | None |

### 6.4 Idempotency Key Generation

For processor activities, idempotency keys are generated deterministically:

```
Key Format: {workflow_id}-{activity_type}-{attempt_number}

Examples:
- payment-abc123-authorize-1
- payment-abc123-authorize-2  (retry)
- payment-abc123-capture-1
```

This ensures:
- Same workflow + same operation = same key
- Retries use same key (safe)
- New attempts use new keys (distinct charges)

---

## 7. Database Schema

### 7.1 Core Tables

**payment_intents**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ payment_intents                                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│ id                    UUID PRIMARY KEY                                       │
│ idempotency_key       VARCHAR(100) UNIQUE NOT NULL                          │
│ customer_id           VARCHAR(50) NOT NULL                                   │
│ amount                DECIMAL(19,4) NOT NULL                                 │
│ currency              VARCHAR(3) NOT NULL                                    │
│ status                VARCHAR(20) NOT NULL                                   │
│ capture_method        VARCHAR(20) NOT NULL DEFAULT 'AUTOMATIC'              │
│ payment_method_id     UUID REFERENCES payment_methods(id)                   │
│ workflow_id           VARCHAR(100) UNIQUE                                    │
│ metadata              JSONB                                                  │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
│ updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_intents_customer ON (customer_id)                                     │
│ - idx_intents_status ON (status)                                            │
│ - idx_intents_created ON (created_at DESC)                                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

**authorization_holds**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ authorization_holds                                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│ id                    UUID PRIMARY KEY                                       │
│ payment_intent_id     UUID NOT NULL REFERENCES payment_intents(id)          │
│ amount                DECIMAL(19,4) NOT NULL                                 │
│ currency              VARCHAR(3) NOT NULL                                    │
│ status                VARCHAR(20) NOT NULL                                   │
│ processor_auth_code   VARCHAR(50)                                            │
│ processor_txn_id      VARCHAR(100)                                           │
│ expires_at            TIMESTAMPTZ NOT NULL                                   │
│ captured_amount       DECIMAL(19,4)                                          │
│ captured_at           TIMESTAMPTZ                                            │
│ voided_at             TIMESTAMPTZ                                            │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_holds_intent ON (payment_intent_id)                                   │
│ - idx_holds_status ON (status) WHERE status = 'ACTIVE'                      │
│ - idx_holds_expires ON (expires_at) WHERE status = 'ACTIVE'                 │
└─────────────────────────────────────────────────────────────────────────────┘
```

**payment_attempts**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ payment_attempts                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ id                    UUID PRIMARY KEY                                       │
│ payment_intent_id     UUID NOT NULL REFERENCES payment_intents(id)          │
│ attempt_number        INT NOT NULL                                           │
│ status                VARCHAR(20) NOT NULL                                   │
│ processor_response    VARCHAR(50)                                            │
│ decline_code          VARCHAR(50)                                            │
│ decline_type          VARCHAR(20)                                            │
│ processor_txn_id      VARCHAR(100)                                           │
│ idempotency_key       VARCHAR(150) UNIQUE NOT NULL                          │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
│ completed_at          TIMESTAMPTZ                                            │
├─────────────────────────────────────────────────────────────────────────────┤
│ CONSTRAINTS:                                                                 │
│ - UNIQUE (payment_intent_id, attempt_number)                                │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_attempts_intent ON (payment_intent_id)                                │
│ - idx_attempts_decline ON (decline_type)                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

**accounts**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ accounts                                                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│ id                    UUID PRIMARY KEY                                       │
│ type                  VARCHAR(20) NOT NULL                                   │
│ owner_id              VARCHAR(50) NOT NULL                                   │
│ ledger_balance        DECIMAL(19,4) NOT NULL DEFAULT 0                      │
│ pending_balance       DECIMAL(19,4) NOT NULL DEFAULT 0                      │
│ available_balance     DECIMAL(19,4) NOT NULL DEFAULT 0                      │
│ reserved_balance      DECIMAL(19,4) NOT NULL DEFAULT 0                      │
│ currency              VARCHAR(3) NOT NULL DEFAULT 'USD'                     │
│ status                VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'                 │
│ daily_limit           DECIMAL(19,4)                                          │
│ version               INT NOT NULL DEFAULT 1                                 │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
│ updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_accounts_owner ON (owner_id)                                          │
│ - idx_accounts_status ON (status)                                           │
│ - idx_accounts_clearing ON (type) WHERE type = 'CLEARING'                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

**ledger_entries**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ ledger_entries                                                               │
├─────────────────────────────────────────────────────────────────────────────┤
│ id                    UUID PRIMARY KEY                                       │
│ journal_entry_id      UUID NOT NULL                                          │
│ account_id            UUID NOT NULL REFERENCES accounts(id)                 │
│ payment_intent_id     UUID REFERENCES payment_intents(id)                   │
│ entry_type            VARCHAR(10) NOT NULL                                   │
│ amount                DECIMAL(19,4) NOT NULL                                 │
│ balance_after         DECIMAL(19,4) NOT NULL                                 │
│ description           TEXT                                                   │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ CONSTRAINTS:                                                                 │
│ - CHECK (amount > 0)                                                         │
│ - CHECK (entry_type IN ('DEBIT', 'CREDIT'))                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│ RULES:                                                                       │
│ - ledger_no_update: ON UPDATE DO INSTEAD NOTHING                            │
│ - ledger_no_delete: ON DELETE DO INSTEAD NOTHING                            │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_ledger_account ON (account_id, created_at DESC)                       │
│ - idx_ledger_journal ON (journal_entry_id)                                  │
│ - idx_ledger_intent ON (payment_intent_id)                                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

**outbox**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ outbox                                                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│ id                    UUID PRIMARY KEY                                       │
│ aggregate_type        VARCHAR(50) NOT NULL                                   │
│ aggregate_id          UUID NOT NULL                                          │
│ event_type            VARCHAR(50) NOT NULL                                   │
│ payload               JSONB NOT NULL                                         │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ REPLICA IDENTITY: FULL (required for CDC)                                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

**idempotency_keys**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ idempotency_keys                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ key                   VARCHAR(100) PRIMARY KEY                               │
│ request_hash          VARCHAR(64) NOT NULL                                   │
│ recovery_point        VARCHAR(50) NOT NULL                                   │
│ response_code         INT                                                    │
│ response_body         JSONB                                                  │
│ locked_at             TIMESTAMPTZ                                            │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
│ expires_at            TIMESTAMPTZ NOT NULL                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_idempotency_expires ON (expires_at)                                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

**decline_codes**
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ decline_codes                                                                │
├─────────────────────────────────────────────────────────────────────────────┤
│ processor_code        VARCHAR(50) PRIMARY KEY                                │
│ processor             VARCHAR(20) NOT NULL                                   │
│ normalized_code       VARCHAR(50) NOT NULL                                   │
│ decline_type          VARCHAR(20) NOT NULL                                   │
│ description           TEXT                                                   │
│ retry_eligible        BOOLEAN NOT NULL DEFAULT false                        │
│ suggested_action      TEXT                                                   │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_decline_type ON (decline_type)                                        │
│ - idx_decline_normalized ON (normalized_code)                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Audit Log Table

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ audit_log                                                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ id                    BIGSERIAL PRIMARY KEY                                  │
│ entity_type           VARCHAR(50) NOT NULL                                   │
│ entity_id             UUID NOT NULL                                          │
│ action                VARCHAR(50) NOT NULL                                   │
│ actor_id              VARCHAR(50)                                            │
│ actor_type            VARCHAR(20)                                            │
│ old_values            JSONB                                                  │
│ new_values            JSONB                                                  │
│ metadata              JSONB                                                  │
│ ip_address            INET                                                   │
│ created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ INDEXES:                                                                     │
│ - idx_audit_entity ON (entity_type, entity_id)                              │
│ - idx_audit_actor ON (actor_id)                                             │
│ - idx_audit_time ON (created_at DESC)                                       │
│ - idx_audit_action ON (action)                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.3 Database Triggers

**Audit Trigger for Payment Intents:**
Automatically logs all changes to payment_intents table with old/new values.

**Balance Recalculation Trigger:**
On ledger_entries insert, recalculates and updates account ledger_balance.

**Optimistic Locking Check:**
On account update, verifies version matches expected and increments.

---

## 8. Event System Design

### 8.1 Transactional Outbox Implementation

**Write Pattern:**
```
BEGIN TRANSACTION;
  -- Business operation
  UPDATE payment_intents SET status = 'CAPTURED' WHERE id = $1;
  
  -- Ledger entries
  INSERT INTO ledger_entries (...) VALUES (...);
  INSERT INTO ledger_entries (...) VALUES (...);
  
  -- Outbox event (same transaction)
  INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload)
  VALUES ('PaymentIntent', $1, 'payment.captured', $2);
COMMIT;
```

**CDC Configuration (Debezium):**
- Source: PostgreSQL logical replication
- Monitored table: outbox
- Transforms: Outbox Event Router (extracts payload as event)
- Sink: Kafka topics based on event_type

### 8.2 Event Schema (Avro)

**CloudEvents Envelope:**
```
{
  "specversion": "1.0",
  "type": "payment.captured",
  "source": "payment-service",
  "id": "<uuid>",
  "time": "<iso8601>",
  "datacontenttype": "application/json",
  "data": { ... }
}
```

**Event Types:**

| Event Type | Trigger | Key Fields |
|------------|---------|------------|
| payment.intent_created | Intent creation | intent_id, amount, currency |
| payment.authorized | Successful auth | intent_id, hold_id, auth_code |
| payment.captured | Successful capture | intent_id, amount, txn_id |
| payment.failed | Hard decline/exhaust | intent_id, decline_code, attempts |
| payment.recovering | Soft decline | intent_id, decline_code, next_retry |
| payment.cancelled | User cancellation | intent_id, reason |
| account.balance_updated | Balance change | account_id, balances |
| ledger.entries_created | Ledger write | journal_id, entries |

### 8.3 Schema Evolution Strategy

**Compatibility Mode:** BACKWARD_TRANSITIVE
- New consumers can read old messages
- All historical messages remain readable
- Breaking changes require new topic

**Safe Schema Changes:**
- Adding optional fields with defaults
- Adding new event types
- Adding aliases for field names

**Breaking Changes (Avoid):**
- Removing required fields
- Changing field types
- Renaming fields without aliases

### 8.4 Consumer Idempotency

All consumers must track processed message IDs:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ processed_events (per consumer)                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│ event_id              UUID PRIMARY KEY                                       │
│ consumer_group        VARCHAR(50) NOT NULL                                   │
│ processed_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()                    │
├─────────────────────────────────────────────────────────────────────────────┤
│ TTL: 7 days (or configurable)                                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 9. API Design

### 9.1 Endpoint Summary

| Method | Endpoint | Description | Idempotency |
|--------|----------|-------------|-------------|
| POST | /api/v1/intents | Create payment intent | Required |
| GET | /api/v1/intents/:id | Get intent status | N/A |
| PUT | /api/v1/intents/:id/method | Attach payment method | Required |
| POST | /api/v1/intents/:id/authorize | Request authorization | Required |
| POST | /api/v1/intents/:id/capture | Capture authorized funds | Required |
| POST | /api/v1/intents/:id/cancel | Cancel intent | Required |
| POST | /api/v1/intents/:id/void | Void authorization | Required |
| GET | /api/v1/intents/:id/attempts | Get attempt history | N/A |
| GET | /api/v1/intents/:id/hold | Get hold status | N/A |
| GET | /api/v1/accounts/:id | Get account balances | N/A |
| GET | /api/v1/accounts/:id/ledger | Get ledger entries | N/A |
| POST | /api/v1/webhooks/stripe | Stripe webhook receiver | N/A |
| GET | /health | Health check | N/A |
| GET | /metrics | Prometheus metrics | N/A |

### 9.2 Request/Response Schemas

**Create Intent Request:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| amount | string | Yes | Decimal amount (e.g., "100.00") |
| currency | string | Yes | ISO 4217 code |
| customer_id | string | Yes | Customer reference |
| capture_method | string | No | "automatic" or "manual" |
| payment_method_id | string | No | Pre-attach method |
| metadata | object | No | Custom key-value pairs |

**Intent Response:**
| Field | Type | Description |
|-------|------|-------------|
| id | string | Intent identifier |
| status | string | Current status |
| amount | string | Payment amount |
| currency | string | Currency code |
| capture_method | string | Capture strategy |
| payment_method | object | Attached method (if any) |
| authorization_hold | object | Hold details (if authorized) |
| latest_attempt | object | Most recent attempt |
| created_at | string | ISO 8601 timestamp |
| updated_at | string | ISO 8601 timestamp |

### 9.3 Error Response Schema

| Field | Type | Description |
|-------|------|-------------|
| error.type | string | Error category |
| error.code | string | Specific error code |
| error.message | string | Human-readable message |
| error.param | string | Related parameter (if applicable) |
| error.decline_code | string | Processor decline code (if applicable) |

### 9.4 HTTP Status Codes

| Code | Meaning | Usage |
|------|---------|-------|
| 200 | Success | Successful read or idempotent replay |
| 201 | Created | New resource created |
| 202 | Accepted | Async operation started |
| 400 | Bad Request | Validation error |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Idempotency conflict or invalid state |
| 422 | Unprocessable | Business rule violation |
| 500 | Server Error | Internal error |

---

## 10. Idempotency Implementation

### 10.1 Atomic Phases Pattern

Operations are broken into phases with recovery points stored in the idempotency record:

| Phase | Description | Recovery Point | Safe to Restart |
|-------|-------------|----------------|-----------------|
| started | Request received | Yes | From beginning |
| validated | Input validated | Yes | Skip validation |
| authorized | Processor called | Yes | Return cached result |
| hold_created | Hold recorded | Yes | Skip hold creation |
| ledger_written | Entries created | Yes | Skip ledger |
| outbox_written | Event queued | Yes | Skip outbox |
| completed | All done | Yes | Return response |

### 10.2 Idempotency Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      IDEMPOTENCY FLOW                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Request with Idempotency-Key                                              │
│         │                                                                   │
│         ▼                                                                   │
│   ┌─────────────────────────────────────────────┐                          │
│   │  SELECT * FROM idempotency_keys             │                          │
│   │  WHERE key = $1                              │                          │
│   │  FOR UPDATE NOWAIT                           │                          │
│   └────────────────────┬────────────────────────┘                          │
│                        │                                                    │
│         ┌──────────────┼──────────────┐                                    │
│         │              │              │                                     │
│         ▼              ▼              ▼                                     │
│      Not Found      Locked       Found Complete                            │
│         │              │              │                                     │
│         │              │              │                                     │
│         ▼              ▼              ▼                                     │
│   Create new      409 Conflict   Return cached                             │
│   record with     (in progress)  response                                  │
│   phase=started                                                            │
│         │                                                                   │
│         ▼                                                                   │
│   Process request phase by phase                                           │
│   Update recovery_point after each                                         │
│         │                                                                   │
│         ▼                                                                   │
│   On completion: store response                                            │
│   Release lock                                                             │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 10.3 Key Expiration

- Default expiration: 48 hours
- Background job cleans expired keys
- Re-using expired key: treated as new request

---

## 11. Concurrency and Race Conditions

### 11.1 Webhook + API Race Condition

Problem: Stripe may send both a synchronous response AND an asynchronous webhook for the same transaction. Without proper locking, concurrent processing causes issues.

Solution:
```
-- Always lock the payment record before processing
SELECT * FROM payment_intents 
WHERE id = $1 
FOR UPDATE NOWAIT;

-- If locked, webhook waits or retries later
```

### 11.2 Balance Update Race Condition

Problem: Concurrent balance reads can lead to incorrect deductions.

Solution: Optimistic locking with version column:
```
UPDATE accounts 
SET ledger_balance = ledger_balance - $1,
    version = version + 1,
    updated_at = NOW()
WHERE id = $2 AND version = $3;

-- If 0 rows affected: concurrent modification detected
-- Retry with fresh read
```

### 11.3 Hold Expiration Race

Problem: Hold may expire between check and capture.

Solution:
```
UPDATE authorization_holds
SET status = 'CAPTURED',
    captured_amount = $1,
    captured_at = NOW()
WHERE id = $2 
  AND status = 'ACTIVE'
  AND expires_at > NOW();

-- If 0 rows affected: hold expired or already captured
```

---

## 12. Observability Design

### 12.1 Metrics

| Metric | Type | Labels | Purpose |
|--------|------|--------|---------|
| payment_intents_total | Counter | status, type | Volume tracking |
| payment_authorization_duration_seconds | Histogram | processor | Latency monitoring |
| payment_capture_duration_seconds | Histogram | processor | Latency monitoring |
| payment_decline_total | Counter | decline_type, code | Decline analysis |
| ledger_entries_total | Counter | entry_type | Transaction volume |
| clearing_account_balance | Gauge | account_id | Balance monitoring |
| workflow_duration_seconds | Histogram | workflow_type | Workflow performance |
| activity_duration_seconds | Histogram | activity_type | Activity performance |

### 12.2 Alerting Rules

| Alert | Condition | Severity |
|-------|-----------|----------|
| High Decline Rate | decline_rate > 5% for 5m | Warning |
| Clearing Account Non-Zero | balance > $100 for 1h | Warning |
| Clearing Account Stuck | balance > $0 for 24h | Critical |
| Authorization Latency | p95 > 3s for 5m | Warning |
| Workflow Failures | failure_rate > 1% for 5m | Critical |
| CDC Lag | lag > 1000 messages | Warning |

### 12.3 Structured Logging

All logs include:
- `correlation_id`: Request trace ID
- `workflow_id`: Temporal workflow ID
- `payment_id`: Payment intent ID
- `customer_id`: Customer reference
- `timestamp`: ISO 8601 with microseconds

---

## 13. Project Structure

```
payment-service/
├── cmd/
│   ├── api/
│   │   └── main.go                    # HTTP API server
│   ├── worker/
│   │   └── main.go                    # Temporal worker
│   └── migrate/
│       └── main.go                    # Database migrations
├── internal/
│   ├── api/
│   │   ├── handler/
│   │   │   ├── intent.go              # PaymentIntent handlers
│   │   │   ├── account.go             # Account handlers
│   │   │   ├── webhook.go             # Webhook handlers
│   │   │   └── health.go              # Health/metrics
│   │   ├── middleware/
│   │   │   ├── idempotency.go         # Idempotency middleware
│   │   │   ├── logging.go             # Request logging
│   │   │   └── recovery.go            # Panic recovery
│   │   └── router.go
│   ├── workflow/
│   │   ├── payment.go                 # PaymentWorkflow
│   │   ├── recovery.go                # RecoveryWorkflow
│   │   ├── scheduled.go               # ScheduledPaymentWorkflow
│   │   ├── signals.go                 # Signal definitions
│   │   ├── queries.go                 # Query definitions
│   │   └── state.go                   # Workflow state types
│   ├── activity/
│   │   ├── authorization.go           # Auth activities
│   │   ├── capture.go                 # Capture activities
│   │   ├── ledger.go                  # Ledger activities
│   │   ├── outbox.go                  # Outbox activities
│   │   ├── validation.go              # Validation activities
│   │   └── decline.go                 # Decline classification
│   ├── domain/
│   │   ├── intent.go                  # PaymentIntent entity
│   │   ├── hold.go                    # AuthorizationHold entity
│   │   ├── attempt.go                 # PaymentAttempt entity
│   │   ├── account.go                 # Account entity
│   │   ├── ledger.go                  # LedgerEntry entity
│   │   ├── method.go                  # PaymentMethod entity
│   │   └── errors.go                  # Domain errors
│   ├── repository/
│   │   ├── intent.go                  # Intent repository
│   │   ├── hold.go                    # Hold repository
│   │   ├── attempt.go                 # Attempt repository
│   │   ├── account.go                 # Account repository
│   │   ├── ledger.go                  # Ledger repository
│   │   ├── outbox.go                  # Outbox repository
│   │   └── idempotency.go             # Idempotency repository
│   ├── service/
│   │   ├── payment.go                 # Payment service
│   │   └── balance.go                 # Balance service
│   └── config/
│       └── config.go                  # Configuration
├── pkg/
│   ├── stripe/
│   │   └── client.go                  # Stripe client
│   ├── temporal/
│   │   └── client.go                  # Temporal client
│   └── kafka/
│       └── producer.go                # Kafka producer (if needed)
├── migrations/
│   ├── 001_create_accounts.up.sql
│   ├── 002_create_intents.up.sql
│   ├── 003_create_holds.up.sql
│   ├── 004_create_attempts.up.sql
│   ├── 005_create_ledger.up.sql
│   ├── 006_create_outbox.up.sql
│   ├── 007_create_idempotency.up.sql
│   ├── 008_create_decline_codes.up.sql
│   └── 009_create_audit.up.sql
├── deployments/
│   ├── docker-compose.yml
│   ├── debezium/
│   │   └── connector.json
│   └── temporal/
│       └── dynamicconfig.yaml
├── scripts/
│   ├── seed_decline_codes.sql
│   └── seed_test_accounts.sql
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## 14. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)

**Objectives:**
- Project structure with domain model separation
- Infrastructure with CDC pipeline
- Core database schema

**Deliverables:**
- [ ] Go module with Intent/Method/Attempt separation
- [ ] Docker Compose: Temporal, PostgreSQL, Kafka, Debezium
- [ ] Database migrations including outbox
- [ ] Basic API server with health endpoint
- [ ] Temporal worker connecting
- [ ] Debezium connector configured
- [ ] Verify: outbox events appearing in Kafka

**Exit Criteria:** CDC pipeline operational

### Phase 2: Authorization Flow (Week 3-4)

**Objectives:**
- PaymentWorkflow with authorization
- Hold tracking
- Multi-balance accounts

**Deliverables:**
- [ ] PaymentIntent CRUD
- [ ] PaymentWorkflow basic structure
- [ ] Signal handlers (authorize, cancel)
- [ ] RequestAuthorization activity
- [ ] AuthorizationHold creation
- [ ] Multi-balance tracking
- [ ] Query handlers for status

**Exit Criteria:** Authorizations create holds; pending balance updates

### Phase 3: Capture and Ledger (Week 5-6)

**Objectives:**
- Capture flow with ledger
- Double-entry bookkeeping
- Clearing accounts

**Deliverables:**
- [ ] Capture signal handler
- [ ] ProcessCapture activity
- [ ] RecordLedgerEntries activity
- [ ] WriteOutbox activity
- [ ] Clearing account setup
- [ ] Balance monitoring queries
- [ ] Events published via CDC

**Exit Criteria:** Complete auth/capture with balanced ledger

### Phase 4: Recovery and Retry (Week 7-8)

**Objectives:**
- Decline classification
- Recovery workflow
- Intelligent retry timing

**Deliverables:**
- [ ] Decline code mapping table (50+ codes)
- [ ] ClassifyDecline activity
- [ ] RecoveryWorkflow
- [ ] PaymentAttempt per retry
- [ ] Payday alignment logic
- [ ] Signal handlers (update-method, cancel)

**Exit Criteria:** Soft declines trigger intelligent retry

### Phase 5: Production Readiness (Week 9-10)

**Objectives:**
- Real Stripe integration
- Comprehensive testing
- Documentation

**Deliverables:**
- [ ] Stripe API integration (test mode)
- [ ] Webhook handler with signature verification
- [ ] Race condition handling
- [ ] Idempotency middleware
- [ ] Workflow unit tests
- [ ] Integration tests
- [ ] API documentation
- [ ] Architecture documentation

**Exit Criteria:** All tests pass; demo ready

---

## 15. Testing Strategy

### 15.1 Workflow Unit Testing

Temporal's test framework enables workflow testing without infrastructure:

**Test Categories:**

| Category | What to Test | Mocking |
|----------|--------------|---------|
| Happy Path | Auth → Capture flow | All activities succeed |
| Soft Decline | Recovery workflow trigger | Auth returns soft decline |
| Hard Decline | Immediate failure | Auth returns hard decline |
| Signal Handling | Method update, cancel | Signals during recovery |
| Timeout | Hold expiration | Time manipulation |
| Concurrency | Simultaneous signals | Multiple signal delivery |

### 15.2 Activity Unit Testing

| Activity | Test Focus |
|----------|------------|
| RequestAuthorization | Stripe error mapping, idempotency |
| RecordLedgerEntries | Balance calculations, atomicity |
| ClassifyDecline | Code mapping accuracy |
| WriteOutbox | Event format, transaction inclusion |

### 15.3 Integration Testing

| Test Scenario | Components | Verification |
|---------------|------------|--------------|
| End-to-End Payment | API → Workflow → DB → Kafka | Event in Kafka, balance correct |
| Idempotency | Duplicate API calls | Same response, single charge |
| Race Condition | Concurrent webhook + API | No duplicate processing |
| CDC Pipeline | DB write → Kafka | Event arrives < 100ms |

---

## 16. Infrastructure Configuration

### 16.1 Docker Compose Services

| Service | Image | Ports | Purpose |
|---------|-------|-------|---------|
| temporal | temporalio/auto-setup:1.22 | 7233 | Workflow server |
| temporal-ui | temporalio/ui:2.21 | 8080 | Web UI |
| temporal-db | postgres:15-alpine | - | Temporal storage |
| app-db | postgres:15-alpine | 5433 | Application database |
| pgbouncer | edoburu/pgbouncer | 6432 | Connection pooling |
| kafka | confluentinc/cp-kafka | 9092 | Event streaming |
| zookeeper | confluentinc/cp-zookeeper | 2181 | Kafka coordination |
| schema-registry | confluentinc/cp-schema-registry | 8081 | Avro schemas |
| debezium | debezium/connect | 8083 | CDC connector |
| api | (build) | 8000 | Payment API |
| worker | (build) | - | Temporal worker |

### 16.2 PgBouncer Configuration

| Setting | Value | Rationale |
|---------|-------|-----------|
| pool_mode | transaction | Return connection after each transaction |
| max_client_conn | 1000 | Support many worker connections |
| default_pool_size | 20 | Connections per user/database |
| reserve_pool_size | 5 | Extra connections for burst |

### 16.3 Debezium Connector Configuration

| Setting | Value |
|---------|-------|
| connector.class | io.debezium.connector.postgresql.PostgresConnector |
| plugin.name | pgoutput |
| table.include.list | public.outbox |
| transforms | outbox |
| transforms.outbox.type | io.debezium.transforms.outbox.EventRouter |
| transforms.outbox.table.field.event.key | aggregate_id |
| transforms.outbox.table.field.event.type | event_type |
| transforms.outbox.table.field.event.payload | payload |

---

## Appendix A: Decline Code Reference

| Code | Type | Retry | Action |
|------|------|-------|--------|
| insufficient_funds | SOFT_FUNDS | Yes | Wait for payday |
| card_declined | SOFT_GENERIC | Yes | Retry with time variation |
| expired_card | HARD_CARD | No | Request new card |
| fraudulent | FRAUD | No | Flag for review |
| processing_error | SOFT_TEMPORARY | Yes | Short backoff |
| rate_limit | TEMPORARY | Activity retry | Exponential backoff |

(Full mapping table seeded via migration)

---

## Appendix B: Glossary

| Term | Definition |
|------|------------|
| **Authorization** | Request to place a hold on customer funds |
| **Capture** | Claim previously authorized funds |
| **CDC** | Change Data Capture - streaming database changes |
| **Clearing Account** | Account tracking in-flight transactions |
| **Idempotency Key** | Client-provided key for safe request retry |
| **Outbox Pattern** | Writing events to DB table for reliable delivery |
| **PaymentAttempt** | Single attempt to process (immutable) |
| **PaymentIntent** | Abstract payment to be made |
| **PaymentMethod** | Instrument used to pay |
| **Settlement** | Actual fund transfer between banks |

---

*Technical specification incorporating production-grade patterns from Stripe, Square, and Adyen.*