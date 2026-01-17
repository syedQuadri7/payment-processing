# Payment Processing Service
## Technical Specification

**A Learning Project with Golang and Temporal**

Designed for Credit Union / Banking Domain Experience

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [System Architecture](#2-system-architecture)
3. [Core Domain Models](#3-core-domain-models)
4. [Temporal Workflows](#4-temporal-workflows)
5. [Activities Implementation](#5-activities-implementation)
6. [API Design](#6-api-design)
7. [Database Schema](#7-database-schema)
8. [Event System](#8-event-system)
9. [Project Structure](#9-project-structure)
10. [Implementation Roadmap](#10-implementation-roadmap)
11. [Testing Strategy](#11-testing-strategy)
12. [Docker Compose Setup](#12-docker-compose-setup)

---

## 1. Executive Summary

### 1.1 Project Overview

This document specifies a Payment Processing Service that demonstrates production-grade patterns for financial transaction handling. The project combines intelligent payment retry logic (inspired by Butter Payments) with core banking transaction patterns relevant to credit unions and financial institutions.

The service is built with Golang and Temporal, providing durable workflow orchestration for long-running payment operations that may span hours or days.

### 1.2 Learning Objectives

- Master Temporal workflow patterns: long-running processes, signals, queries, and activity retries
- Implement idempotent payment processing with proper error classification
- Build event-driven architecture with audit trails for compliance
- Design APIs that mirror real banking/fintech integrations
- Practice domain modeling for financial services

### 1.3 Relevance to Credit Union / Banking Roles

This project demonstrates skills directly applicable to positions at institutions like Vancity, including:

- Building scalable data pipelines for transaction processing
- Implementing event-driven architectures
- Handling batch and real-time processing
- Maintaining audit trails for compliance

The patterns align with job requirements for data engineers working with Azure, Databricks, and event streaming platforms.

---

## 2. System Architecture

### 2.1 High-Level Architecture

The system consists of five primary components that work together to process payments reliably:

| Component | Responsibility |
|-----------|----------------|
| **API Server** | HTTP/gRPC endpoints for payment submission, status queries, and webhook reception |
| **Temporal Worker** | Executes workflow and activity code; can scale horizontally |
| **Temporal Server** | Manages workflow state, scheduling, and task queues (run via Docker) |
| **PostgreSQL** | Stores accounts, ledger entries, payment records, and audit logs |
| **Message Broker** | NATS or Kafka for publishing events to downstream consumers |

### 2.2 Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      Payment Processing Service                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────┐                                                    │
│  │   API Server     │     ┌─────────────────────────────────────────┐    │
│  │   (Echo/Fiber)   │────▶│       Temporal Workflows                │    │
│  │                  │     │                                         │    │
│  │  POST /payments  │     │  PaymentWorkflow                        │    │
│  │  GET  /payments  │     │    ├─ ValidatePayment                   │    │
│  │  POST /webhooks  │     │    ├─ RouteByPaymentType                │    │
│  └──────────────────┘     │    ├─ ExecutePayment (with retry)       │    │
│                           │    ├─ UpdateLedger                      │    │
│  ┌──────────────────┐     │    └─ PublishEvents                     │    │
│  │  Signal Handlers │────▶│                                         │    │
│  │                  │     │  ScheduledPaymentWorkflow               │    │
│  │  - UpdateMethod  │     │    ├─ WaitForScheduledTime              │    │
│  │  - CancelPayment │     │    ├─ CheckAccountStatus                │    │
│  │  - ApprovePayment│     │    └─ StartPaymentWorkflow              │    │
│  └──────────────────┘     │                                         │    │
│                           │  RecoveryWorkflow (Butter-style)        │    │
│  ┌──────────────────┐     │    ├─ ClassifyDecline                   │    │
│  │  Query Handlers  │◀────│    ├─ CalculateOptimalRetryTime         │    │
│  │                  │     │    ├─ Sleep until retry time            │    │
│  │  - GetStatus     │     │    └─ RetryOrEscalate                   │    │
│  │  - GetAttempts   │     └─────────────────────────────────────────┘    │
│  │  - GetTimeline   │                                                    │
│  └──────────────────┘              Activities                            │
│                           ┌─────────────────────────────────────────┐    │
│                           │  ProcessCardPayment   → Stripe API      │    │
│                           │  ProcessACHPayment    → ACH Simulator   │    │
│                           │  UpdateLedger         → PostgreSQL      │    │
│                           │  SendNotification     → Email/SMS       │    │
│                           │  PublishEvent         → NATS/Kafka      │    │
│                           │  CheckAccountStatus   → Internal DB     │    │
│                           └─────────────────────────────────────────┘    │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │                    Data Layer                                     │    │
│  │  PostgreSQL: accounts, ledger, payments, audit_log                │    │
│  │  NATS/Kafka: payment.completed, payment.failed, account.updated   │    │
│  └──────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Payment Flow Sequence

1. Client submits payment request via `POST /api/v1/payments`
2. API server validates request and starts PaymentWorkflow with deterministic workflow ID
3. Workflow executes ValidatePayment activity (checks account status, limits, fraud rules)
4. Workflow routes to appropriate payment processor based on payment type (card vs ACH)
5. Payment activity executes with idempotency key; processor returns success or decline
6. On success: UpdateLedger activity records debit/credit entries atomically
7. On soft decline: Workflow transitions to recovery mode (schedules intelligent retry)
8. PublishEvent activity emits `payment.completed` or `payment.failed` to message broker
9. Workflow completes; state persisted for queries and audit

---

## 3. Core Domain Models

### 3.1 Payment Entity

The Payment entity represents a single payment request and tracks its lifecycle through the system:

```go
type Payment struct {
    ID              string          `json:"id"`
    IdempotencyKey  string          `json:"idempotency_key"`
    Type            PaymentType     `json:"type"`      // CARD, ACH, INTERNAL
    Status          PaymentStatus   `json:"status"`
    
    // Parties
    SourceAccountID string          `json:"source_account_id"`
    DestAccountID   string          `json:"dest_account_id,omitempty"`
    
    // Amount
    Amount          decimal.Decimal `json:"amount"`
    Currency        string          `json:"currency"`  // ISO 4217
    
    // Payment Method Details
    PaymentMethod   *PaymentMethod  `json:"payment_method,omitempty"`
    
    // Scheduling
    ScheduledFor    *time.Time      `json:"scheduled_for,omitempty"`
    ProcessedAt     *time.Time      `json:"processed_at,omitempty"`
    
    // Recovery
    AttemptCount    int             `json:"attempt_count"`
    NextRetryAt     *time.Time      `json:"next_retry_at,omitempty"`
    LastDeclineCode string          `json:"last_decline_code,omitempty"`
    
    // Audit
    CreatedAt       time.Time       `json:"created_at"`
    UpdatedAt       time.Time       `json:"updated_at"`
    Metadata        map[string]string `json:"metadata,omitempty"`
}

type PaymentType string

const (
    PaymentTypeCard     PaymentType = "CARD"
    PaymentTypeACH      PaymentType = "ACH"
    PaymentTypeInternal PaymentType = "INTERNAL"
)
```

### 3.2 Payment Status State Machine

Payments transition through well-defined states. Invalid transitions are rejected:

| Status | Description | Valid Transitions |
|--------|-------------|-------------------|
| `PENDING` | Payment created, awaiting processing | PROCESSING, CANCELLED |
| `SCHEDULED` | Future-dated payment waiting | PENDING, CANCELLED |
| `PROCESSING` | Currently executing | SUCCEEDED, FAILED, RECOVERING |
| `RECOVERING` | Soft decline, scheduled for retry | PROCESSING, FAILED, CANCELLED |
| `SUCCEEDED` | Payment completed successfully | *(terminal)* |
| `FAILED` | Hard decline or exhausted retries | *(terminal)* |
| `CANCELLED` | Cancelled by user or system | *(terminal)* |

```go
type PaymentStatus string

const (
    StatusPending    PaymentStatus = "PENDING"
    StatusScheduled  PaymentStatus = "SCHEDULED"
    StatusProcessing PaymentStatus = "PROCESSING"
    StatusRecovering PaymentStatus = "RECOVERING"
    StatusSucceeded  PaymentStatus = "SUCCEEDED"
    StatusFailed     PaymentStatus = "FAILED"
    StatusCancelled  PaymentStatus = "CANCELLED"
)
```

### 3.3 Account and Ledger Models

Double-entry bookkeeping ensures every transaction is balanced:

```go
type Account struct {
    ID            string          `json:"id"`
    Type          AccountType     `json:"type"`     // CHECKING, SAVINGS, LOAN
    OwnerID       string          `json:"owner_id"`
    Balance       decimal.Decimal `json:"balance"`
    AvailableBal  decimal.Decimal `json:"available_balance"`
    Currency      string          `json:"currency"`
    Status        AccountStatus   `json:"status"`   // ACTIVE, FROZEN, CLOSED
    DailyLimit    decimal.Decimal `json:"daily_limit"`
    CreatedAt     time.Time       `json:"created_at"`
}

type LedgerEntry struct {
    ID            string          `json:"id"`
    AccountID     string          `json:"account_id"`
    PaymentID     string          `json:"payment_id"`
    Type          EntryType       `json:"type"`     // DEBIT, CREDIT
    Amount        decimal.Decimal `json:"amount"`
    BalanceAfter  decimal.Decimal `json:"balance_after"`
    Description   string          `json:"description"`
    CreatedAt     time.Time       `json:"created_at"`
}

type EntryType string

const (
    EntryTypeDebit  EntryType = "DEBIT"
    EntryTypeCredit EntryType = "CREDIT"
)
```

### 3.4 Decline Classification

Proper decline classification is critical for retry decisions. This mirrors how Butter Payments handles the 2,000+ decline codes from payment processors:

| Category | Example Codes | Action | Retry Strategy |
|----------|---------------|--------|----------------|
| **Soft Decline** | `insufficient_funds`, `processing_error`, `try_again` | Enter recovery workflow | Intelligent timing based on decline type |
| **Hard Decline** | `stolen_card`, `expired_card`, `invalid_account` | Fail immediately | No retry; require new payment method |
| **Fraud** | `fraudulent`, `pickup_card` | Fail and flag | No retry; trigger fraud review |
| **Temporary** | `rate_limit`, `timeout`, `service_unavailable` | Activity-level retry | Exponential backoff (seconds) |

```go
type DeclineType string

const (
    DeclineTypeSoft      DeclineType = "SOFT"
    DeclineTypeHard      DeclineType = "HARD"
    DeclineTypeFraud     DeclineType = "FRAUD"
    DeclineTypeTemporary DeclineType = "TEMPORARY"
)

func classifyDecline(code string) DeclineType {
    switch code {
    case "insufficient_funds", "generic_decline", "do_not_honor",
         "processing_error", "try_again_later":
        return DeclineTypeSoft
    case "stolen_card", "lost_card", "expired_card", 
         "invalid_account", "card_not_supported":
        return DeclineTypeHard
    case "fraudulent", "pickup_card", "security_violation":
        return DeclineTypeFraud
    case "rate_limit", "timeout", "service_unavailable":
        return DeclineTypeTemporary
    default:
        return DeclineTypeSoft // Default to soft for unknown codes
    }
}
```

---

## 4. Temporal Workflows

### 4.1 Core Concepts Review

Temporal provides durable execution for long-running business processes. Key concepts for this project:

- **Workflows**: Deterministic functions that orchestrate activities; state survives process crashes
- **Activities**: Non-deterministic operations (API calls, database writes); automatically retried on failure
- **Signals**: External events sent to running workflows (e.g., payment method updated)
- **Queries**: Read-only access to workflow state without affecting execution
- **Timers**: Durable sleep that persists through worker restarts (critical for retry scheduling)

### 4.2 PaymentWorkflow - Main Orchestrator

This is the primary workflow handling payment processing. It manages the complete lifecycle from validation through completion or failure:

```go
package workflow

import (
    "fmt"
    "time"

    "go.temporal.io/sdk/temporal"
    "go.temporal.io/sdk/workflow"
    
    "payment-service/internal/activity"
)

const (
    SignalUpdatePaymentMethod = "update-payment-method"
    SignalCancelPayment       = "cancel-payment"
)

type PaymentState struct {
    PaymentID     string
    Status        PaymentStatus
    AttemptCount  int
    Attempts      []AttemptRecord
    NextRetryAt   time.Time
    TransactionID string
    FailureReason string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type AttemptRecord struct {
    AttemptNumber int
    Timestamp     time.Time
    Success       bool
    DeclineCode   string
    ErrorMessage  string
}

func PaymentWorkflow(ctx workflow.Context, req PaymentRequest) (*PaymentResult, error) {
    logger := workflow.GetLogger(ctx)
    
    // Initialize workflow state
    state := &PaymentState{
        PaymentID: req.PaymentID,
        Status:    StatusPending,
        Attempts:  []AttemptRecord{},
        CreatedAt: workflow.Now(ctx),
    }
    
    // Register query handlers for external visibility
    _ = workflow.SetQueryHandler(ctx, "get-status", func() (*PaymentState, error) {
        return state, nil
    })
    
    _ = workflow.SetQueryHandler(ctx, "get-attempts", func() ([]AttemptRecord, error) {
        return state.Attempts, nil
    })
    
    // Signal channels for external events
    updateMethodCh := workflow.GetSignalChannel(ctx, SignalUpdatePaymentMethod)
    cancelCh := workflow.GetSignalChannel(ctx, SignalCancelPayment)
    
    // Activity options with retry policy for transient failures
    actOpts := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    time.Second,
            BackoffCoefficient: 2.0,
            MaximumAttempts:    3,
            NonRetryableErrorTypes: []string{
                "HardDeclineError",
                "FraudError",
                "ValidationError",
            },
        },
    }
    ctx = workflow.WithActivityOptions(ctx, actOpts)
    
    // Step 1: Validate payment
    state.Status = StatusProcessing
    var validationResult ValidationResult
    err := workflow.ExecuteActivity(ctx, activities.ValidatePayment, req).Get(ctx, &validationResult)
    if err != nil {
        state.Status = StatusFailed
        state.FailureReason = "validation_failed"
        return &PaymentResult{Success: false, Error: err.Error()}, nil
    }
    
    // Step 2: Execute payment with recovery loop
    maxAttempts := 6
    for attempt := 0; attempt < maxAttempts; attempt++ {
        state.AttemptCount = attempt + 1
        state.UpdatedAt = workflow.Now(ctx)
        
        // Check for cancellation signal (non-blocking)
        if cancelled := checkCancellation(ctx, cancelCh); cancelled {
            state.Status = StatusCancelled
            return &PaymentResult{Success: false, Reason: "cancelled"}, nil
        }
        
        // Check for payment method update signal (non-blocking)
        checkPaymentMethodUpdate(ctx, updateMethodCh, &req)
        
        // Execute payment based on type
        var chargeResult ChargeResult
        switch req.PaymentType {
        case PaymentTypeCard:
            err = workflow.ExecuteActivity(ctx, activities.ProcessCardPayment, req).Get(ctx, &chargeResult)
        case PaymentTypeACH:
            err = workflow.ExecuteActivity(ctx, activities.ProcessACHPayment, req).Get(ctx, &chargeResult)
        case PaymentTypeInternal:
            err = workflow.ExecuteActivity(ctx, activities.ProcessInternalTransfer, req).Get(ctx, &chargeResult)
        }
        
        // Record attempt
        state.Attempts = append(state.Attempts, AttemptRecord{
            AttemptNumber: attempt + 1,
            Timestamp:     workflow.Now(ctx),
            Success:       err == nil && chargeResult.Success,
            DeclineCode:   chargeResult.DeclineCode,
            ErrorMessage:  getErrorMessage(err),
        })
        
        // Success path
        if err == nil && chargeResult.Success {
            // Update ledger atomically
            err = workflow.ExecuteActivity(ctx, activities.UpdateLedger, LedgerRequest{
                PaymentID:       req.PaymentID,
                SourceAccountID: req.SourceAccountID,
                DestAccountID:   req.DestAccountID,
                Amount:          req.Amount,
                TransactionID:   chargeResult.TransactionID,
            }).Get(ctx, nil)
            
            if err != nil {
                logger.Error("Ledger update failed", "error", err)
                // In production: trigger compensation/refund workflow
            }
            
            state.Status = StatusSucceeded
            state.TransactionID = chargeResult.TransactionID
            
            // Publish success event
            _ = workflow.ExecuteActivity(ctx, activities.PublishEvent, PaymentEvent{
                Type:      "payment.succeeded",
                PaymentID: req.PaymentID,
                Timestamp: workflow.Now(ctx),
            }).Get(ctx, nil)
            
            return &PaymentResult{
                Success:       true,
                TransactionID: chargeResult.TransactionID,
            }, nil
        }
        
        // Failure path - classify and decide retry
        declineType := classifyDecline(chargeResult.DeclineCode)
        
        if declineType == DeclineTypeHard || declineType == DeclineTypeFraud {
            state.Status = StatusFailed
            state.FailureReason = chargeResult.DeclineCode
            
            _ = workflow.ExecuteActivity(ctx, activities.PublishEvent, PaymentEvent{
                Type:        "payment.failed",
                PaymentID:   req.PaymentID,
                DeclineCode: chargeResult.DeclineCode,
                Timestamp:   workflow.Now(ctx),
            }).Get(ctx, nil)
            
            return &PaymentResult{
                Success: false,
                Reason:  chargeResult.DeclineCode,
            }, nil
        }
        
        // Soft decline - calculate retry delay and wait
        if attempt < maxAttempts-1 {
            state.Status = StatusRecovering
            retryDelay := calculateRetryDelay(chargeResult.DeclineCode, attempt, req)
            state.NextRetryAt = workflow.Now(ctx).Add(retryDelay)
            
            logger.Info("Scheduling retry",
                "attempt", attempt+1,
                "delay", retryDelay,
                "decline_code", chargeResult.DeclineCode)
            
            // Wait with signal handling
            if err := waitWithSignals(ctx, retryDelay, updateMethodCh, cancelCh, &req, state); err != nil {
                return &PaymentResult{Success: false, Reason: "cancelled"}, nil
            }
        }
    }
    
    // Exhausted all retries
    state.Status = StatusFailed
    state.FailureReason = "exhausted_retries"
    
    _ = workflow.ExecuteActivity(ctx, activities.PublishEvent, PaymentEvent{
        Type:      "payment.exhausted",
        PaymentID: req.PaymentID,
        Attempts:  state.AttemptCount,
        Timestamp: workflow.Now(ctx),
    }).Get(ctx, nil)
    
    return &PaymentResult{Success: false, Reason: "exhausted_retries"}, nil
}

// Helper: Check for cancellation signal without blocking
func checkCancellation(ctx workflow.Context, cancelCh workflow.ReceiveChannel) bool {
    selector := workflow.NewSelector(ctx)
    cancelled := false
    
    selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
        var reason string
        c.Receive(ctx, &reason)
        cancelled = true
    })
    
    // Non-blocking check
    selector.AddDefault(func() {})
    selector.Select(ctx)
    
    return cancelled
}

// Helper: Check for payment method update without blocking
func checkPaymentMethodUpdate(ctx workflow.Context, ch workflow.ReceiveChannel, req *PaymentRequest) {
    selector := workflow.NewSelector(ctx)
    
    selector.AddReceive(ch, func(c workflow.ReceiveChannel, _ bool) {
        var newMethod PaymentMethod
        c.Receive(ctx, &newMethod)
        req.PaymentMethod = &newMethod
        req.PaymentMethodID = newMethod.ID
    })
    
    selector.AddDefault(func() {})
    selector.Select(ctx)
}

// Helper: Wait for duration while handling signals
func waitWithSignals(
    ctx workflow.Context,
    duration time.Duration,
    updateCh, cancelCh workflow.ReceiveChannel,
    req *PaymentRequest,
    state *PaymentState,
) error {
    timer := workflow.NewTimer(ctx, duration)
    selector := workflow.NewSelector(ctx)
    
    selector.AddFuture(timer, func(f workflow.Future) {
        // Timer fired, continue to next retry
    })
    
    selector.AddReceive(updateCh, func(c workflow.ReceiveChannel, _ bool) {
        var newMethod PaymentMethod
        c.Receive(ctx, &newMethod)
        req.PaymentMethod = &newMethod
        req.PaymentMethodID = newMethod.ID
        // Payment method updated - will retry immediately on next loop
    })
    
    selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
        state.Status = StatusCancelled
    })
    
    selector.Select(ctx)
    
    if state.Status == StatusCancelled {
        return fmt.Errorf("cancelled")
    }
    return nil
}

func getErrorMessage(err error) string {
    if err == nil {
        return ""
    }
    return err.Error()
}
```

### 4.3 ScheduledPaymentWorkflow

Handles future-dated payments such as scheduled bill pays or recurring loan payments:

```go
func ScheduledPaymentWorkflow(ctx workflow.Context, schedule ScheduledPayment) error {
    logger := workflow.GetLogger(ctx)
    
    state := &ScheduledState{
        ScheduleID: schedule.ID,
        Status:     "waiting",
        Payments:   []string{},
    }
    
    _ = workflow.SetQueryHandler(ctx, "get-schedule-status", func() (*ScheduledState, error) {
        return state, nil
    })
    
    cancelCh := workflow.GetSignalChannel(ctx, "cancel-schedule")
    
    // For recurring payments, loop until end date or cancellation
    for {
        nextPaymentTime := calculateNextPaymentTime(schedule, workflow.Now(ctx))
        
        if schedule.EndDate != nil && nextPaymentTime.After(*schedule.EndDate) {
            state.Status = "completed"
            return nil
        }
        
        waitDuration := nextPaymentTime.Sub(workflow.Now(ctx))
        
        // Wait until scheduled time, checking for cancellation
        selector := workflow.NewSelector(ctx)
        timerFired := false
        
        selector.AddFuture(workflow.NewTimer(ctx, waitDuration), func(f workflow.Future) {
            timerFired = true
        })
        
        selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
            var reason string
            c.Receive(ctx, &reason)
            state.Status = "cancelled"
            state.CancelReason = reason
        })
        
        selector.Select(ctx)
        
        if state.Status == "cancelled" {
            return nil
        }
        
        if !timerFired {
            continue
        }
        
        // Check account is still active before processing
        actOpts := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second}
        ctx = workflow.WithActivityOptions(ctx, actOpts)
        
        var accountActive bool
        err := workflow.ExecuteActivity(ctx, activities.CheckAccountStatus, schedule.SourceAccountID).Get(ctx, &accountActive)
        if err != nil || !accountActive {
            logger.Warn("Account no longer active, skipping payment")
            continue
        }
        
        // Start child workflow for this payment
        paymentReq := PaymentRequest{
            PaymentID:       fmt.Sprintf("%s-%d", schedule.ID, len(state.Payments)),
            SourceAccountID: schedule.SourceAccountID,
            DestAccountID:   schedule.DestAccountID,
            Amount:          schedule.Amount,
            PaymentType:     schedule.PaymentType,
        }
        
        childOpts := workflow.ChildWorkflowOptions{
            WorkflowID: fmt.Sprintf("payment-%s", paymentReq.PaymentID),
        }
        ctx = workflow.WithChildOptions(ctx, childOpts)
        
        var result PaymentResult
        err = workflow.ExecuteChildWorkflow(ctx, PaymentWorkflow, paymentReq).Get(ctx, &result)
        
        state.Payments = append(state.Payments, paymentReq.PaymentID)
        state.LastPaymentTime = workflow.Now(ctx)
        
        // For one-time scheduled payments, exit after processing
        if schedule.Frequency == FrequencyOnce {
            state.Status = "completed"
            return nil
        }
    }
}

type ScheduledPayment struct {
    ID              string
    SourceAccountID string
    DestAccountID   string
    Amount          decimal.Decimal
    Currency        string
    PaymentType     PaymentType
    Frequency       Frequency       // ONCE, WEEKLY, BIWEEKLY, MONTHLY
    StartDate       time.Time
    EndDate         *time.Time
    DayOfMonth      int             // For monthly: which day (1-28)
    DayOfWeek       time.Weekday    // For weekly
}

type Frequency string

const (
    FrequencyOnce     Frequency = "ONCE"
    FrequencyWeekly   Frequency = "WEEKLY"
    FrequencyBiweekly Frequency = "BIWEEKLY"
    FrequencyMonthly  Frequency = "MONTHLY"
)
```

### 4.4 Retry Timing Strategy

The retry scheduler implements intelligent timing similar to Butter Payments. For a learning project, we use rule-based logic that can later be enhanced with ML:

```go
func calculateRetryDelay(declineCode string, attempt int, req PaymentRequest) time.Duration {
    // Rule 1: Insufficient funds - align with typical paydays
    if declineCode == "insufficient_funds" {
        return alignToNextPayday(req.CustomerTimezone)
    }
    
    // Rule 2: Generic decline - try different times of day
    if declineCode == "generic_decline" || declineCode == "do_not_honor" {
        // Banks may have different approval patterns at different times
        baseDelay := []time.Duration{
            4 * time.Hour,       // Try again in 4 hours
            12 * time.Hour,      // Try next half-day
            24 * time.Hour,      // Try tomorrow
            48 * time.Hour,      // Try in 2 days
            7 * 24 * time.Hour,  // Try next week
        }
        if attempt < len(baseDelay) {
            return baseDelay[attempt]
        }
    }
    
    // Rule 3: Rate limited - short backoff
    if declineCode == "rate_limit" || declineCode == "try_again_later" {
        return time.Duration(1<<attempt) * time.Minute // 1, 2, 4, 8 minutes
    }
    
    // Rule 4: Debit cards - retry on common pay dates (1st and 15th)
    if req.PaymentMethod != nil && req.PaymentMethod.Type == "debit" {
        return alignToPayDate(req.CustomerTimezone)
    }
    
    // Default: exponential backoff with cap
    delay := time.Duration(1<<attempt) * time.Hour
    maxDelay := 7 * 24 * time.Hour
    if delay > maxDelay {
        delay = maxDelay
    }
    return delay
}

func alignToNextPayday(tz string) time.Duration {
    loc, _ := time.LoadLocation(tz)
    if loc == nil {
        loc = time.UTC
    }
    now := time.Now().In(loc)
    
    // Find next 1st or 15th at 6 AM (after direct deposits clear)
    day := now.Day()
    var nextPayday time.Time
    
    if day < 15 {
        nextPayday = time.Date(now.Year(), now.Month(), 15, 6, 0, 0, 0, loc)
    } else {
        // Move to 1st of next month
        nextMonth := now.AddDate(0, 1, 0)
        nextPayday = time.Date(nextMonth.Year(), nextMonth.Month(), 1, 6, 0, 0, 0, loc)
    }
    
    // Skip weekends - if 1st/15th falls on weekend, use next Monday
    for nextPayday.Weekday() == time.Saturday || nextPayday.Weekday() == time.Sunday {
        nextPayday = nextPayday.AddDate(0, 0, 1)
    }
    
    delay := nextPayday.Sub(now)
    if delay < time.Hour {
        delay = time.Hour // Minimum 1 hour delay
    }
    return delay
}

func alignToPayDate(tz string) time.Duration {
    loc, _ := time.LoadLocation(tz)
    if loc == nil {
        loc = time.UTC
    }
    now := time.Now().In(loc)
    
    // For debit cards, align to next Monday (common post-weekend deposit day)
    daysUntilMonday := (8 - int(now.Weekday())) % 7
    if daysUntilMonday == 0 {
        daysUntilMonday = 7
    }
    
    nextMonday := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 6, 0, 0, 0, loc)
    return nextMonday.Sub(now)
}
```

---

## 5. Activities Implementation

Activities handle all non-deterministic operations. Each activity should be idempotent where possible to handle retries safely.

### 5.1 ProcessCardPayment Activity

```go
package activities

import (
    "context"
    "errors"
    "fmt"

    "github.com/shopspring/decimal"
    "github.com/stripe/stripe-go/v76"
    "go.temporal.io/sdk/activity"
    "go.temporal.io/sdk/temporal"
)

type PaymentActivities struct {
    stripeClient  *stripe.Client
    db            *sql.DB
    eventBus      EventPublisher
}

func NewPaymentActivities(stripeClient *stripe.Client, db *sql.DB, eventBus EventPublisher) *PaymentActivities {
    return &PaymentActivities{
        stripeClient: stripeClient,
        db:           db,
        eventBus:     eventBus,
    }
}

func (a *PaymentActivities) ProcessCardPayment(ctx context.Context, req PaymentRequest) (*ChargeResult, error) {
    info := activity.GetInfo(ctx)
    
    // Create idempotency key from workflow execution + payment ID + attempt
    // This prevents duplicate charges if the activity is retried
    idempotencyKey := fmt.Sprintf("%s-%s-%d",
        info.WorkflowExecution.ID,
        req.PaymentID,
        info.Attempt,
    )
    
    // Create charge via Stripe
    params := &stripe.ChargeParams{
        Amount:         stripe.Int64(req.Amount.Mul(decimal.NewFromInt(100)).IntPart()),
        Currency:       stripe.String(req.Currency),
        Customer:       stripe.String(req.CustomerID),
        PaymentMethod:  stripe.String(req.PaymentMethodID),
        Capture:        stripe.Bool(true),
        IdempotencyKey: stripe.String(idempotencyKey),
    }
    
    // Add metadata for reconciliation
    params.AddMetadata("payment_id", req.PaymentID)
    params.AddMetadata("source_account", req.SourceAccountID)
    
    charge, err := a.stripeClient.Charges.Create(params)
    if err != nil {
        return mapStripeError(err)
    }
    
    return &ChargeResult{
        Success:       charge.Status == "succeeded",
        TransactionID: charge.ID,
        DeclineCode:   string(charge.FailureCode),
    }, nil
}

func mapStripeError(err error) (*ChargeResult, error) {
    var stripeErr *stripe.Error
    if errors.As(err, &stripeErr) {
        result := &ChargeResult{
            Success:     false,
            DeclineCode: string(stripeErr.DeclineCode),
            Message:     stripeErr.Message,
        }
        
        // Map to appropriate error type for retry policy
        switch stripeErr.DeclineCode {
        case stripe.DeclineCodeInsufficientFunds,
             stripe.DeclineCodeGenericDecline,
             stripe.DeclineCodeProcessingError:
            // Soft decline - return result without error to let workflow decide
            return result, nil
            
        case stripe.DeclineCodeStolenCard,
             stripe.DeclineCodeLostCard,
             stripe.DeclineCodeExpiredCard:
            // Hard decline - return non-retryable error
            return result, temporal.NewApplicationError(
                stripeErr.Message, "HardDeclineError",
            )
            
        case stripe.DeclineCodeFraudulent:
            return result, temporal.NewApplicationError(
                stripeErr.Message, "FraudError",
            )
        }
    }
    
    // Unknown error - let Temporal retry
    return nil, err
}
```

### 5.2 ProcessACHPayment Activity

```go
func (a *PaymentActivities) ProcessACHPayment(ctx context.Context, req PaymentRequest) (*ChargeResult, error) {
    info := activity.GetInfo(ctx)
    
    idempotencyKey := fmt.Sprintf("ach-%s-%s-%d",
        info.WorkflowExecution.ID,
        req.PaymentID,
        info.Attempt,
    )
    
    // ACH payments work differently - they're initiated and then settle later
    // For this learning project, we simulate ACH with Stripe's ACH support
    
    params := &stripe.PaymentIntentParams{
        Amount:   stripe.Int64(req.Amount.Mul(decimal.NewFromInt(100)).IntPart()),
        Currency: stripe.String(req.Currency),
        Customer: stripe.String(req.CustomerID),
        PaymentMethodTypes: stripe.StringSlice([]string{
            "us_bank_account",
        }),
        PaymentMethod: stripe.String(req.PaymentMethodID),
        Confirm:       stripe.Bool(true),
        MandateData: &stripe.PaymentIntentMandateDataParams{
            CustomerAcceptance: &stripe.PaymentIntentMandateDataCustomerAcceptanceParams{
                Type: stripe.String("online"),
                Online: &stripe.PaymentIntentMandateDataCustomerAcceptanceOnlineParams{
                    IPAddress: stripe.String(req.CustomerIP),
                    UserAgent: stripe.String(req.UserAgent),
                },
            },
        },
    }
    params.SetIdempotencyKey(idempotencyKey)
    
    pi, err := a.stripeClient.PaymentIntents.Create(params)
    if err != nil {
        return mapStripeError(err)
    }
    
    // ACH payments may be in "processing" state initially
    success := pi.Status == stripe.PaymentIntentStatusSucceeded ||
               pi.Status == stripe.PaymentIntentStatusProcessing
    
    return &ChargeResult{
        Success:       success,
        TransactionID: pi.ID,
        Status:        string(pi.Status),
    }, nil
}
```

### 5.3 UpdateLedger Activity

Double-entry bookkeeping ensures every debit has a matching credit:

```go
func (a *PaymentActivities) UpdateLedger(ctx context.Context, req LedgerRequest) error {
    // Use transaction for atomicity
    tx, err := a.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Check for idempotency - has this payment already been recorded?
    var exists bool
    err = tx.QueryRowContext(ctx,
        "SELECT EXISTS(SELECT 1 FROM ledger_entries WHERE payment_id = $1)",
        req.PaymentID,
    ).Scan(&exists)
    if err != nil {
        return err
    }
    if exists {
        // Already processed - idempotent success
        return nil
    }
    
    // Get current balances with row locks
    var sourceBalance, destBalance decimal.Decimal
    
    err = tx.QueryRowContext(ctx,
        "SELECT balance FROM accounts WHERE id = $1 FOR UPDATE",
        req.SourceAccountID,
    ).Scan(&sourceBalance)
    if err != nil {
        return fmt.Errorf("source account not found: %w", err)
    }
    
    err = tx.QueryRowContext(ctx,
        "SELECT balance FROM accounts WHERE id = $1 FOR UPDATE",
        req.DestAccountID,
    ).Scan(&destBalance)
    if err != nil {
        return fmt.Errorf("dest account not found: %w", err)
    }
    
    // Verify sufficient funds
    if sourceBalance.LessThan(req.Amount) {
        return temporal.NewApplicationError(
            "insufficient funds", "ValidationError",
        )
    }
    
    now := time.Now()
    newSourceBalance := sourceBalance.Sub(req.Amount)
    newDestBalance := destBalance.Add(req.Amount)
    
    // Insert debit entry (money leaving source account)
    _, err = tx.ExecContext(ctx, `
        INSERT INTO ledger_entries 
        (id, account_id, payment_id, type, amount, balance_after, description, created_at)
        VALUES ($1, $2, $3, 'DEBIT', $4, $5, $6, $7)`,
        uuid.New().String(),
        req.SourceAccountID,
        req.PaymentID,
        req.Amount,
        newSourceBalance,
        fmt.Sprintf("Payment to %s", req.DestAccountID),
        now,
    )
    if err != nil {
        return err
    }
    
    // Insert credit entry (money entering destination account)
    _, err = tx.ExecContext(ctx, `
        INSERT INTO ledger_entries 
        (id, account_id, payment_id, type, amount, balance_after, description, created_at)
        VALUES ($1, $2, $3, 'CREDIT', $4, $5, $6, $7)`,
        uuid.New().String(),
        req.DestAccountID,
        req.PaymentID,
        req.Amount,
        newDestBalance,
        fmt.Sprintf("Payment from %s", req.SourceAccountID),
        now,
    )
    if err != nil {
        return err
    }
    
    // Update account balances
    _, err = tx.ExecContext(ctx,
        "UPDATE accounts SET balance = $1, updated_at = $2 WHERE id = $3",
        newSourceBalance, now, req.SourceAccountID,
    )
    if err != nil {
        return err
    }
    
    _, err = tx.ExecContext(ctx,
        "UPDATE accounts SET balance = $1, updated_at = $2 WHERE id = $3",
        newDestBalance, now, req.DestAccountID,
    )
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### 5.4 PublishEvent Activity

```go
func (a *PaymentActivities) PublishEvent(ctx context.Context, event PaymentEvent) error {
    event.ID = uuid.New().String()
    event.Source = "payment-service"
    
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    
    // Publish to NATS subject based on event type
    // e.g., "payments.payment_succeeded" for "payment.succeeded"
    subject := fmt.Sprintf("payments.%s", strings.ReplaceAll(event.Type, ".", "_"))
    
    return a.natsConn.Publish(subject, data)
}
```

### 5.5 CheckAccountStatus Activity

```go
func (a *PaymentActivities) CheckAccountStatus(ctx context.Context, accountID string) (bool, error) {
    var status string
    err := a.db.QueryRowContext(ctx,
        "SELECT status FROM accounts WHERE id = $1",
        accountID,
    ).Scan(&status)
    
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return false, nil
        }
        return false, err
    }
    
    return status == "ACTIVE", nil
}
```

### 5.6 ValidatePayment Activity

```go
func (a *PaymentActivities) ValidatePayment(ctx context.Context, req PaymentRequest) (*ValidationResult, error) {
    result := &ValidationResult{Valid: true, Errors: []string{}}
    
    // Check source account exists and is active
    var sourceStatus string
    var sourceBalance, dailyLimit decimal.Decimal
    err := a.db.QueryRowContext(ctx,
        "SELECT status, balance, daily_limit FROM accounts WHERE id = $1",
        req.SourceAccountID,
    ).Scan(&sourceStatus, &sourceBalance, &dailyLimit)
    
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            result.Valid = false
            result.Errors = append(result.Errors, "source account not found")
            return result, nil
        }
        return nil, err
    }
    
    if sourceStatus != "ACTIVE" {
        result.Valid = false
        result.Errors = append(result.Errors, "source account is not active")
    }
    
    // Check sufficient funds
    if sourceBalance.LessThan(req.Amount) {
        result.Valid = false
        result.Errors = append(result.Errors, "insufficient funds")
    }
    
    // Check daily limit
    if dailyLimit.GreaterThan(decimal.Zero) {
        todayTotal, err := a.getTodayTransactionTotal(ctx, req.SourceAccountID)
        if err != nil {
            return nil, err
        }
        if todayTotal.Add(req.Amount).GreaterThan(dailyLimit) {
            result.Valid = false
            result.Errors = append(result.Errors, "daily limit exceeded")
        }
    }
    
    // Check amount is positive
    if req.Amount.LessThanOrEqual(decimal.Zero) {
        result.Valid = false
        result.Errors = append(result.Errors, "amount must be positive")
    }
    
    // Check destination account if provided
    if req.DestAccountID != "" {
        var destStatus string
        err := a.db.QueryRowContext(ctx,
            "SELECT status FROM accounts WHERE id = $1",
            req.DestAccountID,
        ).Scan(&destStatus)
        if err != nil {
            result.Valid = false
            result.Errors = append(result.Errors, "destination account not found")
        } else if destStatus != "ACTIVE" {
            result.Valid = false
            result.Errors = append(result.Errors, "destination account is not active")
        }
    }
    
    return result, nil
}

func (a *PaymentActivities) getTodayTransactionTotal(ctx context.Context, accountID string) (decimal.Decimal, error) {
    var total decimal.Decimal
    err := a.db.QueryRowContext(ctx, `
        SELECT COALESCE(SUM(amount), 0)
        FROM ledger_entries
        WHERE account_id = $1
          AND type = 'DEBIT'
          AND created_at >= CURRENT_DATE
    `, accountID).Scan(&total)
    return total, err
}
```

---

## 6. API Design

### 6.1 REST API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/payments` | Submit a new payment for processing |
| `GET` | `/api/v1/payments/:id` | Get payment status and details |
| `GET` | `/api/v1/payments/:id/attempts` | Get list of payment attempts |
| `POST` | `/api/v1/payments/:id/cancel` | Cancel a pending/recovering payment |
| `PUT` | `/api/v1/payments/:id/method` | Update payment method (triggers immediate retry) |
| `POST` | `/api/v1/scheduled-payments` | Create a scheduled/recurring payment |
| `GET` | `/api/v1/scheduled-payments/:id` | Get scheduled payment status |
| `DELETE` | `/api/v1/scheduled-payments/:id` | Cancel a scheduled payment |
| `GET` | `/api/v1/accounts/:id/balance` | Get account balance |
| `GET` | `/api/v1/accounts/:id/ledger` | Get ledger entries for account |
| `POST` | `/api/v1/webhooks/stripe` | Receive Stripe webhooks |

### 6.2 Payment Request Schema

```json
// POST /api/v1/payments
{
  "idempotency_key": "pay_abc123",           // Required: Client-provided
  "type": "CARD",                            // CARD, ACH, INTERNAL
  "source_account_id": "acc_12345",
  "dest_account_id": "acc_67890",            // Optional for card payments
  "amount": "150.00",
  "currency": "USD",
  "payment_method": {
    "type": "card",
    "token": "pm_card_visa"                  // Stripe payment method ID
  },
  "scheduled_for": "2026-02-01T00:00:00Z",   // Optional: future date
  "metadata": {
    "invoice_id": "inv_999",
    "description": "Monthly subscription"
  }
}
```

### 6.3 Payment Response Schema

```json
// Response: 202 Accepted (async processing)
{
  "id": "pay_abc123",
  "status": "PROCESSING",
  "workflow_id": "payment-pay_abc123",       // For Temporal UI
  "created_at": "2026-01-16T10:30:00Z",
  "links": {
    "self": "/api/v1/payments/pay_abc123",
    "attempts": "/api/v1/payments/pay_abc123/attempts"
  }
}
```

### 6.4 Payment Status Response

```json
// GET /api/v1/payments/pay_abc123
{
  "id": "pay_abc123",
  "status": "RECOVERING",
  "type": "CARD",
  "amount": "150.00",
  "currency": "USD",
  "source_account_id": "acc_12345",
  "attempt_count": 2,
  "last_decline_code": "insufficient_funds",
  "next_retry_at": "2026-01-17T06:00:00Z",
  "created_at": "2026-01-16T10:30:00Z",
  "updated_at": "2026-01-16T14:30:00Z"
}
```

### 6.5 API Handler Implementation

```go
package handler

import (
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/labstack/echo/v4"
    "go.temporal.io/api/enums/v1"
    "go.temporal.io/sdk/client"
    
    "payment-service/internal/workflow"
)

type PaymentHandler struct {
    temporalClient client.Client
    taskQueue      string
}

func NewPaymentHandler(temporalClient client.Client, taskQueue string) *PaymentHandler {
    return &PaymentHandler{
        temporalClient: temporalClient,
        taskQueue:      taskQueue,
    }
}

func (h *PaymentHandler) CreatePayment(c echo.Context) error {
    var req CreatePaymentRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }
    
    if err := req.Validate(); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }
    
    // Use idempotency key as workflow ID - prevents duplicate workflows
    workflowID := fmt.Sprintf("payment-%s", req.IdempotencyKey)
    
    workflowOpts := client.StartWorkflowOptions{
        ID:                       workflowID,
        TaskQueue:                h.taskQueue,
        WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
        WorkflowExecutionTimeout: 30 * 24 * time.Hour, // 30 day max dunning window
    }
    
    paymentReq := workflow.PaymentRequest{
        PaymentID:       req.IdempotencyKey,
        SourceAccountID: req.SourceAccountID,
        DestAccountID:   req.DestAccountID,
        Amount:          req.Amount,
        Currency:        req.Currency,
        PaymentType:     mapPaymentType(req.Type),
        PaymentMethodID: req.PaymentMethod.Token,
        CustomerID:      req.CustomerID,
    }
    
    // Start workflow (non-blocking)
    run, err := h.temporalClient.ExecuteWorkflow(
        c.Request().Context(),
        workflowOpts,
        workflow.PaymentWorkflow,
        paymentReq,
    )
    
    if err != nil {
        // Check if workflow already exists (idempotent)
        if strings.Contains(err.Error(), "already started") {
            // Return existing workflow status
            return h.getPaymentByID(c, req.IdempotencyKey)
        }
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    
    return c.JSON(http.StatusAccepted, CreatePaymentResponse{
        ID:         req.IdempotencyKey,
        Status:     "PROCESSING",
        WorkflowID: run.GetID(),
        CreatedAt:  time.Now(),
        Links: Links{
            Self:     fmt.Sprintf("/api/v1/payments/%s", req.IdempotencyKey),
            Attempts: fmt.Sprintf("/api/v1/payments/%s/attempts", req.IdempotencyKey),
        },
    })
}

func (h *PaymentHandler) GetPayment(c echo.Context) error {
    paymentID := c.Param("id")
    return h.getPaymentByID(c, paymentID)
}

func (h *PaymentHandler) getPaymentByID(c echo.Context, paymentID string) error {
    workflowID := fmt.Sprintf("payment-%s", paymentID)
    
    // Query workflow state
    resp, err := h.temporalClient.QueryWorkflow(
        c.Request().Context(),
        workflowID,
        "", // RunID - empty for latest
        "get-status",
    )
    
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "payment not found")
    }
    
    var state workflow.PaymentState
    if err := resp.Get(&state); err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    
    return c.JSON(http.StatusOK, mapStateToResponse(state))
}

func (h *PaymentHandler) GetPaymentAttempts(c echo.Context) error {
    paymentID := c.Param("id")
    workflowID := fmt.Sprintf("payment-%s", paymentID)
    
    resp, err := h.temporalClient.QueryWorkflow(
        c.Request().Context(),
        workflowID,
        "",
        "get-attempts",
    )
    
    if err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "payment not found")
    }
    
    var attempts []workflow.AttemptRecord
    if err := resp.Get(&attempts); err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    
    return c.JSON(http.StatusOK, AttemptsResponse{
        PaymentID: paymentID,
        Attempts:  attempts,
    })
}

func (h *PaymentHandler) CancelPayment(c echo.Context) error {
    paymentID := c.Param("id")
    workflowID := fmt.Sprintf("payment-%s", paymentID)
    
    // Send cancel signal to workflow
    err := h.temporalClient.SignalWorkflow(
        c.Request().Context(),
        workflowID,
        "",
        workflow.SignalCancelPayment,
        "user_requested",
    )
    
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    
    return c.JSON(http.StatusOK, map[string]string{
        "status": "cancellation_requested",
    })
}

func (h *PaymentHandler) UpdatePaymentMethod(c echo.Context) error {
    paymentID := c.Param("id")
    workflowID := fmt.Sprintf("payment-%s", paymentID)
    
    var req UpdatePaymentMethodRequest
    if err := c.Bind(&req); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }
    
    // Send signal with new payment method
    err := h.temporalClient.SignalWorkflow(
        c.Request().Context(),
        workflowID,
        "",
        workflow.SignalUpdatePaymentMethod,
        workflow.PaymentMethod{
            ID:   req.Token,
            Type: req.Type,
        },
    )
    
    if err != nil {
        return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
    }
    
    return c.JSON(http.StatusOK, map[string]string{
        "status": "payment_method_updated",
    })
}

func mapStateToResponse(state workflow.PaymentState) PaymentStatusResponse {
    return PaymentStatusResponse{
        ID:              state.PaymentID,
        Status:          string(state.Status),
        AttemptCount:    state.AttemptCount,
        LastDeclineCode: state.FailureReason,
        NextRetryAt:     state.NextRetryAt,
        TransactionID:   state.TransactionID,
        CreatedAt:       state.CreatedAt,
        UpdatedAt:       state.UpdatedAt,
    }
}
```

---

## 7. Database Schema

### 7.1 Core Tables

```sql
-- Accounts table
CREATE TABLE accounts (
    id              VARCHAR(50) PRIMARY KEY,
    type            VARCHAR(20) NOT NULL,  -- CHECKING, SAVINGS, LOAN
    owner_id        VARCHAR(50) NOT NULL,
    balance         DECIMAL(19,4) NOT NULL DEFAULT 0,
    available_bal   DECIMAL(19,4) NOT NULL DEFAULT 0,
    currency        VARCHAR(3) NOT NULL DEFAULT 'USD',
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    daily_limit     DECIMAL(19,4),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_owner ON accounts(owner_id);
CREATE INDEX idx_accounts_status ON accounts(status);

-- Payments table (mirrors Temporal workflow state for querying)
CREATE TABLE payments (
    id                  VARCHAR(50) PRIMARY KEY,
    idempotency_key     VARCHAR(100) UNIQUE NOT NULL,
    workflow_id         VARCHAR(100) UNIQUE,
    type                VARCHAR(20) NOT NULL,
    status              VARCHAR(20) NOT NULL,
    source_account_id   VARCHAR(50) NOT NULL REFERENCES accounts(id),
    dest_account_id     VARCHAR(50) REFERENCES accounts(id),
    amount              DECIMAL(19,4) NOT NULL,
    currency            VARCHAR(3) NOT NULL,
    attempt_count       INT NOT NULL DEFAULT 0,
    last_decline_code   VARCHAR(50),
    transaction_id      VARCHAR(100),
    scheduled_for       TIMESTAMPTZ,
    processed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_source ON payments(source_account_id);
CREATE INDEX idx_payments_dest ON payments(dest_account_id);
CREATE INDEX idx_payments_scheduled ON payments(scheduled_for) 
    WHERE scheduled_for IS NOT NULL;
CREATE INDEX idx_payments_created ON payments(created_at DESC);

-- Ledger entries (immutable audit trail)
CREATE TABLE ledger_entries (
    id              VARCHAR(50) PRIMARY KEY,
    account_id      VARCHAR(50) NOT NULL REFERENCES accounts(id),
    payment_id      VARCHAR(50) NOT NULL REFERENCES payments(id),
    type            VARCHAR(10) NOT NULL,  -- DEBIT, CREDIT
    amount          DECIMAL(19,4) NOT NULL,
    balance_after   DECIMAL(19,4) NOT NULL,
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_account ON ledger_entries(account_id, created_at DESC);
CREATE INDEX idx_ledger_payment ON ledger_entries(payment_id);

-- Ensure ledger entries are never updated or deleted
CREATE RULE ledger_no_update AS ON UPDATE TO ledger_entries DO INSTEAD NOTHING;
CREATE RULE ledger_no_delete AS ON DELETE TO ledger_entries DO INSTEAD NOTHING;

-- Audit log for compliance
CREATE TABLE audit_log (
    id              BIGSERIAL PRIMARY KEY,
    entity_type     VARCHAR(50) NOT NULL,
    entity_id       VARCHAR(50) NOT NULL,
    action          VARCHAR(50) NOT NULL,
    actor_id        VARCHAR(50),
    actor_type      VARCHAR(20),  -- USER, SYSTEM, WORKFLOW
    changes         JSONB,
    metadata        JSONB,
    ip_address      INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit_log(actor_id);
CREATE INDEX idx_audit_time ON audit_log(created_at DESC);
CREATE INDEX idx_audit_action ON audit_log(action);

-- Scheduled payments
CREATE TABLE scheduled_payments (
    id                  VARCHAR(50) PRIMARY KEY,
    workflow_id         VARCHAR(100) UNIQUE,
    source_account_id   VARCHAR(50) NOT NULL REFERENCES accounts(id),
    dest_account_id     VARCHAR(50) REFERENCES accounts(id),
    amount              DECIMAL(19,4) NOT NULL,
    currency            VARCHAR(3) NOT NULL DEFAULT 'USD',
    payment_type        VARCHAR(20) NOT NULL,
    frequency           VARCHAR(20) NOT NULL,  -- ONCE, WEEKLY, BIWEEKLY, MONTHLY
    start_date          DATE NOT NULL,
    end_date            DATE,
    day_of_month        INT,  -- For monthly (1-28)
    day_of_week         INT,  -- For weekly (0-6)
    status              VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    last_payment_at     TIMESTAMPTZ,
    next_payment_at     TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scheduled_status ON scheduled_payments(status);
CREATE INDEX idx_scheduled_next ON scheduled_payments(next_payment_at) 
    WHERE status = 'ACTIVE';
```

### 7.2 Audit Trigger

```sql
-- Automatic audit logging for payments
CREATE OR REPLACE FUNCTION audit_payment_changes()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_log (entity_type, entity_id, action, changes, metadata)
    VALUES (
        'payment',
        COALESCE(NEW.id, OLD.id),
        TG_OP,
        jsonb_build_object(
            'old', CASE WHEN TG_OP = 'INSERT' THEN NULL ELSE row_to_json(OLD) END,
            'new', CASE WHEN TG_OP = 'DELETE' THEN NULL ELSE row_to_json(NEW) END
        ),
        jsonb_build_object('trigger', 'auto')
    );
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER payment_audit_trigger
AFTER INSERT OR UPDATE OR DELETE ON payments
FOR EACH ROW EXECUTE FUNCTION audit_payment_changes();
```

---

## 8. Event System

### 8.1 Event Types

The system publishes domain events for downstream consumers. This enables analytics, notifications, and integration with other services:

| Event Type | Trigger |
|------------|---------|
| `payment.created` | New payment workflow started |
| `payment.processing` | Payment attempt in progress |
| `payment.succeeded` | Payment completed successfully |
| `payment.failed` | Payment failed (hard decline or exhausted) |
| `payment.recovering` | Soft decline, retry scheduled |
| `payment.cancelled` | Payment cancelled by user/system |
| `payment.exhausted` | All retry attempts exhausted |
| `account.balance_updated` | Account balance changed |
| `schedule.created` | New scheduled payment created |
| `schedule.cancelled` | Scheduled payment cancelled |

### 8.2 Event Schema

```go
type PaymentEvent struct {
    ID          string            `json:"id"`
    Type        string            `json:"type"`
    Source      string            `json:"source"`
    Time        time.Time         `json:"time"`
    PaymentID   string            `json:"payment_id"`
    Data        PaymentEventData  `json:"data"`
}

type PaymentEventData struct {
    Status          string          `json:"status"`
    Amount          decimal.Decimal `json:"amount"`
    Currency        string          `json:"currency"`
    SourceAccountID string          `json:"source_account_id"`
    DestAccountID   string          `json:"dest_account_id,omitempty"`
    TransactionID   string          `json:"transaction_id,omitempty"`
    DeclineCode     string          `json:"decline_code,omitempty"`
    AttemptCount    int             `json:"attempt_count,omitempty"`
    NextRetryAt     *time.Time      `json:"next_retry_at,omitempty"`
}
```

### 8.3 Example Event

```json
{
  "id": "evt_abc123",
  "type": "payment.recovering",
  "source": "payment-service",
  "time": "2026-01-16T14:30:00Z",
  "payment_id": "pay_xyz789",
  "data": {
    "status": "RECOVERING",
    "amount": "150.00",
    "currency": "USD",
    "source_account_id": "acc_12345",
    "decline_code": "insufficient_funds",
    "attempt_count": 2,
    "next_retry_at": "2026-01-17T06:00:00Z"
  }
}
```

### 8.4 Event Consumers

Example consumer for sending notifications:

```go
func (c *NotificationConsumer) HandlePaymentEvent(msg *nats.Msg) {
    var event PaymentEvent
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        log.Error("Failed to unmarshal event", "error", err)
        return
    }
    
    switch event.Type {
    case "payment.succeeded":
        c.sendSuccessNotification(event)
    case "payment.failed":
        c.sendFailureNotification(event)
    case "payment.recovering":
        // Maybe send "we're still trying" after 3rd attempt
        if event.Data.AttemptCount >= 3 {
            c.sendRetryNotification(event)
        }
    }
}
```

---

## 9. Project Structure

```
payment-service/
├── cmd/
│   ├── api/
│   │   └── main.go              # HTTP API server entrypoint
│   ├── worker/
│   │   └── main.go              # Temporal worker entrypoint
│   └── migrate/
│       └── main.go              # Database migration tool
├── internal/
│   ├── api/
│   │   ├── handler/
│   │   │   ├── payment.go       # Payment HTTP handlers
│   │   │   ├── account.go       # Account HTTP handlers
│   │   │   ├── scheduled.go     # Scheduled payment handlers
│   │   │   └── webhook.go       # Webhook handlers (Stripe)
│   │   ├── middleware/
│   │   │   ├── auth.go          # Authentication middleware
│   │   │   ├── logging.go       # Request logging
│   │   │   └── ratelimit.go     # Rate limiting
│   │   └── router.go            # Route definitions
│   ├── workflow/
│   │   ├── payment.go           # PaymentWorkflow
│   │   ├── scheduled.go         # ScheduledPaymentWorkflow
│   │   ├── types.go             # Workflow input/output types
│   │   ├── signals.go           # Signal definitions
│   │   └── retry.go             # Retry timing logic
│   ├── activity/
│   │   ├── payment.go           # Card/ACH payment activities
│   │   ├── ledger.go            # Ledger update activity
│   │   ├── validation.go        # Payment validation
│   │   ├── notification.go      # Email/SMS activities
│   │   └── event.go             # Event publishing activity
│   ├── domain/
│   │   ├── payment.go           # Payment entity and enums
│   │   ├── account.go           # Account entity
│   │   ├── ledger.go            # LedgerEntry entity
│   │   ├── decline.go           # Decline classification
│   │   └── errors.go            # Domain errors
│   ├── repository/
│   │   ├── payment.go           # Payment database operations
│   │   ├── account.go           # Account database operations
│   │   └── ledger.go            # Ledger database operations
│   └── config/
│       └── config.go            # Configuration loading
├── pkg/
│   ├── stripe/
│   │   └── client.go            # Stripe client wrapper
│   ├── temporal/
│   │   └── client.go            # Temporal client wrapper
│   └── nats/
│       └── client.go            # NATS client wrapper
├── migrations/
│   ├── 001_create_accounts.up.sql
│   ├── 001_create_accounts.down.sql
│   ├── 002_create_payments.up.sql
│   ├── 002_create_payments.down.sql
│   ├── 003_create_ledger.up.sql
│   ├── 003_create_ledger.down.sql
│   ├── 004_create_audit.up.sql
│   ├── 004_create_audit.down.sql
│   └── 005_create_scheduled.up.sql
├── docker-compose.yml           # Local dev environment
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## 10. Implementation Roadmap

### Phase 1: Foundation (Week 1-2)

1. Set up project structure with Go modules
2. Configure Docker Compose with Temporal Server, PostgreSQL, NATS
3. Implement database migrations and repository layer
4. Create basic API server with health check endpoint
5. Set up Temporal worker with empty workflow registration

**Deliverables:**
- Running infrastructure (docker-compose up)
- Database schema applied
- Health check endpoint responding
- Worker connecting to Temporal

### Phase 2: Core Workflows (Week 3-4)

1. Implement PaymentWorkflow with basic processing (no retry)
2. Implement ProcessCardPayment activity with mock Stripe
3. Implement ProcessInternalTransfer activity
4. Implement UpdateLedger activity with double-entry bookkeeping
5. Add query handlers for workflow state inspection

**Deliverables:**
- End-to-end payment processing (happy path)
- Ledger entries being created
- Workflow state queryable

### Phase 3: Recovery Logic (Week 5-6)

1. Implement decline classification (soft/hard/fraud)
2. Add retry loop with intelligent timing (Butter-style)
3. Implement signal handlers for payment method updates
4. Implement signal handlers for cancellation
5. Add event publishing for downstream consumers

**Deliverables:**
- Soft declines trigger retry scheduling
- Payment method updates trigger immediate retry
- Events published to NATS

### Phase 4: Scheduled Payments (Week 7-8)

1. Implement ScheduledPaymentWorkflow for future-dated payments
2. Add support for recurring payments (weekly, monthly)
3. Implement child workflow pattern for individual payment execution
4. Add API endpoints for schedule management
5. Add schedule cancellation via signals

**Deliverables:**
- Scheduled payments executing at correct times
- Recurring payments generating individual payment workflows
- Schedule management API working

### Phase 5: Production Readiness (Week 9-10)

1. Integrate real Stripe test mode
2. Add comprehensive error handling and observability
3. Implement rate limiting on API
4. Write integration tests for critical paths
5. Create documentation and README
6. Add Prometheus metrics

**Deliverables:**
- Real Stripe test payments working
- Integration tests passing
- Documentation complete
- Metrics exposed

---

## 11. Testing Strategy

### 11.1 Workflow Unit Testing

Temporal provides a test framework that allows workflow testing without a running server:

```go
package workflow_test

import (
    "testing"
    "time"

    "github.com/shopspring/decimal"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
    "go.temporal.io/sdk/testsuite"
    
    "payment-service/internal/activity"
    "payment-service/internal/workflow"
)

func TestPaymentWorkflow_Success(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()
    
    // Mock activities
    env.OnActivity(activity.ValidatePayment, mock.Anything, mock.Anything).
        Return(&workflow.ValidationResult{Valid: true}, nil)
    
    env.OnActivity(activity.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(&workflow.ChargeResult{
            Success:       true,
            TransactionID: "txn_123",
        }, nil)
    
    env.OnActivity(activity.UpdateLedger, mock.Anything, mock.Anything).
        Return(nil)
    
    env.OnActivity(activity.PublishEvent, mock.Anything, mock.Anything).
        Return(nil)
    
    // Execute workflow
    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID:       "pay_test",
        Amount:          decimal.NewFromFloat(100.00),
        SourceAccountID: "acc_source",
        DestAccountID:   "acc_dest",
        PaymentType:     workflow.PaymentTypeCard,
    })
    
    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())
    
    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.True(t, result.Success)
    assert.Equal(t, "txn_123", result.TransactionID)
}

func TestPaymentWorkflow_SoftDecline_Retry(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()
    
    attemptCount := 0
    
    env.OnActivity(activity.ValidatePayment, mock.Anything, mock.Anything).
        Return(&workflow.ValidationResult{Valid: true}, nil)
    
    env.OnActivity(activity.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(func(ctx context.Context, req workflow.PaymentRequest) (*workflow.ChargeResult, error) {
            attemptCount++
            if attemptCount < 3 {
                return &workflow.ChargeResult{
                    Success:     false,
                    DeclineCode: "insufficient_funds",
                }, nil
            }
            return &workflow.ChargeResult{
                Success:       true,
                TransactionID: "txn_retry_success",
            }, nil
        })
    
    env.OnActivity(activity.UpdateLedger, mock.Anything, mock.Anything).
        Return(nil)
    
    env.OnActivity(activity.PublishEvent, mock.Anything, mock.Anything).
        Return(nil)
    
    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID:       "pay_retry_test",
        Amount:          decimal.NewFromFloat(50.00),
        SourceAccountID: "acc_source",
        PaymentType:     workflow.PaymentTypeCard,
    })
    
    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())
    
    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.True(t, result.Success)
    assert.Equal(t, 3, attemptCount, "Should have taken 3 attempts")
}

func TestPaymentWorkflow_HardDecline_NoRetry(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()
    
    attemptCount := 0
    
    env.OnActivity(activity.ValidatePayment, mock.Anything, mock.Anything).
        Return(&workflow.ValidationResult{Valid: true}, nil)
    
    env.OnActivity(activity.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(func(ctx context.Context, req workflow.PaymentRequest) (*workflow.ChargeResult, error) {
            attemptCount++
            return &workflow.ChargeResult{
                Success:     false,
                DeclineCode: "stolen_card",
            }, temporal.NewApplicationError("Card reported stolen", "HardDeclineError")
        })
    
    env.OnActivity(activity.PublishEvent, mock.Anything, mock.Anything).
        Return(nil)
    
    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID:       "pay_hard_decline",
        Amount:          decimal.NewFromFloat(75.00),
        SourceAccountID: "acc_source",
        PaymentType:     workflow.PaymentTypeCard,
    })
    
    require.True(t, env.IsWorkflowCompleted())
    
    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.False(t, result.Success)
    assert.Equal(t, "stolen_card", result.Reason)
    assert.Equal(t, 1, attemptCount, "Should not retry hard declines")
}
```

### 11.2 Testing Signal Handling

```go
func TestPaymentWorkflow_PaymentMethodUpdate(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()
    
    firstCall := true
    
    env.OnActivity(activity.ValidatePayment, mock.Anything, mock.Anything).
        Return(&workflow.ValidationResult{Valid: true}, nil)
    
    env.OnActivity(activity.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(func(ctx context.Context, req workflow.PaymentRequest) (*workflow.ChargeResult, error) {
            if firstCall {
                firstCall = false
                return &workflow.ChargeResult{
                    Success:     false,
                    DeclineCode: "insufficient_funds",
                }, nil
            }
            // After signal, verify new payment method is used
            assert.Equal(t, "pm_new_card", req.PaymentMethodID)
            return &workflow.ChargeResult{
                Success:       true,
                TransactionID: "txn_new_method",
            }, nil
        })
    
    env.OnActivity(activity.UpdateLedger, mock.Anything, mock.Anything).
        Return(nil)
    
    env.OnActivity(activity.PublishEvent, mock.Anything, mock.Anything).
        Return(nil)
    
    // Register callback to send signal during retry wait
    env.RegisterDelayedCallback(func() {
        env.SignalWorkflow(workflow.SignalUpdatePaymentMethod, workflow.PaymentMethod{
            ID:   "pm_new_card",
            Type: "card",
        })
    }, 1*time.Hour) // After initial retry delay starts
    
    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID:       "pay_update_method",
        Amount:          decimal.NewFromFloat(100.00),
        SourceAccountID: "acc_source",
        PaymentType:     workflow.PaymentTypeCard,
        PaymentMethodID: "pm_old_card",
    })
    
    require.True(t, env.IsWorkflowCompleted())
    
    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.True(t, result.Success)
    assert.Equal(t, "txn_new_method", result.TransactionID)
}

func TestPaymentWorkflow_Cancellation(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()
    
    env.OnActivity(activity.ValidatePayment, mock.Anything, mock.Anything).
        Return(&workflow.ValidationResult{Valid: true}, nil)
    
    env.OnActivity(activity.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(&workflow.ChargeResult{
            Success:     false,
            DeclineCode: "insufficient_funds",
        }, nil)
    
    // Send cancel signal during retry wait
    env.RegisterDelayedCallback(func() {
        env.SignalWorkflow(workflow.SignalCancelPayment, "user_requested")
    }, 30*time.Minute)
    
    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID:       "pay_cancel_test",
        Amount:          decimal.NewFromFloat(100.00),
        SourceAccountID: "acc_source",
        PaymentType:     workflow.PaymentTypeCard,
    })
    
    require.True(t, env.IsWorkflowCompleted())
    
    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.False(t, result.Success)
    assert.Equal(t, "cancelled", result.Reason)
}
```

### 11.3 Integration Testing

```go
// +build integration

