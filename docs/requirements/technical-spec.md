# Payment Processing Service
## Technical Specification

**Version:** 1.2  
**A Learning Project with Golang and Temporal**

Designed for Credit Union / Banking Domain Experience  
Incorporating Production Patterns from Stripe, Square, and Adyen

---

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | Dec 2025 | Initial technical specification |
| 1.1 | Jan 2026 | Added production-grade patterns (Intent/Method separation, transactional outbox, clearing accounts, linear state machines) |
| 1.2 | Jan 2026 | Added multi-provider adapter architecture, canonical event model, provider-specific activities |

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architectural Principles](#2-architectural-principles)
3. [System Architecture](#3-system-architecture)
4. [Adapter Layer Design](#4-adapter-layer-design)
5. [Domain Model Design](#5-domain-model-design)
6. [Temporal Workflow Design](#6-temporal-workflow-design)
7. [Activity Design](#7-activity-design)
8. [Database Schema](#8-database-schema)
9. [Event System Design](#9-event-system-design)
10. [API Design](#10-api-design)
11. [Idempotency Implementation](#11-idempotency-implementation)
12. [Concurrency and Race Conditions](#12-concurrency-and-race-conditions)
13. [Observability Design](#13-observability-design)
14. [Project Structure](#14-project-structure)
15. [Implementation Roadmap](#15-implementation-roadmap)
16. [Testing Strategy](#16-testing-strategy)
17. [Infrastructure Configuration](#17-infrastructure-configuration)

---

## 1. Executive Summary

### 1.1 Project Overview

This specification describes a Payment Processing Service demonstrating production-grade patterns derived from studying Stripe, Square/Block, Adyen, and major financial institutions. The service implements durable workflow orchestration using Temporal, enabling payment operations that may span hours or days.

**Core Architectural Truth:** Payments are promises about money movement, not money movement itself. Every design decision flows from understanding the distinction between authorization (the promise) and settlement (the actual transfer).

**Multi-Provider Design Truth:** Provider-specific logic belongs at the edges. The core system speaks a canonical language, and adapters translate at the boundaries.

**Key Insight:** Exactly-once payment processing is achieved through at-least-once delivery combined with idempotent consumers. True exactly-once delivery is theoretically impossible in distributed systems.

### 1.2 Learning Objectives

- Master Temporal workflow patterns including signals, queries, and durable timers
- Implement the separation of Intent, Method, and Order
- Build multi-provider adapter layer with canonical event normalization
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
| **Payment Processors** | Stripe, Adyen, PayPal (test mode) | Demonstrate multi-provider patterns |

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

#### Principle 7: Normalize at the Edge

Provider-specific logic should exist only at system boundaries. The core system should never see provider-specific types, event names, or decline codes.

**Application:** Adapters transform provider webhooks to canonical events before they enter the workflow engine. Provider-specific API calls are isolated in activities.

### 2.2 Anti-Patterns to Avoid

| Anti-Pattern | Risk | Mitigation |
|--------------|------|------------|
| **Card-first abstractions** | Other payment methods don't fit | Separate Intent from Method |
| **Provider-specific core logic** | Tight coupling, hard to add providers | Canonical model, adapter pattern |
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
│                       PAYMENT PROCESSING SERVICE                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                      ADAPTER LAYER (Edge)                              │  │
│  │                  "Normalize at the Boundary"                           │  │
│  │                                                                        │  │
│  │   Stripe         Adyen          PayPal         [Future]               │  │
│  │   Webhook        Webhook        Webhook        Providers              │  │
│  │      │              │              │                                   │  │
│  │      ▼              ▼              ▼                                   │  │
│  │  ┌────────┐    ┌────────┐    ┌────────┐                               │  │
│  │  │ Stripe │    │ Adyen  │    │ PayPal │                               │  │
│  │  │Adapter │    │Adapter │    │Adapter │                               │  │
│  │  └───┬────┘    └───┬────┘    └───┬────┘                               │  │
│  │      │             │             │                                     │  │
│  │      └─────────────┼─────────────┘                                     │  │
│  │                    │                                                   │  │
│  │                    ▼                                                   │  │
│  │            ┌──────────────┐                                            │  │
│  │            │  Canonical   │                                            │  │
│  │            │   Event      │                                            │  │
│  │            └──────────────┘                                            │  │
│  └────────────────────┼──────────────────────────────────────────────────┘  │
│                       │                                                     │
│  ┌────────────────────┼──────────────────────────────────────────────────┐  │
│  │                    ▼                CORE LAYER                         │  │
│  │                                (Provider-Agnostic)                     │  │
│  │                                                                        │  │
│  │  ┌─────────────────────┐     ┌─────────────────────────────────────┐  │  │
│  │  │     API Server      │     │         Temporal Workflows          │  │  │
│  │  │     (Stateless)     │────▶│                                     │  │  │
│  │  │                     │     │  PaymentWorkflow                    │  │  │
│  │  │  POST /intents      │     │    ├─ ValidateIntent                │  │  │
│  │  │  POST /intents/:id/ │     │    ├─ SelectProviderActivity        │  │  │
│  │  │       authorize     │     │    ├─ RequestAuthorization          │  │  │
│  │  │  POST /intents/:id/ │     │    ├─ WaitForCapture                │  │  │
│  │  │       capture       │     │    ├─ ProcessCapture                │  │  │
│  │  │                     │     │    ├─ RecordLedgerEntries           │  │  │
│  │  └─────────────────────┘     │    └─ WriteToOutbox                 │  │  │
│  │                              │                                     │  │  │
│  │  ┌─────────────────────┐     │  RecoveryWorkflow                   │  │  │
│  │  │   Signal Handlers   │────▶│    ├─ ClassifyDecline (canonical)   │  │  │
│  │  │                     │     │    ├─ CreateAttemptRecord           │  │  │
│  │  │  - ProviderEvent    │     │    ├─ CalculateRetryTime            │  │  │
│  │  │  - UpdateMethod     │     │    └─ RetryOrEscalate               │  │  │
│  │  │  - CancelIntent     │     │                                     │  │  │
│  │  └─────────────────────┘     └─────────────────────────────────────┘  │  │
│  │                                                                        │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────────┐  │
│  │                        ACTIVITY LAYER                                   │  │
│  │                                                                         │  │
│  │  ┌─────────────────────────────┐  ┌─────────────────────────────────┐  │  │
│  │  │  Provider-Specific          │  │  Provider-Agnostic              │  │  │
│  │  │  (Outbound API Calls)       │  │  (Internal Operations)          │  │  │
│  │  │                             │  │                                 │  │  │
│  │  │  StripeAuthActivity         │  │  ValidateIntentActivity         │  │  │
│  │  │  StripeCaptureActivity      │  │  RecordLedgerActivity           │  │  │
│  │  │  AdyenAuthActivity          │  │  WriteOutboxActivity            │  │  │
│  │  │  AdyenCaptureActivity       │  │  ClassifyDeclineActivity        │  │  │
│  │  │  PayPalAuthActivity         │  │  CalculateRetryActivity         │  │  │
│  │  │  PayPalCaptureActivity      │  │  CheckAccountActivity           │  │  │
│  │  └─────────────────────────────┘  └─────────────────────────────────┘  │  │
│  │                                                                         │  │
│  └─────────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────────┐  │
│  │                          DATA LAYER                                      │  │
│  │                                                                          │  │
│  │   PostgreSQL                             Kafka                           │  │
│  │   ┌─────────────────────────────┐       ┌─────────────────────────────┐ │  │
│  │   │ payment_intents             │       │ payments.authorized         │ │  │
│  │   │ authorization_holds         │       │ payments.captured           │ │  │
│  │   │ payment_attempts            │       │ payments.failed             │ │  │
│  │   │ ledger_entries              │       │                             │ │  │
│  │   │ accounts                    │       │ (Canonical events only -    │ │  │
│  │   │ clearing_accounts           │       │  no provider specifics)     │ │  │
│  │   │ decline_code_mappings       │       │                             │ │  │
│  │   │ outbox ─────────────────────┼──────▶│ via Debezium CDC            │ │  │
│  │   │ audit_log                   │       │                             │ │  │
│  │   └─────────────────────────────┘       └─────────────────────────────┘ │  │
│  │                                                                          │  │
│  └──────────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Layer Responsibilities

| Layer | Responsibility | Provider-Aware? |
|-------|----------------|-----------------|
| **Adapter Layer** | Receive webhooks, verify signatures, normalize to canonical | Yes (by design) |
| **API Layer** | REST endpoints, request validation, workflow orchestration | No |
| **Workflow Layer** | Business logic, state management, signal handling | No |
| **Activity Layer (PSP)** | Make outbound calls to payment providers | Yes (isolated) |
| **Activity Layer (Internal)** | Ledger, outbox, decline classification | No |
| **Data Layer** | Persistence, event publishing | No |

### 3.3 Data Flow Patterns

**Inbound (Provider → System):**
```
Provider Webhook → Adapter → Canonical Event → Signal Workflow → Process
```

**Outbound (System → Provider):**
```
Workflow → Provider-Specific Activity → Provider API → Response → Continue Workflow
```

**Events (System → Consumers):**
```
DB Transaction (state + outbox) → CDC → Kafka → Consumers (canonical events only)
```

---

## 4. Adapter Layer Design

### 4.1 Adapter Responsibilities

Each provider adapter is responsible for:

1. **Signature Verification** - Provider-specific authentication
2. **Event Parsing** - Unmarshal provider's JSON/XML format
3. **Event Mapping** - Translate event type to canonical type
4. **Decline Code Mapping** - Translate decline codes to canonical codes
5. **Payload Preservation** - Store raw payload for debugging
6. **Workflow Signaling** - Send canonical event to appropriate workflow
7. **Response Formatting** - Return provider-expected acknowledgment

### 4.2 Provider Signature Verification

| Provider | Method | Implementation |
|----------|--------|----------------|
| **Stripe** | HMAC-SHA256 with timestamp | Verify `Stripe-Signature` header; reject if timestamp > 5 min old |
| **Adyen** | HMAC-SHA256 | Verify using shared HMAC key from Adyen dashboard |
| **PayPal** | Webhook ID verification | Call PayPal API to verify webhook authenticity |

### 4.3 Canonical Event Model

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        CANONICAL PAYMENT EVENT                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Type CanonicalPaymentEvent struct {                                        │
│      // Event identification                                                │
│      EventID          string                  // Unique event ID            │
│      EventType        CanonicalEventType      // AUTHORIZATION_SUCCEEDED    │
│                                                                             │
│      // Payment identification                                              │
│      PaymentID        string                  // Our internal payment ID    │
│      ProviderPaymentID string                 // Provider's transaction ID  │
│      Provider         Provider                // STRIPE, ADYEN, PAYPAL      │
│                                                                             │
│      // Transaction details                                                 │
│      Amount           int64                   // Amount in smallest unit    │
│      Currency         string                  // ISO 4217 (USD, EUR)        │
│                                                                             │
│      // Status                                                              │
│      Status           CanonicalStatus         // SUCCEEDED, FAILED, PENDING │
│      FailureCode      *CanonicalDeclineCode   // Normalized decline code    │
│      FailureMessage   *string                 // Human-readable message     │
│                                                                             │
│      // Authorization details (if applicable)                               │
│      AuthorizationCode *string                // Auth code from issuer      │
│      NetworkTxnID      *string                // Card network reference     │
│                                                                             │
│      // Metadata                                                            │
│      RawPayload       json.RawMessage         // Original webhook body      │
│      ProviderTimestamp time.Time              // When provider recorded it  │
│      ReceivedAt       time.Time               // When we received it        │
│  }                                                                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.4 Canonical Event Types

```go
type CanonicalEventType string

const (
    // Authorization
    EventAuthorizationSucceeded CanonicalEventType = "AUTHORIZATION_SUCCEEDED"
    EventAuthorizationFailed    CanonicalEventType = "AUTHORIZATION_FAILED"
    
    // Capture
    EventCaptureSucceeded       CanonicalEventType = "CAPTURE_SUCCEEDED"
    EventCaptureFailed          CanonicalEventType = "CAPTURE_FAILED"
    
    // Void/Cancel
    EventVoidSucceeded          CanonicalEventType = "VOID_SUCCEEDED"
    EventVoidFailed             CanonicalEventType = "VOID_FAILED"
    
    // Refund
    EventRefundSucceeded        CanonicalEventType = "REFUND_SUCCEEDED"
    EventRefundFailed           CanonicalEventType = "REFUND_FAILED"
    
    // Disputes
    EventDisputeOpened          CanonicalEventType = "DISPUTE_OPENED"
    EventDisputeWon             CanonicalEventType = "DISPUTE_WON"
    EventDisputeLost            CanonicalEventType = "DISPUTE_LOST"
)
```

### 4.5 Event Type Mapping Tables

**Stripe Event Mapping:**

| Stripe Event | Canonical Event |
|--------------|-----------------|
| payment_intent.succeeded | AUTHORIZATION_SUCCEEDED (if not captured) |
| payment_intent.payment_failed | AUTHORIZATION_FAILED |
| charge.captured | CAPTURE_SUCCEEDED |
| charge.failed | CAPTURE_FAILED |
| charge.refunded | REFUND_SUCCEEDED |
| charge.dispute.created | DISPUTE_OPENED |
| charge.dispute.closed (won) | DISPUTE_WON |
| charge.dispute.closed (lost) | DISPUTE_LOST |

**Adyen Notification Mapping:**

| Adyen Notification | Canonical Event |
|--------------------|-----------------|
| AUTHORISATION (success=true) | AUTHORIZATION_SUCCEEDED |
| AUTHORISATION (success=false) | AUTHORIZATION_FAILED |
| CAPTURE | CAPTURE_SUCCEEDED |
| CAPTURE_FAILED | CAPTURE_FAILED |
| CANCELLATION | VOID_SUCCEEDED |
| REFUND | REFUND_SUCCEEDED |
| CHARGEBACK | DISPUTE_OPENED |
| CHARGEBACK_REVERSED | DISPUTE_WON |

**PayPal Event Mapping:**

| PayPal Event | Canonical Event |
|--------------|-----------------|
| PAYMENT.AUTHORIZATION.CREATED | AUTHORIZATION_SUCCEEDED |
| PAYMENT.AUTHORIZATION.VOIDED | AUTHORIZATION_FAILED |
| PAYMENT.CAPTURE.COMPLETED | CAPTURE_SUCCEEDED |
| PAYMENT.CAPTURE.DENIED | CAPTURE_FAILED |
| PAYMENT.CAPTURE.REFUNDED | REFUND_SUCCEEDED |
| CUSTOMER.DISPUTE.CREATED | DISPUTE_OPENED |
| CUSTOMER.DISPUTE.RESOLVED | DISPUTE_WON or DISPUTE_LOST |

### 4.6 Canonical Decline Codes

```go
type CanonicalDeclineCode string

const (
    // Soft Declines - Retry Eligible
    DeclineInsufficientFunds  CanonicalDeclineCode = "INSUFFICIENT_FUNDS"
    DeclineOverLimit          CanonicalDeclineCode = "OVER_LIMIT"
    DeclineGenericDecline     CanonicalDeclineCode = "GENERIC_DECLINE"
    DeclineDoNotHonor         CanonicalDeclineCode = "DO_NOT_HONOR"
    DeclineTryAgain           CanonicalDeclineCode = "TRY_AGAIN"
    DeclineProcessingError    CanonicalDeclineCode = "PROCESSING_ERROR"
    
    // Hard Declines - Not Retry Eligible
    DeclineCardExpired        CanonicalDeclineCode = "CARD_EXPIRED"
    DeclineInvalidNumber      CanonicalDeclineCode = "INVALID_NUMBER"
    DeclineInvalidCVV         CanonicalDeclineCode = "INVALID_CVV"
    DeclineAccountClosed      CanonicalDeclineCode = "ACCOUNT_CLOSED"
    DeclineCardRestricted     CanonicalDeclineCode = "CARD_RESTRICTED"
    
    // Fraud
    DeclineFraudSuspicion     CanonicalDeclineCode = "FRAUD_SUSPICION"
    DeclineStolenCard         CanonicalDeclineCode = "STOLEN_CARD"
    DeclineLostCard           CanonicalDeclineCode = "LOST_CARD"
)
```

### 4.7 Decline Code Mapping Tables

**Stripe Decline Mapping:**

| Stripe Code | Canonical Code | Decline Type |
|-------------|----------------|--------------|
| insufficient_funds | INSUFFICIENT_FUNDS | SOFT |
| card_declined | GENERIC_DECLINE | SOFT |
| do_not_honor | DO_NOT_HONOR | SOFT |
| expired_card | CARD_EXPIRED | HARD |
| incorrect_cvc | INVALID_CVV | HARD |
| fraudulent | FRAUD_SUSPICION | FRAUD |
| lost_card | LOST_CARD | FRAUD |
| stolen_card | STOLEN_CARD | FRAUD |

**Adyen Decline Mapping:**

| Adyen Reason Code | Canonical Code | Decline Type |
|-------------------|----------------|--------------|
| Refused:51 | INSUFFICIENT_FUNDS | SOFT |
| Refused:05 | GENERIC_DECLINE | SOFT |
| Refused:57 | DO_NOT_HONOR | SOFT |
| Refused:33 | CARD_EXPIRED | HARD |
| Refused:63 | CARD_RESTRICTED | HARD |
| Refused:59 | FRAUD_SUSPICION | FRAUD |
| Refused:41 | LOST_CARD | FRAUD |
| Refused:43 | STOLEN_CARD | FRAUD |

**PayPal Decline Mapping:**

| PayPal Code | Canonical Code | Decline Type |
|-------------|----------------|--------------|
| INSUFFICIENT_FUNDS | INSUFFICIENT_FUNDS | SOFT |
| INSTRUMENT_DECLINED | GENERIC_DECLINE | SOFT |
| DO_NOT_HONOR | DO_NOT_HONOR | SOFT |
| CREDIT_CARD_EXPIRED | CARD_EXPIRED | HARD |
| CREDIT_CARD_CVV_CHECK_FAILED | INVALID_CVV | HARD |
| TRANSACTION_REFUSED | FRAUD_SUSPICION | FRAUD |

### 4.8 Adapter Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        STRIPE ADAPTER FLOW                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   POST /webhooks/stripe                                                     │
│         │                                                                   │
│         ▼                                                                   │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │  1. VERIFY SIGNATURE                                                 │  │
│   │                                                                      │  │
│   │  signature := r.Header.Get("Stripe-Signature")                      │  │
│   │  event, err := webhook.ConstructEvent(body, signature, secret)      │  │
│   │                                                                      │  │
│   │  if err != nil {                                                    │  │
│   │      return 401 Unauthorized                                        │  │
│   │  }                                                                   │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│         │                                                                   │
│         ▼                                                                   │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │  2. PARSE EVENT                                                      │  │
│   │                                                                      │  │
│   │  switch event.Type {                                                │  │
│   │  case "payment_intent.succeeded":                                   │  │
│   │      var pi stripe.PaymentIntent                                    │  │
│   │      json.Unmarshal(event.Data.Raw, &pi)                           │  │
│   │  case "charge.captured":                                            │  │
│   │      var charge stripe.Charge                                       │  │
│   │      json.Unmarshal(event.Data.Raw, &charge)                       │  │
│   │  }                                                                   │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│         │                                                                   │
│         ▼                                                                   │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │  3. MAP TO CANONICAL                                                 │  │
│   │                                                                      │  │
│   │  canonicalEvent := CanonicalPaymentEvent{                           │  │
│   │      EventType:         mapStripeEventType(event.Type),             │  │
│   │      PaymentID:         lookupInternalID(pi.ID),                    │  │
│   │      ProviderPaymentID: pi.ID,                                      │  │
│   │      Provider:          ProviderStripe,                             │  │
│   │      Amount:            pi.Amount,                                  │  │
│   │      Currency:          strings.ToUpper(pi.Currency),               │  │
│   │      Status:            mapStripeStatus(pi.Status),                 │  │
│   │      FailureCode:       mapStripeDeclineCode(pi.LastPaymentError),  │  │
│   │      RawPayload:        event.Data.Raw,                             │  │
│   │      ReceivedAt:        time.Now(),                                 │  │
│   │  }                                                                   │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│         │                                                                   │
│         ▼                                                                   │
│   ┌─────────────────────────────────────────────────────────────────────┐  │
│   │  4. SIGNAL WORKFLOW                                                  │  │
│   │                                                                      │  │
│   │  workflowID := fmt.Sprintf("payment-%s", canonicalEvent.PaymentID)  │  │
│   │  err := temporalClient.SignalWorkflow(                              │  │
│   │      ctx,                                                           │  │
│   │      workflowID,                                                    │  │
│   │      "",                                                            │  │
│   │      SignalProviderEvent,                                           │  │
│   │      canonicalEvent,                                                │  │
│   │  )                                                                   │  │
│   └─────────────────────────────────────────────────────────────────────┘  │
│         │                                                                   │
│         ▼                                                                   │
│   Return 200 OK to Stripe                                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.9 Handling Unknown Events and Codes

```go
// For unknown event types
func mapStripeEventType(stripeType string) CanonicalEventType {
    if canonical, ok := stripeEventMap[stripeType]; ok {
        return canonical
    }
    // Log unmapped event for future mapping
    log.Warn("unmapped stripe event type", "type", stripeType)
    return EventUnknown
}

// For unknown decline codes
func mapStripeDeclineCode(code string) CanonicalDeclineCode {
    if canonical, ok := stripeDeclineMap[code]; ok {
        return canonical
    }
    // Log unmapped code for future mapping
    log.Warn("unmapped stripe decline code", "code", code)
    // Default to generic decline (retry eligible) for safety
    return DeclineGenericDecline
}
```

---

## 5. Domain Model Design

### 5.1 Entity Relationship Diagram

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
│   │ provider        │◀────────│ provider        │                          │
│   │ provider_pmt_id │         │ is_default      │                          │
│   │ payment_method_id────────▶│ created_at      │                          │
│   │ metadata        │         └─────────────────┘                          │
│   │ created_at      │                                                       │
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
│   │ provider        │                                                       │
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
│   │ provider        │                                                       │
│   │ provider_code   │  (Raw provider response)                             │
│   │ canonical_code  │  (Normalized)                                        │
│   │ decline_type    │                                                       │
│   │ processor_txn_id│                                                       │
│   │ idempotency_key │                                                       │
│   │ created_at      │                                                       │
│   │ completed_at    │                                                       │
│   └─────────────────┘                                                       │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 PaymentIntent State Machine

```
                              ┌─────────────────┐
                              │     CREATED     │
                              └────────┬────────┘
                                       │
                                       │ attach_method
                                       ▼
                              ┌─────────────────┐
                              │ REQUIRES_AUTH   │
                              └────────┬────────┘
                                       │
                           ┌───────────┼───────────┐
                           │           │           │
                           │ authorize │           │ cancel
                           ▼           │           ▼
                  ┌─────────────────┐  │  ┌─────────────────┐
                  │   AUTHORIZED    │  │  │   CANCELLED     │
                  └────────┬────────┘  │  └─────────────────┘
                           │           │
               ┌───────────┼───────────┤
               │           │           │
               │ capture   │ void      │ decline (soft)
               ▼           ▼           ▼
      ┌─────────────┐ ┌──────────┐ ┌─────────────────┐
      │  CAPTURED   │ │ VOIDED   │ │   RECOVERING    │
      └─────────────┘ └──────────┘ └────────┬────────┘
                                            │
                                ┌───────────┼───────────┐
                                │           │           │
                                │ success   │ exhaust   │
                                ▼           ▼           ▼
                       ┌────────────┐ ┌──────────┐ ┌──────────┐
                       │ AUTHORIZED │ │  FAILED  │ │CANCELLED │
                       └────────────┘ └──────────┘ └──────────┘
```

### 5.3 PaymentAttempt State Machine (Linear)

```
     PENDING ────▶ PROCESSING ────▶ SUCCEEDED
                        │
                        └────────▶ FAILED

     (No backward transitions. New retry = new attempt record)
```

---

## 6. Temporal Workflow Design

### 6.1 PaymentWorkflow Overview

**Workflow Characteristics:**
- **Execution Timeout:** 30 days (maximum dunning window)
- **Task Queue:** "payments"
- **ID Pattern:** `payment-{idempotency_key}`
- **ID Reuse Policy:** REJECT_DUPLICATE

**Signal Definitions:**

| Signal | Purpose | Payload |
|--------|---------|---------|
| `provider-event` | Receive canonical event from adapter | CanonicalPaymentEvent |
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

### 6.2 Provider Selection Logic

The workflow uses the payment intent's `provider` field to select activities:

```go
func (w *PaymentWorkflow) executeAuthorization(ctx workflow.Context) error {
    var authResult AuthorizationResult
    
    switch w.state.Provider {
    case ProviderStripe:
        err := workflow.ExecuteActivity(ctx, w.activities.StripeAuthorize, w.state.Intent).Get(ctx, &authResult)
    case ProviderAdyen:
        err := workflow.ExecuteActivity(ctx, w.activities.AdyenAuthorize, w.state.Intent).Get(ctx, &authResult)
    case ProviderPayPal:
        err := workflow.ExecuteActivity(ctx, w.activities.PayPalAuthorize, w.state.Intent).Get(ctx, &authResult)
    }
    
    // Process result using canonical types
    return w.processAuthResult(ctx, authResult)
}
```

### 6.3 Workflow State Machine

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
│   │         Note: provider already set on intent │                          │
│   └────────────────────┬────────────────────────┘                          │
│                        │                                                    │
│                        ▼                                                    │
│   ┌─────────────────────────────────────────────┐                          │
│   │         AWAIT SIGNAL                         │                          │
│   │                                              │                          │
│   │   Selector:                                  │                          │
│   │   - authorize signal → AUTHORIZATION        │                          │
│   │   - provider-event signal → handle event    │                          │
│   │   - cancel signal → CANCELLED               │                          │
│   │   - update-method → update & continue       │                          │
│   └────────────────────┬────────────────────────┘                          │
│                        │                                                    │
│                        ▼                                                    │
│   ┌─────────────────────────────────────────────┐                          │
│   │         AUTHORIZATION                        │                          │
│   │                                              │                          │
│   │   // Select activity based on provider      │                          │
│   │   switch intent.Provider {                  │                          │
│   │     case STRIPE:  StripeAuthActivity        │                          │
│   │     case ADYEN:   AdyenAuthActivity         │                          │
│   │     case PAYPAL:  PayPalAuthActivity        │                          │
│   │   }                                          │                          │
│   │                                              │                          │
│   │   // Result is canonical AuthorizationResult│                          │
│   └────────────────────┬────────────────────────┘                          │
│                        │                                                    │
│            ┌───────────┴───────────┬─────────────────┐                     │
│            │                       │                 │                      │
│            ▼                       ▼                 ▼                      │
│         SUCCESS              SOFT DECLINE      HARD DECLINE                │
│            │                       │                 │                      │
│            │                       │                 ▼                      │
│            │                       │         ┌─────────────────┐           │
│            │                       │         │     FAILED      │           │
│            │                       │         │ (canonical code)│           │
│            │                       │         └─────────────────┘           │
│            │                       │                                       │
│            │                       ▼                                       │
│            │               ┌─────────────────┐                             │
│            │               │   RECOVERING    │                             │
│            │               │                 │                             │
│            │               │ Uses canonical  │                             │
│            │               │ decline codes   │                             │
│            │               └─────────────────┘                             │
│            │                                                               │
│            ▼                                                               │
│   ┌─────────────────────────────────────────────┐                         │
│   │         AUTHORIZED                           │                         │
│   │                                              │                         │
│   │   CreateHoldActivity (provider-agnostic)    │                         │
│   │   UpdateBalancesActivity                    │                         │
│   │                                              │                         │
│   │   If capture_method == AUTOMATIC:           │                         │
│   │     → immediate capture                      │                         │
│   │   Else:                                      │                         │
│   │     → await capture signal                   │                         │
│   └────────────────────┬────────────────────────┘                         │
│                        │                                                   │
│                        ▼                                                   │
│   ┌─────────────────────────────────────────────┐                         │
│   │         CAPTURE                              │                         │
│   │                                              │                         │
│   │   // Select activity based on provider      │                         │
│   │   switch intent.Provider {                  │                         │
│   │     case STRIPE:  StripeCaptureActivity     │                         │
│   │     case ADYEN:   AdyenCaptureActivity      │                         │
│   │     case PAYPAL:  PayPalCaptureActivity     │                         │
│   │   }                                          │                         │
│   │                                              │                         │
│   │   RecordLedgerActivity (provider-agnostic)  │                         │
│   │   WriteOutboxActivity (canonical events)    │                         │
│   └────────────────────┬────────────────────────┘                         │
│                        │                                                   │
│                        ▼                                                   │
│   ┌─────────────────────────────────────────────┐                         │
│   │         CAPTURED                             │                         │
│   │                                              │                         │
│   │   Workflow complete                          │                         │
│   └─────────────────────────────────────────────┘                         │
│                                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.4 RecoveryWorkflow Design

The RecoveryWorkflow handles intelligent retry scheduling using canonical decline codes:

```go
func (w *RecoveryWorkflow) Execute(ctx workflow.Context, input RecoveryInput) error {
    // Decline classification uses canonical codes - NOT provider codes
    declineType := classifyCanonicalDecline(input.CanonicalDeclineCode)
    
    for attempt := 1; attempt <= w.maxAttempts; attempt++ {
        // Calculate retry time based on canonical decline type
        retryDelay := w.calculateRetryDelay(declineType, attempt)
        
        // Durable sleep with signal handling
        err := w.sleepWithSignals(ctx, retryDelay)
        if err == ErrCancelled {
            return nil
        }
        
        // Create new attempt record
        w.createAttemptRecord(ctx, attempt)
        
        // Retry using provider-specific activity
        result := w.retryAuthorization(ctx)
        if result.Succeeded {
            return nil
        }
        
        // Update canonical decline code from new attempt
        declineType = classifyCanonicalDecline(result.CanonicalDeclineCode)
    }
    
    return ErrRetriesExhausted
}
```

**Retry Timing by Canonical Decline Type:**

| Canonical Decline Type | Attempt 1 | Attempt 2 | Attempt 3 | Attempt 4 |
|------------------------|-----------|-----------|-----------|-----------|
| INSUFFICIENT_FUNDS | Next payday | +1 payday | +2 paydays | +3 paydays |
| GENERIC_DECLINE | 4 hours | 12 hours | 24 hours | 48 hours |
| DO_NOT_HONOR | 6 hours | 24 hours | 48 hours | 5 days |
| TRY_AGAIN | 1 hour | 4 hours | 12 hours | 24 hours |

---

## 7. Activity Design

### 7.1 Activity Categories

Activities are categorized by whether they interact with external providers:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          ACTIVITY TAXONOMY                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  PROVIDER-SPECIFIC ACTIVITIES           PROVIDER-AGNOSTIC ACTIVITIES        │
│  (Outbound PSP calls)                   (Internal operations)               │
│  ─────────────────────────              ──────────────────────────          │
│                                                                             │
│  ┌─────────────────────────┐           ┌─────────────────────────┐         │
│  │ StripeAuthActivity      │           │ ValidateIntentActivity  │         │
│  │ StripeCaptureActivity   │           │ CreateHoldActivity      │         │
│  │ StripeRefundActivity    │           │ UpdateHoldActivity      │         │
│  └─────────────────────────┘           │ RecordLedgerActivity    │         │
│                                         │ WriteOutboxActivity     │         │
│  ┌─────────────────────────┐           │ ClassifyDeclineActivity │         │
│  │ AdyenAuthActivity       │           │ CalculateRetryActivity  │         │
│  │ AdyenCaptureActivity    │           │ CheckAccountActivity    │         │
│  │ AdyenRefundActivity     │           │ UpdateBalancesActivity  │         │
│  └─────────────────────────┘           └─────────────────────────┘         │
│                                                                             │
│  ┌─────────────────────────┐                                               │
│  │ PayPalAuthActivity      │           INPUT: Canonical types               │
│  │ PayPalCaptureActivity   │           OUTPUT: Canonical types              │
│  │ PayPalRefundActivity    │           (Provider details encapsulated)      │
│  └─────────────────────────┘                                               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Provider-Specific Activity Interface

All provider activities implement a common interface returning canonical results:

```go
// Common result type used by all provider authorization activities
type AuthorizationResult struct {
    Succeeded          bool
    AuthorizationCode  string
    ProviderTxnID      string
    CanonicalStatus    CanonicalStatus
    CanonicalDecline   *CanonicalDeclineCode
    DeclineType        *DeclineType
    HoldExpiresAt      time.Time
}

// Common result type used by all provider capture activities
type CaptureResult struct {
    Succeeded        bool
    ProviderTxnID    string
    CapturedAmount   int64
    CanonicalStatus  CanonicalStatus
    CanonicalDecline *CanonicalDeclineCode
}
```

### 7.3 Activity Retry Configuration

| Activity Type | Initial Interval | Backoff | Max Attempts | Non-Retryable Errors |
|---------------|------------------|---------|--------------|---------------------|
| **Provider Auth** | 1s | 2.0 | 3 | HardDecline, Fraud |
| **Provider Capture** | 1s | 2.0 | 3 | HardDecline |
| **Database Read** | 100ms | 2.0 | 5 | None |
| **Database Write** | 100ms | 2.0 | 5 | UniqueViolation |
| **Ledger Update** | 100ms | 2.0 | 5 | None |

### 7.4 Idempotency Key Generation

Provider activities use deterministic idempotency keys:

```go
func generateProviderIdempotencyKey(workflowID string, provider Provider, operation string, attempt int) string {
    return fmt.Sprintf("%s-%s-%s-%d", workflowID, provider, operation, attempt)
}

// Examples:
// payment-abc123-STRIPE-authorize-1
// payment-abc123-STRIPE-capture-1
// payment-abc123-ADYEN-authorize-2  (retry attempt)
```

---

## 8. Database Schema

### 8.1 Core Tables

**payment_intents**
```sql
CREATE TABLE payment_intents (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key       VARCHAR(100) UNIQUE NOT NULL,
    customer_id           VARCHAR(50) NOT NULL,
    amount                DECIMAL(19,4) NOT NULL,
    currency              VARCHAR(3) NOT NULL,
    status                VARCHAR(20) NOT NULL,
    capture_method        VARCHAR(20) NOT NULL DEFAULT 'AUTOMATIC',
    provider              VARCHAR(20) NOT NULL,  -- STRIPE, ADYEN, PAYPAL
    provider_payment_id   VARCHAR(100),          -- Provider's ID for this payment
    payment_method_id     UUID REFERENCES payment_methods(id),
    workflow_id           VARCHAR(100) UNIQUE,
    metadata              JSONB,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_intents_customer ON payment_intents(customer_id);
CREATE INDEX idx_intents_provider ON payment_intents(provider);
CREATE INDEX idx_intents_provider_pmt_id ON payment_intents(provider, provider_payment_id);
```

**payment_attempts**
```sql
CREATE TABLE payment_attempts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_intent_id     UUID NOT NULL REFERENCES payment_intents(id),
    attempt_number        INT NOT NULL,
    status                VARCHAR(20) NOT NULL,
    provider              VARCHAR(20) NOT NULL,
    provider_response     VARCHAR(100),          -- Raw provider code
    canonical_decline     VARCHAR(50),           -- Normalized code
    decline_type          VARCHAR(20),
    processor_txn_id      VARCHAR(100),
    idempotency_key       VARCHAR(150) UNIQUE NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at          TIMESTAMPTZ,
    
    CONSTRAINT uq_attempt UNIQUE (payment_intent_id, attempt_number)
);

CREATE INDEX idx_attempts_intent ON payment_attempts(payment_intent_id);
CREATE INDEX idx_attempts_canonical ON payment_attempts(canonical_decline);
```

**decline_code_mappings**
```sql
CREATE TABLE decline_code_mappings (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider              VARCHAR(20) NOT NULL,  -- STRIPE, ADYEN, PAYPAL
    provider_code         VARCHAR(100) NOT NULL, -- Raw provider code
    canonical_code        VARCHAR(50) NOT NULL,  -- Normalized code
    decline_type          VARCHAR(20) NOT NULL,  -- SOFT, HARD, FRAUD, TEMPORARY
    description           TEXT,
    retry_eligible        BOOLEAN NOT NULL DEFAULT false,
    suggested_action      TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT uq_provider_code UNIQUE (provider, provider_code)
);

CREATE INDEX idx_decline_provider ON decline_code_mappings(provider);
CREATE INDEX idx_decline_canonical ON decline_code_mappings(canonical_code);
```

### 8.2 Outbox Table

```sql
CREATE TABLE outbox (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type        VARCHAR(50) NOT NULL,
    aggregate_id          UUID NOT NULL,
    event_type            VARCHAR(50) NOT NULL,  -- Canonical event types only
    payload               JSONB NOT NULL,        -- Canonical event payload
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Required for CDC
ALTER TABLE outbox REPLICA IDENTITY FULL;
```

**Important:** The outbox contains only canonical events. Provider-specific details are stripped before insertion.

---

## 9. Event System Design

### 9.1 Outbox Events Are Canonical

Events written to the outbox use canonical types, not provider-specific types:

```go
func (a *WriteOutboxActivity) Execute(ctx context.Context, input WriteOutboxInput) error {
    // Event payload uses canonical types
    payload := CanonicalPaymentCapturedEvent{
        PaymentID:     input.PaymentID,
        Amount:        input.Amount,
        Currency:      input.Currency,
        Provider:      input.Provider,  // Provider included for reference only
        CapturedAt:    time.Now(),
        // NO provider-specific fields
    }
    
    event := OutboxEvent{
        AggregateType: "PaymentIntent",
        AggregateID:   input.PaymentID,
        EventType:     "payment.captured",  // Canonical event type
        Payload:       payload,
    }
    
    return a.repo.WriteOutbox(ctx, event)
}
```

### 9.2 Kafka Topic Structure

| Topic | Event Types | Consumers |
|-------|-------------|-----------|
| `payments.authorized` | AUTHORIZATION_SUCCEEDED | Analytics, Notifications |
| `payments.captured` | CAPTURE_SUCCEEDED | Accounting, Reporting |
| `payments.failed` | AUTHORIZATION_FAILED, CAPTURE_FAILED | Alerts, Customer Service |
| `payments.disputes` | DISPUTE_OPENED, DISPUTE_WON, DISPUTE_LOST | Risk, Accounting |

### 9.3 Event Schema (Avro)

```json
{
  "type": "record",
  "name": "PaymentCapturedEvent",
  "namespace": "com.payments.events",
  "fields": [
    {"name": "event_id", "type": "string"},
    {"name": "event_type", "type": "string"},
    {"name": "payment_id", "type": "string"},
    {"name": "amount", "type": "long"},
    {"name": "currency", "type": "string"},
    {"name": "provider", "type": {"type": "enum", "name": "Provider", "symbols": ["STRIPE", "ADYEN", "PAYPAL"]}},
    {"name": "captured_at", "type": {"type": "long", "logicalType": "timestamp-millis"}},
    {"name": "correlation_id", "type": "string"}
  ]
}
```

---

## 10. API Design

### 10.1 Endpoint Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/v1/intents | Create payment intent (specify provider) |
| GET | /api/v1/intents/:id | Get intent status |
| PUT | /api/v1/intents/:id/method | Attach payment method |
| POST | /api/v1/intents/:id/authorize | Request authorization |
| POST | /api/v1/intents/:id/capture | Capture authorized funds |
| POST | /api/v1/intents/:id/cancel | Cancel intent |
| GET | /api/v1/intents/:id/attempts | Get attempt history |
| POST | /webhooks/stripe | Stripe webhook receiver |
| POST | /webhooks/adyen | Adyen notification receiver |
| POST | /webhooks/paypal | PayPal webhook receiver |
| GET | /health | Health check |
| GET | /metrics | Prometheus metrics |

### 10.2 Create Intent Request

```json
{
  "amount": "100.00",
  "currency": "USD",
  "customer_id": "cust_123",
  "provider": "STRIPE",
  "capture_method": "manual",
  "payment_method_id": "pm_456",
  "metadata": {
    "order_id": "order_789"
  }
}
```

### 10.3 Webhook Endpoints

Each provider has a dedicated webhook endpoint:

| Endpoint | Expected Response | Notes |
|----------|-------------------|-------|
| POST /webhooks/stripe | 200 OK | Must respond within 30 seconds |
| POST /webhooks/adyen | "[accepted]" | Must respond within 30 seconds |
| POST /webhooks/paypal | 200 OK | Or 503 for retry |

---

## 11. Idempotency Implementation

### 11.1 Atomic Phases Pattern

| Phase | Description | Recovery Point |
|-------|-------------|----------------|
| **started** | Request received | Can restart from beginning |
| **validated** | Input validated | Skip validation on retry |
| **authorized** | Provider called | Return cached result |
| **hold_created** | Hold recorded | Skip hold creation |
| **ledger_written** | Entries created | Skip ledger |
| **event_written** | Outbox event | Skip outbox |
| **completed** | All done | Return response |

---

## 12. Concurrency and Race Conditions

### 12.1 Webhook + API Race Condition

**Problem:** Provider may send webhook AND return API response simultaneously.

**Solution:**
```sql
-- Lock payment before processing
SELECT * FROM payment_intents 
WHERE id = $1 
FOR UPDATE NOWAIT;
```

### 12.2 Duplicate Webhook Handling

Adapters must handle duplicate webhooks idempotently:

```go
func (a *StripeAdapter) HandleWebhook(ctx context.Context, event stripe.Event) error {
    // Check if we've already processed this event
    if a.repo.EventProcessed(ctx, event.ID) {
        return nil  // Already processed, return success
    }
    
    // Process event...
    
    // Mark as processed
    a.repo.MarkEventProcessed(ctx, event.ID)
    
    return nil
}
```

---

## 13. Observability Design

### 13.1 Metrics by Provider

| Metric | Labels | Purpose |
|--------|--------|---------|
| payment_authorization_total | provider, status | Volume by provider |
| payment_authorization_duration_seconds | provider | Latency by provider |
| payment_decline_total | provider, canonical_code | Decline patterns |
| webhook_received_total | provider, event_type | Webhook volume |
| webhook_processing_duration_seconds | provider | Webhook latency |

### 13.2 Alerting Rules

| Alert | Condition | Severity |
|-------|-----------|----------|
| Provider High Error Rate | error_rate{provider=X} > 5% for 5m | Warning |
| Provider Down | success_rate{provider=X} < 1% for 2m | Critical |
| Unmapped Decline Code | unmapped_decline_total increases | Info |
| Webhook Signature Failure | signature_failure_total > 10 in 5m | Warning |

---

## 14. Project Structure

```
payment-service/
├── cmd/
│   ├── api/
│   │   └── main.go
│   ├── worker/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── adapter/                          # ADAPTER LAYER
│   │   ├── canonical.go                  # Canonical event types
│   │   ├── decline_codes.go              # Canonical decline codes
│   │   ├── stripe/
│   │   │   ├── adapter.go                # Webhook handler
│   │   │   ├── mapper.go                 # Event/code mapping
│   │   │   ├── signature.go              # Signature verification
│   │   │   └── adapter_test.go
│   │   ├── adyen/
│   │   │   ├── adapter.go
│   │   │   ├── mapper.go
│   │   │   ├── hmac.go
│   │   │   └── adapter_test.go
│   │   └── paypal/
│   │       ├── adapter.go
│   │       ├── mapper.go
│   │       ├── verify.go
│   │       └── adapter_test.go
│   ├── api/
│   │   ├── handler/
│   │   │   ├── intent.go
│   │   │   ├── account.go
│   │   │   └── health.go
│   │   ├── middleware/
│   │   │   ├── idempotency.go
│   │   │   └── logging.go
│   │   └── router.go
│   ├── workflow/
│   │   ├── payment.go                    # PaymentWorkflow
│   │   ├── recovery.go                   # RecoveryWorkflow
│   │   ├── signals.go
│   │   ├── queries.go
│   │   └── state.go
│   ├── activity/
│   │   ├── provider/                     # Provider-specific
│   │   │   ├── stripe.go
│   │   │   ├── adyen.go
│   │   │   └── paypal.go
│   │   ├── ledger.go                     # Provider-agnostic
│   │   ├── outbox.go
│   │   ├── decline.go
│   │   └── validation.go
│   ├── domain/
│   │   ├── intent.go
│   │   ├── hold.go
│   │   ├── attempt.go
│   │   ├── account.go
│   │   └── errors.go
│   ├── repository/
│   │   ├── intent.go
│   │   ├── hold.go
│   │   ├── attempt.go
│   │   ├── decline_mapping.go
│   │   └── outbox.go
│   └── config/
│       └── config.go
├── pkg/
│   ├── stripe/
│   │   └── client.go
│   ├── adyen/
│   │   └── client.go
│   ├── paypal/
│   │   └── client.go
│   └── temporal/
│       └── client.go
├── migrations/
│   ├── 001_create_accounts.up.sql
│   ├── 002_create_intents.up.sql
│   ├── 003_create_holds.up.sql
│   ├── 004_create_attempts.up.sql
│   ├── 005_create_ledger.up.sql
│   ├── 006_create_outbox.up.sql
│   ├── 007_create_decline_mappings.up.sql
│   └── 008_seed_decline_codes.up.sql
├── deployments/
│   ├── docker-compose.yml
│   └── debezium/
│       └── connector.json
├── Makefile
├── go.mod
└── README.md
```

---

## 15. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)
- Project structure with adapter layer
- Docker Compose with all infrastructure
- Database migrations including decline mappings
- Basic API server and Temporal worker
- CDC pipeline operational

### Phase 2: Adapter Layer (Week 3-4)
- StripeAdapter with signature verification
- AdyenAdapter with HMAC verification  
- Canonical event and decline code definitions
- Decline code mapping tables
- Adapter unit tests

### Phase 3: Authorization Flow (Week 5-6)
- PaymentWorkflow structure
- Provider-specific authorization activities
- AuthorizationHold management
- Multi-balance tracking

### Phase 4: Capture and Ledger (Week 7-8)
- Provider-specific capture activities
- Double-entry ledger
- Transactional outbox with canonical events
- CDC event publication

### Phase 5: Recovery and Production Readiness (Week 9-10)
- RecoveryWorkflow with canonical decline handling
- Race condition protection
- Comprehensive testing
- Documentation

---

## 16. Testing Strategy

### 16.1 Adapter Testing

Each adapter requires extensive testing with real webhook payloads:

```go
func TestStripeAdapter_ChargeSucceeded(t *testing.T) {
    // Use actual Stripe webhook payload format
    payload := loadFixture("stripe_charge_succeeded.json")
    signature := generateTestSignature(payload)
    
    adapter := NewStripeAdapter(config)
    event, err := adapter.Process(payload, signature)
    
    assert.NoError(t, err)
    assert.Equal(t, EventCaptureSucceeded, event.EventType)
    assert.Equal(t, CanonicalStatusSucceeded, event.Status)
}

func TestStripeAdapter_DeclineMapping(t *testing.T) {
    testCases := []struct {
        stripeCode   string
        canonicalCode CanonicalDeclineCode
        declineType  DeclineType
    }{
        {"insufficient_funds", DeclineInsufficientFunds, DeclineTypeSoft},
        {"expired_card", DeclineCardExpired, DeclineTypeHard},
        {"fraudulent", DeclineFraudSuspicion, DeclineTypeFraud},
    }
    
    for _, tc := range testCases {
        t.Run(tc.stripeCode, func(t *testing.T) {
            canonical := mapStripeDeclineCode(tc.stripeCode)
            assert.Equal(t, tc.canonicalCode, canonical)
        })
    }
}
```

### 16.2 Workflow Testing

Test workflows using Temporal's test framework with mocked activities:

```go
func TestPaymentWorkflow_MultiProvider(t *testing.T) {
    providers := []Provider{ProviderStripe, ProviderAdyen, ProviderPayPal}
    
    for _, provider := range providers {
        t.Run(string(provider), func(t *testing.T) {
            testSuite := &testsuite.WorkflowTestSuite{}
            env := testSuite.NewTestWorkflowEnvironment()
            
            // Mock provider-specific activity
            switch provider {
            case ProviderStripe:
                env.OnActivity(activities.StripeAuthorize, mock.Anything, mock.Anything).
                    Return(AuthorizationResult{Succeeded: true}, nil)
            // ... other providers
            }
            
            input := PaymentWorkflowInput{
                Intent: PaymentIntent{Provider: provider},
            }
            
            env.ExecuteWorkflow(PaymentWorkflow, input)
            assert.True(t, env.IsWorkflowCompleted())
        })
    }
}
```

---

## 17. Infrastructure Configuration

### 17.1 Docker Compose Services

| Service | Purpose |
|---------|---------|
| temporal | Workflow server |
| temporal-ui | Web UI |
| app-db | Application database |
| pgbouncer | Connection pooling |
| kafka | Event streaming |
| zookeeper | Kafka coordination |
| schema-registry | Avro schemas |
| debezium | CDC connector |
| api | Payment API |
| worker | Temporal worker |

### 17.2 Environment Configuration

```yaml
# Provider credentials (per environment)
STRIPE_SECRET_KEY: sk_test_...
STRIPE_WEBHOOK_SECRET: whsec_...
ADYEN_API_KEY: ...
ADYEN_HMAC_KEY: ...
PAYPAL_CLIENT_ID: ...
PAYPAL_CLIENT_SECRET: ...
PAYPAL_WEBHOOK_ID: ...
```

---

## Appendix A: Canonical Code Quick Reference

**Event Types:**
- AUTHORIZATION_SUCCEEDED, AUTHORIZATION_FAILED
- CAPTURE_SUCCEEDED, CAPTURE_FAILED
- VOID_SUCCEEDED, REFUND_SUCCEEDED
- DISPUTE_OPENED, DISPUTE_WON, DISPUTE_LOST

**Decline Codes (Soft):**
- INSUFFICIENT_FUNDS, OVER_LIMIT, GENERIC_DECLINE, DO_NOT_HONOR, TRY_AGAIN

**Decline Codes (Hard):**
- CARD_EXPIRED, INVALID_NUMBER, INVALID_CVV, ACCOUNT_CLOSED

**Decline Codes (Fraud):**
- FRAUD_SUSPICION, STOLEN_CARD, LOST_CARD

---

## Appendix B: Glossary

| Term | Definition |
|------|------------|
| **Adapter** | Component that normalizes provider-specific webhooks to canonical model |
| **Canonical Model** | Provider-agnostic internal representation |
| **Provider** | Third-party payment processor (Stripe, Adyen, PayPal) |
| **Normalize at Edge** | Pattern of converting external formats at system boundaries |

---

*Technical specification incorporating production-grade patterns with multi-provider adapter architecture.*