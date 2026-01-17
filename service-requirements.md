# Payment Processing Service
## Requirements and Project Document

**Version:** 1.0  
**Last Updated:** January 2026  
**Project Type:** Learning Project / Portfolio Piece

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Business Context](#2-business-context)
3. [Goals and Objectives](#3-goals-and-objectives)
4. [Functional Requirements](#4-functional-requirements)
5. [Non-Functional Requirements](#5-non-functional-requirements)
6. [System Architecture](#6-system-architecture)
7. [Data Models](#7-data-models)
8. [User Stories](#8-user-stories)
9. [API Specifications](#9-api-specifications)
10. [Project Timeline](#10-project-timeline)
11. [Success Criteria](#11-success-criteria)
12. [Risks and Mitigations](#12-risks-and-mitigations)
13. [Future Enhancements](#13-future-enhancements)

---

## 1. Project Overview

### 1.1 Executive Summary

This project involves building a Payment Processing Service that demonstrates production-grade patterns for financial transaction handling. The system combines intelligent payment retry logic (inspired by Butter Payments) with core banking transaction patterns relevant to credit unions and financial institutions.

The service handles the complete payment lifecycle: submission, validation, processing, recovery from failures, and settlement recording. It supports multiple payment types (card, ACH, internal transfers) and implements intelligent retry scheduling for failed payments.

### 1.2 Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| **Language** | Golang | Performance, strong typing, excellent concurrency support |
| **Workflow Engine** | Temporal | Durable execution, built-in retry handling, workflow state persistence |
| **Database** | PostgreSQL | ACID compliance, robust for financial data, excellent tooling |
| **Message Broker** | NATS or Kafka | Event streaming for downstream consumers |
| **Payment Processor** | Stripe (test mode) | Industry-standard API, excellent documentation |
| **API Framework** | Echo or Fiber | Lightweight, high-performance HTTP routing |
| **Containerization** | Docker Compose | Local development environment |

### 1.3 Target Audience

This project is designed as a portfolio piece demonstrating skills relevant to:

- Data Engineer positions at credit unions (e.g., Vancity)
- Backend Engineer roles at fintech companies
- Platform Engineer positions at financial institutions
- Software Engineer roles focused on payment systems

---

## 2. Business Context

### 2.1 Problem Statement

Payment failures represent a significant challenge for businesses that rely on recurring revenue:

- **Involuntary churn** (failed payments, not customer choice) accounts for approximately 50% of all subscriber churn
- Failed payments cost subscription businesses 10-35% of annual revenue
- Traditional retry systems use static schedules that ignore the nuances of different failure types
- Credit unions and banks process millions of transactions requiring robust, auditable systems

### 2.2 Industry Background

**Butter Payments Approach:**
Butter Payments has demonstrated that intelligent retry timing can recover 166% more revenue than traditional dunning systems. Their approach analyzes 128+ data points per transaction to determine optimal retry timing based on:

- Decline code classification (2,000+ unique codes)
- Card type and issuing bank patterns
- Geographic and timezone considerations
- Historical payment patterns

**Credit Union Requirements:**
Financial institutions like Vancity require:

- Complete audit trails for regulatory compliance
- Double-entry bookkeeping for financial accuracy
- Support for scheduled and recurring payments
- Real-time and batch processing capabilities
- Event-driven architecture for downstream analytics

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
   - Implement long-running workflows that span hours or days
   - Use signals for external event handling (payment method updates, cancellations)
   - Implement queries for workflow state inspection
   - Configure activity retry policies appropriately

2. **Implement Domain-Accurate Payment Processing**
   - Proper decline classification (soft, hard, fraud, temporary)
   - Idempotent payment operations preventing duplicate charges
   - Double-entry bookkeeping for ledger accuracy
   - Support for multiple payment types

3. **Build Production-Grade Patterns**
   - Event-driven architecture with published domain events
   - Comprehensive audit logging for compliance
   - API design following REST best practices
   - Proper error handling and observability

4. **Create Portfolio-Ready Deliverable**
   - Well-documented codebase
   - Working local development environment
   - Comprehensive test coverage
   - Clear README with setup instructions

### 3.2 Learning Objectives

| Objective | Skill Demonstrated |
|-----------|-------------------|
| Temporal workflow development | Distributed systems, durable execution |
| Payment domain modeling | Financial services knowledge |
| Event-driven architecture | Data pipeline design |
| API design | Backend engineering |
| Database schema design | Data modeling |
| Testing strategies | Quality engineering |

### 3.3 Out of Scope

- Production deployment infrastructure (Kubernetes, cloud providers)
- Real payment processing (test mode only)
- User authentication and authorization system
- Frontend/UI development
- Machine learning for retry optimization (rule-based approach used instead)
- Multi-currency support beyond USD
- International payment regulations

---

## 4. Functional Requirements

### 4.1 Payment Processing

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-PAY-01 | System shall accept payment requests via REST API | Must Have |
| FR-PAY-02 | System shall validate payment requests before processing | Must Have |
| FR-PAY-03 | System shall route payments to appropriate processor based on type (card, ACH, internal) | Must Have |
| FR-PAY-04 | System shall use idempotency keys to prevent duplicate payments | Must Have |
| FR-PAY-05 | System shall record transaction results in the ledger | Must Have |
| FR-PAY-06 | System shall publish events for payment state changes | Must Have |
| FR-PAY-07 | System shall support payment cancellation before completion | Should Have |
| FR-PAY-08 | System shall allow payment method updates during recovery | Should Have |

### 4.2 Decline Handling and Recovery

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-REC-01 | System shall classify declines into categories (soft, hard, fraud, temporary) | Must Have |
| FR-REC-02 | System shall automatically retry soft declines with intelligent timing | Must Have |
| FR-REC-03 | System shall not retry hard declines or fraud flags | Must Have |
| FR-REC-04 | System shall limit retry attempts to a configurable maximum (default: 6) | Must Have |
| FR-REC-05 | System shall align retry timing with common paydays for insufficient funds | Should Have |
| FR-REC-06 | System shall support immediate retry when payment method is updated | Should Have |
| FR-REC-07 | System shall track all retry attempts with timestamps and results | Must Have |

### 4.3 Scheduled Payments

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-SCH-01 | System shall support one-time future-dated payments | Should Have |
| FR-SCH-02 | System shall support recurring payments (weekly, biweekly, monthly) | Should Have |
| FR-SCH-03 | System shall verify account status before executing scheduled payments | Should Have |
| FR-SCH-04 | System shall support cancellation of scheduled payments | Should Have |
| FR-SCH-05 | System shall skip weekends and holidays for business payments | Nice to Have |

### 4.4 Account and Ledger Management

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-LED-01 | System shall maintain account balances accurately | Must Have |
| FR-LED-02 | System shall record all transactions as ledger entries | Must Have |
| FR-LED-03 | System shall use double-entry bookkeeping (every debit has a credit) | Must Have |
| FR-LED-04 | System shall prevent ledger entry modification or deletion | Must Have |
| FR-LED-05 | System shall enforce daily transaction limits when configured | Should Have |
| FR-LED-06 | System shall verify sufficient funds before debiting | Must Have |

### 4.5 Audit and Compliance

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-AUD-01 | System shall log all payment state changes | Must Have |
| FR-AUD-02 | System shall record actor information for all changes | Must Have |
| FR-AUD-03 | System shall preserve original and new values for updates | Must Have |
| FR-AUD-04 | System shall timestamp all audit entries | Must Have |
| FR-AUD-05 | System shall support audit log querying by entity, actor, or time range | Should Have |

### 4.6 API and Integration

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-API-01 | System shall expose REST endpoints for payment operations | Must Have |
| FR-API-02 | System shall return appropriate HTTP status codes | Must Have |
| FR-API-03 | System shall support querying payment status and history | Must Have |
| FR-API-04 | System shall accept Stripe webhooks for async payment updates | Should Have |
| FR-API-05 | System shall provide workflow IDs for Temporal UI inspection | Should Have |

---

## 5. Non-Functional Requirements

### 5.1 Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-PERF-01 | Payment submission API response time | < 200ms (p95) |
| NFR-PERF-02 | Payment status query response time | < 100ms (p95) |
| NFR-PERF-03 | Workflow startup latency | < 500ms |
| NFR-PERF-04 | Concurrent payment processing capacity | 100+ simultaneous workflows |

### 5.2 Reliability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-REL-01 | Payment processing must survive worker restarts | 100% |
| NFR-REL-02 | Duplicate payment prevention | 100% (via idempotency) |
| NFR-REL-03 | Ledger consistency (balanced entries) | 100% |
| NFR-REL-04 | Workflow state durability | Survives any single component failure |

### 5.3 Scalability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-SCA-01 | Horizontal worker scaling | Workers can be added without code changes |
| NFR-SCA-02 | Database connection pooling | Configurable pool size per worker |
| NFR-SCA-03 | Stateless API servers | Multiple instances behind load balancer |

### 5.4 Maintainability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-MNT-01 | Code documentation | Public functions documented with comments |
| NFR-MNT-02 | Test coverage | Minimum 70% coverage on workflow and activity code |
| NFR-MNT-03 | Configuration externalization | All environment-specific values via config |
| NFR-MNT-04 | Structured logging | JSON-formatted logs with correlation IDs |

### 5.5 Security

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-SEC-01 | Sensitive data handling | No PCI data stored; use tokenized payment methods |
| NFR-SEC-02 | Database credentials | Environment variables, not hardcoded |
| NFR-SEC-03 | Webhook verification | Stripe webhook signature validation |
| NFR-SEC-04 | API key protection | Keys not logged or exposed in responses |

### 5.6 Observability

| ID | Requirement | Description |
|----|-------------|-------------|
| NFR-OBS-01 | Structured logging | All operations logged with context |
| NFR-OBS-02 | Temporal UI integration | Workflows visible and inspectable |
| NFR-OBS-03 | Health check endpoints | API and worker health status |
| NFR-OBS-04 | Metrics exposure | Prometheus-compatible metrics (optional) |

---

## 6. System Architecture

### 6.1 Component Overview

The system follows a workflow-centric architecture where Temporal serves as the central orchestration layer:

```
┌─────────────────────────────────────────────────────────────────────┐
│                         External Clients                             │
│                    (API Consumers, Webhooks)                         │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                           API Server                                 │
│                                                                      │
│  • Receives payment requests                                         │
│  • Validates input                                                   │
│  • Starts Temporal workflows                                         │
│  • Queries workflow state                                            │
│  • Sends signals to workflows                                        │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Temporal Server                               │
│                                                                      │
│  • Manages workflow state                                            │
│  • Schedules activity execution                                      │
│  • Handles timers and retries                                        │
│  • Provides workflow visibility                                      │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                                  ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Temporal Workers                              │
│                                                                      │
│  • Execute workflow logic                                            │
│  • Execute activity implementations                                  │
│  • Can scale horizontally                                            │
│  • Stateless (state in Temporal)                                     │
└──────────┬──────────────────────┬──────────────────────┬────────────┘
           │                      │                      │
           ▼                      ▼                      ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│    PostgreSQL    │  │  Payment Gateway │  │  Message Broker  │
│                  │  │    (Stripe)      │  │   (NATS/Kafka)   │
│  • Accounts      │  │                  │  │                  │
│  • Ledger        │  │  • Card charges  │  │  • Domain events │
│  • Payments      │  │  • ACH transfers │  │  • Notifications │
│  • Audit logs    │  │  • Refunds       │  │  • Analytics     │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

### 6.2 Workflow Design

**PaymentWorkflow** - Main payment orchestrator:

1. Receives payment request with idempotency key
2. Validates payment (account status, funds, limits)
3. Routes to appropriate processor (card, ACH, internal)
4. On success: records ledger entries, publishes event
5. On soft decline: enters recovery loop with intelligent retry timing
6. On hard decline: fails immediately, publishes failure event
7. Handles signals for cancellation or payment method updates
8. Exposes queries for status and attempt history

**ScheduledPaymentWorkflow** - Recurring payment manager:

1. Receives schedule configuration (amount, frequency, dates)
2. Waits until next scheduled payment time
3. Verifies account is still active
4. Starts child PaymentWorkflow for actual processing
5. Loops for recurring schedules until end date or cancellation
6. Handles cancellation signals

### 6.3 Activity Design

Activities encapsulate all external interactions and side effects:

| Activity | Responsibility | External System |
|----------|---------------|-----------------|
| ValidatePayment | Check account status, funds, limits | PostgreSQL |
| ProcessCardPayment | Execute card charge | Stripe API |
| ProcessACHPayment | Execute bank transfer | Stripe API (ACH) |
| ProcessInternalTransfer | Move funds between accounts | PostgreSQL |
| UpdateLedger | Record debit/credit entries | PostgreSQL |
| PublishEvent | Emit domain event | NATS/Kafka |
| SendNotification | Email/SMS alerts | Email/SMS provider |
| CheckAccountStatus | Verify account is active | PostgreSQL |

### 6.4 Data Flow

**Successful Payment Flow:**
```
Client → API → Start Workflow → Validate → Process Payment → Update Ledger → Publish Event → Complete
```

**Failed Payment with Recovery:**
```
Client → API → Start Workflow → Validate → Process Payment → Decline (Soft)
    → Classify Decline → Calculate Retry Delay → Sleep → [Signal Check]
    → Process Payment → Success → Update Ledger → Publish Event → Complete
```

**Payment Method Update During Recovery:**
```
[Workflow in Recovery Sleep]
    → Client sends UpdatePaymentMethod signal
    → Workflow receives signal, updates payment method
    → Workflow wakes immediately, retries with new method
```

---

## 7. Data Models

### 7.1 Payment Entity

The Payment represents a single payment request and its lifecycle:

| Field | Type | Description |
|-------|------|-------------|
| id | String | Unique payment identifier |
| idempotency_key | String | Client-provided key preventing duplicates |
| type | Enum | CARD, ACH, INTERNAL |
| status | Enum | PENDING, SCHEDULED, PROCESSING, RECOVERING, SUCCEEDED, FAILED, CANCELLED |
| source_account_id | String | Account being debited |
| dest_account_id | String | Account being credited (optional for card) |
| amount | Decimal | Payment amount |
| currency | String | ISO 4217 currency code |
| payment_method | Object | Card/bank account details (tokenized) |
| scheduled_for | Timestamp | Future execution time (optional) |
| processed_at | Timestamp | Actual processing time |
| attempt_count | Integer | Number of processing attempts |
| next_retry_at | Timestamp | Next scheduled retry (if recovering) |
| last_decline_code | String | Most recent decline reason |
| created_at | Timestamp | Record creation time |
| updated_at | Timestamp | Last modification time |
| metadata | Map | Custom key-value data |

### 7.2 Payment Status State Machine

```
                    ┌────────────┐
                    │  PENDING   │
                    └─────┬──────┘
                          │
              ┌───────────┼───────────┐
              │           │           │
              ▼           ▼           ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │SCHEDULED │ │PROCESSING│ │CANCELLED │
        └────┬─────┘ └────┬─────┘ └──────────┘
             │            │
             │            ├────────────────┐
             │            │                │
             ▼            ▼                ▼
        ┌──────────┐ ┌──────────┐    ┌──────────┐
        │ PENDING  │ │RECOVERING│    │ FAILED   │
        └──────────┘ └────┬─────┘    └──────────┘
                          │
                          │ (retry)
                          ▼
                    ┌──────────┐
                    │PROCESSING│
                    └────┬─────┘
                         │
              ┌──────────┴──────────┐
              ▼                     ▼
        ┌──────────┐          ┌──────────┐
        │SUCCEEDED │          │  FAILED  │
        └──────────┘          └──────────┘
```

### 7.3 Account Entity

| Field | Type | Description |
|-------|------|-------------|
| id | String | Unique account identifier |
| type | Enum | CHECKING, SAVINGS, LOAN |
| owner_id | String | Customer identifier |
| balance | Decimal | Current balance |
| available_balance | Decimal | Available for withdrawal |
| currency | String | Account currency |
| status | Enum | ACTIVE, FROZEN, CLOSED |
| daily_limit | Decimal | Maximum daily outflow |
| created_at | Timestamp | Account creation time |
| updated_at | Timestamp | Last modification time |

### 7.4 Ledger Entry Entity

| Field | Type | Description |
|-------|------|-------------|
| id | String | Unique entry identifier |
| account_id | String | Affected account |
| payment_id | String | Associated payment |
| type | Enum | DEBIT, CREDIT |
| amount | Decimal | Entry amount |
| balance_after | Decimal | Account balance after entry |
| description | String | Human-readable description |
| created_at | Timestamp | Entry creation time (immutable) |

### 7.5 Decline Classification

| Category | Description | Action | Examples |
|----------|-------------|--------|----------|
| **Soft Decline** | Temporary issue, may resolve | Retry with intelligent timing | insufficient_funds, generic_decline, do_not_honor |
| **Hard Decline** | Permanent issue | Fail immediately, require new payment method | stolen_card, expired_card, invalid_account |
| **Fraud** | Security concern | Fail immediately, flag for review | fraudulent, pickup_card |
| **Temporary** | Infrastructure issue | Activity-level retry (seconds) | rate_limit, timeout, service_unavailable |

### 7.6 Scheduled Payment Entity

| Field | Type | Description |
|-------|------|-------------|
| id | String | Unique schedule identifier |
| workflow_id | String | Associated Temporal workflow |
| source_account_id | String | Account to debit |
| dest_account_id | String | Account to credit |
| amount | Decimal | Payment amount |
| currency | String | Payment currency |
| payment_type | Enum | CARD, ACH, INTERNAL |
| frequency | Enum | ONCE, WEEKLY, BIWEEKLY, MONTHLY |
| start_date | Date | First payment date |
| end_date | Date | Last payment date (optional) |
| day_of_month | Integer | For monthly: 1-28 |
| day_of_week | Integer | For weekly: 0-6 |
| status | Enum | ACTIVE, PAUSED, COMPLETED, CANCELLED |
| last_payment_at | Timestamp | Most recent payment time |
| next_payment_at | Timestamp | Next scheduled payment |

---

## 8. User Stories

### 8.1 Payment Submission

**US-01: Submit a one-time payment**
> As an API consumer, I want to submit a payment request so that funds are transferred from one account to another.

Acceptance Criteria:
- Payment request includes idempotency key, source account, amount, and payment method
- System returns 202 Accepted with payment ID and workflow ID
- Payment progresses through validation and processing
- Successful payment results in ledger entries and event publication

**US-02: Prevent duplicate payments**
> As an API consumer, I want duplicate requests with the same idempotency key to be handled safely so that customers are not charged twice.

Acceptance Criteria:
- Submitting the same idempotency key twice returns the existing payment status
- Only one workflow is created per idempotency key
- Balance is only affected once

### 8.2 Payment Status

**US-03: Check payment status**
> As an API consumer, I want to query the status of a payment so that I can display progress to the customer.

Acceptance Criteria:
- GET request with payment ID returns current status
- Response includes attempt count, last decline code (if any), and next retry time (if recovering)
- Response includes workflow ID for debugging

**US-04: View payment attempts**
> As an API consumer, I want to see the history of payment attempts so that I can understand why a payment is failing.

Acceptance Criteria:
- GET request returns list of all attempts
- Each attempt includes timestamp, success/failure, and decline code
- Attempts are ordered chronologically

### 8.3 Payment Recovery

**US-05: Automatic retry on soft decline**
> As a business, I want failed payments to be automatically retried at intelligent times so that I recover more revenue without manual intervention.

Acceptance Criteria:
- Soft declines (insufficient funds, generic decline) trigger automatic retry
- Retry timing considers decline type (align with paydays for insufficient funds)
- Maximum of 6 retry attempts before giving up
- Hard declines and fraud flags do not trigger retry

**US-06: Update payment method during recovery**
> As an API consumer, I want to update the payment method for a recovering payment so that the customer can provide a working card.

Acceptance Criteria:
- PUT request with new payment method token sends signal to workflow
- Workflow wakes immediately and retries with new method
- Payment method update is recorded in attempt history

**US-07: Cancel a recovering payment**
> As an API consumer, I want to cancel a payment that is in recovery so that customers can choose to stop retry attempts.

Acceptance Criteria:
- POST request to cancel endpoint sends cancellation signal
- Workflow transitions to CANCELLED status
- No further retry attempts are made
- Cancellation event is published

### 8.4 Scheduled Payments

**US-08: Schedule a future payment**
> As an API consumer, I want to schedule a payment for a future date so that customers can set up bill pays.

Acceptance Criteria:
- Payment request with scheduled_for date creates scheduled payment
- Workflow waits until scheduled time
- Payment is processed at scheduled time (if account is still active)
- Schedule can be cancelled before execution

**US-09: Set up recurring payments**
> As an API consumer, I want to create a recurring payment schedule so that loan payments or subscriptions are automatically processed.

Acceptance Criteria:
- Schedule includes frequency (weekly, biweekly, monthly) and start/end dates
- Individual payments are created for each occurrence
- Schedule can be cancelled at any time
- Skipped payments (inactive account) do not cancel the schedule

### 8.5 Account Management

**US-10: Check account balance**
> As an API consumer, I want to query an account balance so that I can display it to customers.

Acceptance Criteria:
- GET request returns current balance and available balance
- Response includes account status

**US-11: View account ledger**
> As an API consumer, I want to view ledger entries for an account so that customers can see their transaction history.

Acceptance Criteria:
- GET request returns list of ledger entries
- Entries include payment reference, type (debit/credit), amount, and running balance
- Entries are ordered by date (newest first)
- Pagination is supported

---

## 9. API Specifications

### 9.1 Endpoints Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/v1/payments | Submit a new payment |
| GET | /api/v1/payments/:id | Get payment status |
| GET | /api/v1/payments/:id/attempts | Get payment attempt history |
| POST | /api/v1/payments/:id/cancel | Cancel a payment |
| PUT | /api/v1/payments/:id/method | Update payment method |
| POST | /api/v1/scheduled-payments | Create scheduled payment |
| GET | /api/v1/scheduled-payments/:id | Get schedule status |
| DELETE | /api/v1/scheduled-payments/:id | Cancel schedule |
| GET | /api/v1/accounts/:id/balance | Get account balance |
| GET | /api/v1/accounts/:id/ledger | Get account ledger |
| POST | /api/v1/webhooks/stripe | Receive Stripe webhooks |
| GET | /health | Health check |

### 9.2 Request/Response Formats

**Create Payment Request:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| idempotency_key | String | Yes | Client-provided unique key |
| type | String | Yes | CARD, ACH, or INTERNAL |
| source_account_id | String | Yes | Account to debit |
| dest_account_id | String | Conditional | Account to credit (required for INTERNAL) |
| amount | String | Yes | Decimal amount (e.g., "150.00") |
| currency | String | Yes | ISO 4217 code (e.g., "USD") |
| payment_method.type | String | Conditional | card or bank_account |
| payment_method.token | String | Conditional | Tokenized payment method ID |
| scheduled_for | String | No | ISO 8601 timestamp for future payment |
| metadata | Object | No | Custom key-value pairs |

**Create Payment Response (202 Accepted):**
| Field | Type | Description |
|-------|------|-------------|
| id | String | Payment identifier (same as idempotency_key) |
| status | String | Current status (typically PROCESSING) |
| workflow_id | String | Temporal workflow ID for debugging |
| created_at | String | ISO 8601 timestamp |
| links.self | String | URL to payment status |
| links.attempts | String | URL to attempt history |

**Payment Status Response:**
| Field | Type | Description |
|-------|------|-------------|
| id | String | Payment identifier |
| status | String | Current status |
| type | String | Payment type |
| amount | String | Payment amount |
| currency | String | Currency code |
| source_account_id | String | Source account |
| dest_account_id | String | Destination account |
| attempt_count | Integer | Number of attempts |
| last_decline_code | String | Most recent decline reason |
| next_retry_at | String | Next retry time (if recovering) |
| transaction_id | String | Processor transaction ID (if succeeded) |
| created_at | String | Creation timestamp |
| updated_at | String | Last update timestamp |

### 9.3 Error Responses

| Status Code | Meaning | When Used |
|-------------|---------|-----------|
| 400 Bad Request | Invalid input | Missing required fields, invalid format |
| 404 Not Found | Resource not found | Payment or account does not exist |
| 409 Conflict | State conflict | Attempting invalid state transition |
| 422 Unprocessable | Business rule violation | Insufficient funds at submission |
| 500 Internal Error | System error | Database or external service failure |

---

## 10. Project Timeline

### Phase 1: Foundation (Week 1-2)

**Objectives:**
- Establish project structure and development environment
- Set up core infrastructure components
- Implement basic database schema

**Deliverables:**
- [ ] Go module initialized with dependencies
- [ ] Docker Compose configuration for Temporal, PostgreSQL, NATS
- [ ] Database migrations for accounts, payments, ledger tables
- [ ] Basic API server with health check endpoint
- [ ] Temporal worker connecting to server
- [ ] README with setup instructions

**Milestone:** `docker-compose up` starts all services; health check returns 200

### Phase 2: Core Workflows (Week 3-4)

**Objectives:**
- Implement main payment workflow (happy path)
- Build essential activities
- Enable basic payment processing

**Deliverables:**
- [ ] PaymentWorkflow with validation and processing steps
- [ ] ValidatePayment activity
- [ ] ProcessCardPayment activity (mock Stripe initially)
- [ ] ProcessInternalTransfer activity
- [ ] UpdateLedger activity with double-entry bookkeeping
- [ ] Query handlers for workflow state
- [ ] API endpoints for payment submission and status

**Milestone:** End-to-end payment processing works; ledger entries created correctly

### Phase 3: Recovery Logic (Week 5-6)

**Objectives:**
- Implement decline classification
- Build intelligent retry scheduling
- Add signal handling for external events

**Deliverables:**
- [ ] Decline classifier (soft/hard/fraud/temporary)
- [ ] Retry loop with configurable attempts
- [ ] calculateRetryDelay function with payday alignment
- [ ] Signal handler for payment method updates
- [ ] Signal handler for cancellation
- [ ] PublishEvent activity
- [ ] API endpoints for cancel and update method

**Milestone:** Soft declines trigger intelligent retries; signals interrupt wait periods

### Phase 4: Scheduled Payments (Week 7-8)

**Objectives:**
- Implement scheduled payment workflow
- Support recurring payment schedules
- Build schedule management APIs

**Deliverables:**
- [ ] ScheduledPaymentWorkflow with timer-based waiting
- [ ] Support for ONCE, WEEKLY, BIWEEKLY, MONTHLY frequencies
- [ ] Child workflow pattern for individual payments
- [ ] Account status check before scheduled execution
- [ ] API endpoints for schedule CRUD operations
- [ ] Schedule cancellation via signals

**Milestone:** Recurring payments execute on schedule; schedules can be managed via API

### Phase 5: Production Readiness (Week 9-10)

**Objectives:**
- Integrate real Stripe test mode
- Achieve comprehensive test coverage
- Complete documentation

**Deliverables:**
- [ ] Real Stripe API integration (test mode)
- [ ] Webhook handler for Stripe events
- [ ] Unit tests for workflows (using Temporal test framework)
- [ ] Unit tests for activities
- [ ] Integration tests for critical paths
- [ ] API documentation
- [ ] Architecture documentation
- [ ] Demo script / walkthrough

**Milestone:** All tests pass; documentation complete; ready for portfolio review

---

## 11. Success Criteria

### 11.1 Functional Completeness

| Criterion | Measurement |
|-----------|-------------|
| Payment processing works end-to-end | Card, ACH, and internal payments complete successfully |
| Decline handling is correct | Soft declines retry; hard declines fail immediately |
| Retry timing is intelligent | Insufficient funds aligns with paydays |
| Signals work correctly | Payment method update triggers immediate retry |
| Ledger is always balanced | Every debit has matching credit; no orphaned entries |
| Scheduled payments execute | Payments process at configured times |

### 11.2 Code Quality

| Criterion | Target |
|-----------|--------|
| Test coverage (workflow code) | ≥ 70% |
| Test coverage (activity code) | ≥ 70% |
| Linting passes | Zero warnings with golangci-lint |
| No hardcoded credentials | All secrets via environment variables |
| Consistent code style | gofmt applied to all files |

### 11.3 Documentation Quality

| Criterion | Measurement |
|-----------|-------------|
| README completeness | Setup instructions work on fresh machine |
| API documentation | All endpoints documented with examples |
| Architecture clarity | Diagrams explain system components |
| Code comments | Public functions have doc comments |

### 11.4 Portfolio Impact

| Criterion | Measurement |
|-----------|-------------|
| Demonstrates Temporal expertise | Workflows use signals, queries, timers, child workflows |
| Shows domain knowledge | Payment processing patterns are industry-accurate |
| Exhibits production thinking | Idempotency, audit trails, error handling present |
| Presents well | Clean code, good documentation, working demo |

---

## 12. Risks and Mitigations

### 12.1 Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Temporal learning curve | Medium | Medium | Start with simple workflows; use official tutorials |
| Stripe API complexity | Low | Low | Use test mode; leverage excellent documentation |
| Database transaction issues | Medium | High | Use serializable isolation; thorough testing |
| Workflow determinism violations | Medium | Medium | Review Temporal best practices; use workflow test framework |

### 12.2 Schedule Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Scope creep | Medium | High | Strict adherence to "Out of Scope" list |
| Infrastructure setup delays | Low | Medium | Use official Docker images; follow quickstart guides |
| Testing takes longer than expected | Medium | Medium | Write tests alongside features, not after |

### 12.3 Quality Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Insufficient test coverage | Medium | Medium | Track coverage metrics; test before merge |
| Documentation falls behind | High | Low | Document as you build; final review phase |
| Edge cases missed | Medium | Medium | Use Temporal test framework's time manipulation |

---

## 13. Future Enhancements

The following items are explicitly out of scope for the initial project but represent natural extensions:

### 13.1 Near-Term Enhancements

- **Multi-currency support:** Handle CAD, EUR, GBP with exchange rate lookups
- **Partial payments:** Allow payments to be split across multiple attempts
- **Payment holds:** Pre-authorize amounts before capture
- **Refund workflow:** Process refunds with ledger reversals

### 13.2 Medium-Term Enhancements

- **ML-based retry optimization:** Replace rule-based timing with trained model
- **Fraud scoring integration:** Add risk assessment before processing
- **Customer notifications:** Email/SMS for payment status changes
- **Dashboard UI:** React frontend for payment monitoring

### 13.3 Long-Term Enhancements

- **Multi-tenant support:** Serve multiple merchants with isolation
- **Real-time analytics:** Stream processing for payment metrics
- **Compliance reporting:** Automated regulatory report generation
- **International payments:** Cross-border transfers with compliance checks

---

## Appendix A: Glossary

| Term | Definition |
|------|------------|
| **ACH** | Automated Clearing House - US bank transfer network |
| **Activity** | Temporal concept: a function that performs side effects (API calls, DB writes) |
| **Decline Code** | Reason code returned by payment processor for failed transactions |
| **Double-Entry Bookkeeping** | Accounting method where every transaction affects two accounts |
| **Dunning** | Process of attempting to collect failed payments |
| **Hard Decline** | Permanent payment failure requiring new payment method |
| **Idempotency** | Property ensuring repeated requests have same effect as single request |
| **Involuntary Churn** | Customer loss due to payment failure, not intentional cancellation |
| **Ledger** | Record of all financial transactions |
| **Signal** | Temporal concept: external event sent to running workflow |
| **Soft Decline** | Temporary payment failure that may succeed on retry |
| **Temporal** | Workflow orchestration platform for durable execution |
| **Workflow** | Temporal concept: orchestration function managing long-running process |

---

## Appendix B: References

- [Temporal Go SDK Documentation](https://docs.temporal.io/dev-guide/go)
- [Stripe API Reference](https://stripe.com/docs/api)
- [Butter Payments - How It Works](https://www.butterpayments.com/)
- [Vancity Data Engineer Job Posting](https://www.vancity.com/careers)
- [Double-Entry Bookkeeping](https://en.wikipedia.org/wiki/Double-entry_bookkeeping)
- [Payment Card Industry Data Security Standard](https://www.pcisecuritystandards.org/)

---

*Document prepared for portfolio project demonstrating payment processing domain expertise and Temporal workflow development skills.*