package integration_test

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "testing"
    "time"

    "github.com/shopspring/decimal"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestIntegration_PaymentEndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup: Create test accounts with balances
    sourceAccount := createTestAccount(t, decimal.NewFromFloat(1000.00))
    destAccount := createTestAccount(t, decimal.NewFromFloat(0))
    
    // Execute: Submit payment via API
    idempotencyKey := fmt.Sprintf("test_%d", time.Now().UnixNano())
    resp, err := http.Post(
        baseURL+"/api/v1/payments",
        "application/json",
        strings.NewReader(fmt.Sprintf(`{
            "idempotency_key": "%s",
            "type": "INTERNAL",
            "source_account_id": "%s",
            "dest_account_id": "%s",
            "amount": "100.00",
            "currency": "USD"
        }`, idempotencyKey, sourceAccount.ID, destAccount.ID)),
    )
    
    require.NoError(t, err)
    require.Equal(t, http.StatusAccepted, resp.StatusCode)
    
    var paymentResp CreatePaymentResponse
    json.NewDecoder(resp.Body).Decode(&paymentResp)
    resp.Body.Close()
    
    // Wait for workflow completion
    waitForPaymentStatus(t, paymentResp.ID, "SUCCEEDED", 30*time.Second)
    
    // Verify: Check account balances
    sourceAccount = getAccount(t, sourceAccount.ID)
    destAccount = getAccount(t, destAccount.ID)
    
    assert.True(t, sourceAccount.Balance.Equal(decimal.NewFromFloat(900.00)),
        "Source balance should be 900.00, got %s", sourceAccount.Balance)
    assert.True(t, destAccount.Balance.Equal(decimal.NewFromFloat(100.00)),
        "Dest balance should be 100.00, got %s", destAccount.Balance)
    
    // Verify: Check ledger entries
    entries := getLedgerEntries(t, paymentResp.ID)
    assert.Len(t, entries, 2, "Should have 2 ledger entries (debit + credit)")
    
    // Verify one debit and one credit
    var hasDebit, hasCredit bool
    for _, entry := range entries {
        if entry.Type == "DEBIT" {
            hasDebit = true
            assert.Equal(t, sourceAccount.ID, entry.AccountID)
        }
        if entry.Type == "CREDIT" {
            hasCredit = true
            assert.Equal(t, destAccount.ID, entry.AccountID)
        }
    }
    assert.True(t, hasDebit, "Should have a debit entry")
    assert.True(t, hasCredit, "Should have a credit entry")
}

func TestIntegration_IdempotencyKey(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    sourceAccount := createTestAccount(t, decimal.NewFromFloat(1000.00))
    destAccount := createTestAccount(t, decimal.NewFromFloat(0))
    
    idempotencyKey := fmt.Sprintf("test_idempotent_%d", time.Now().UnixNano())
    
    // Submit same payment twice
    for i := 0; i < 2; i++ {
        resp, err := http.Post(
            baseURL+"/api/v1/payments",
            "application/json",
            strings.NewReader(fmt.Sprintf(`{
                "idempotency_key": "%s",
                "type": "INTERNAL",
                "source_account_id": "%s",
                "dest_account_id": "%s",
                "amount": "50.00",
                "currency": "USD"
            }`, idempotencyKey, sourceAccount.ID, destAccount.ID)),
        )
        require.NoError(t, err)
        require.Equal(t, http.StatusAccepted, resp.StatusCode)
        resp.Body.Close()
    }
    
    // Wait for processing
    waitForPaymentStatus(t, idempotencyKey, "SUCCEEDED", 30*time.Second)
    
    // Verify: Should only have processed once
    sourceAccount = getAccount(t, sourceAccount.ID)
    assert.True(t, sourceAccount.Balance.Equal(decimal.NewFromFloat(950.00)),
        "Should only debit 50.00 once, got balance %s", sourceAccount.Balance)
}

// Helper functions
func waitForPaymentStatus(t *testing.T, paymentID, expectedStatus string, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        resp, err := http.Get(fmt.Sprintf("%s/api/v1/payments/%s", baseURL, paymentID))
        require.NoError(t, err)
        
        var status PaymentStatusResponse
        json.NewDecoder(resp.Body).Decode(&status)
        resp.Body.Close()
        
        if status.Status == expectedStatus {
            return
        }
        
        time.Sleep(500 * time.Millisecond)
    }
    t.Fatalf("Payment %s did not reach status %s within %v", paymentID, expectedStatus, timeout)
}
```

---

## 12. Docker Compose Setup

```yaml
version: '3.8'

services:
  # Temporal Server (with auto-setup)
  temporal:
    image: temporalio/auto-setup:1.22
    ports:
      - "7233:7233"   # gRPC frontend
    environment:
      - DB=postgresql
      - DB_PORT=5432
      - POSTGRES_USER=temporal
      - POSTGRES_PWD=temporal
      - POSTGRES_SEEDS=temporal-db
      - DYNAMIC_CONFIG_FILE_PATH=config/dynamicconfig/development.yaml
    depends_on:
      temporal-db:
        condition: service_healthy
    volumes:
      - ./config/temporal:/etc/temporal/config/dynamicconfig

  # Temporal Web UI
  temporal-ui:
    image: temporalio/ui:2.21.0
    ports:
      - "8080:8080"
    environment:
      - TEMPORAL_ADDRESS=temporal:7233
      - TEMPORAL_CORS_ORIGINS=http://localhost:3000
    depends_on:
      - temporal

  # Database for Temporal
  temporal-db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=temporal
      - POSTGRES_PASSWORD=temporal
      - POSTGRES_DB=temporal
    volumes:
      - temporal_db_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U temporal"]
      interval: 5s
      timeout: 5s
      retries: 5

  # Application Database
  app-db:
    image: postgres:15-alpine
    ports:
      - "5433:5432"
    environment:
      - POSTGRES_USER=payments
      - POSTGRES_PASSWORD=payments
      - POSTGRES_DB=payments
    volumes:
      - app_db_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U payments -d payments"]
      interval: 5s
      timeout: 5s
      retries: 5

  # NATS Message Broker
  nats:
    image: nats:2.10-alpine
    ports:
      - "4222:4222"   # Client connections
      - "8222:8222"   # HTTP monitoring
    command: ["--http_port", "8222", "--jetstream"]
    volumes:
      - nats_data:/data

  # API Server
  api:
    build:
      context: .
      dockerfile: Dockerfile
      target: api
    ports:
      - "8000:8000"
    environment:
      - DATABASE_URL=postgres://payments:payments@app-db:5432/payments?sslmode=disable
      - TEMPORAL_HOST=temporal:7233
      - TEMPORAL_NAMESPACE=default
      - NATS_URL=nats://nats:4222
      - STRIPE_API_KEY=${STRIPE_API_KEY:-sk_test_xxx}
    depends_on:
      app-db:
        condition: service_healthy
      temporal:
        condition: service_started
      nats:
        condition: service_started

  # Temporal Worker
  worker:
    build:
      context: .
      dockerfile: Dockerfile
      target: worker
    environment:
      - DATABASE_URL=postgres://payments:payments@app-db:5432/payments?sslmode=disable
      - TEMPORAL_HOST=temporal:7233
      - TEMPORAL_NAMESPACE=default
      - TEMPORAL_TASK_QUEUE=payments
      - NATS_URL=nats://nats:4222
      - STRIPE_API_KEY=${STRIPE_API_KEY:-sk_test_xxx}
    depends_on:
      app-db:
        condition: service_healthy
      temporal:
        condition: service_started
      nats:
        condition: service_started
    deploy:
      replicas: 2  # Run multiple workers for availability

volumes:
  temporal_db_data:
  app_db_data:
  nats_data:
```

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build API
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

# Build Worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./cmd/worker

# API image
FROM alpine:3.19 AS api
RUN apk add --no-cache ca-certificates
COPY --from=builder /api /api
EXPOSE 8000
CMD ["/api"]

# Worker image
FROM alpine:3.19 AS worker
RUN apk add --no-cache ca-certificates
COPY --from=builder /worker /worker
CMD ["/worker"]
```

### Makefile

```makefile
.PHONY: all build run test clean migrate

# Development
run-infra:
	docker-compose up -d temporal temporal-ui temporal-db app-db nats

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

# Full stack
up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

# Database
migrate:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

# Testing
test:
	go test ./... -v -short

test-integration:
	go test ./... -v -tags=integration

# Build
build:
	docker-compose build

# Utilities
temporal-ui:
	open http://localhost:8080

clean:
	docker-compose down -v
	rm -rf ./bin
```

---

## Next Steps

This specification provides the foundation for building a production-quality payment processing service. The patterns demonstrated here are directly applicable to roles at financial institutions like Vancity:

- **Durable workflows** for long-running payment operations
- **Idempotent activities** preventing duplicate charges
- **Intelligent retry logic** maximizing payment recovery
- **Event-driven architecture** for downstream analytics
- **Double-entry bookkeeping** for financial accuracy
- **Comprehensive audit trails** for compliance

Start with Phase 1 (Foundation) and work through each phase systematically. The Temporal test framework makes it easy to validate workflow logic without running infrastructure.