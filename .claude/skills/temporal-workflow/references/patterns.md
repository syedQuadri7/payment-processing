# Payment Processing Patterns

## Table of Contents
- [Decline Classification](#decline-classification)
- [Retry Timing Strategy](#retry-timing-strategy)
- [Idempotency](#idempotency)
- [Activity Patterns](#activity-patterns)
- [Scheduled Payments](#scheduled-payments)

## Decline Classification

Proper classification drives retry decisions:

| Category | Example Codes | Action | Retry |
|----------|---------------|--------|-------|
| **Soft** | `insufficient_funds`, `generic_decline`, `do_not_honor`, `processing_error` | Enter recovery | Yes - intelligent timing |
| **Hard** | `stolen_card`, `lost_card`, `expired_card`, `invalid_account` | Fail immediately | No |
| **Fraud** | `fraudulent`, `pickup_card`, `security_violation` | Fail and flag | No |
| **Temporary** | `rate_limit`, `timeout`, `service_unavailable` | Activity retry | Yes - exponential backoff |

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
        return DeclineTypeSoft // Safe default
    }
}
```

## Retry Timing Strategy

Intelligent retry timing based on decline type (inspired by Butter Payments):

```go
func calculateRetryDelay(declineCode string, attempt int, req PaymentRequest) time.Duration {
    // Insufficient funds - align with paydays
    if declineCode == "insufficient_funds" {
        return alignToNextPayday(req.CustomerTimezone)
    }

    // Generic decline - try different times of day
    if declineCode == "generic_decline" || declineCode == "do_not_honor" {
        baseDelay := []time.Duration{
            4 * time.Hour,
            12 * time.Hour,
            24 * time.Hour,
            48 * time.Hour,
            7 * 24 * time.Hour,
        }
        if attempt < len(baseDelay) {
            return baseDelay[attempt]
        }
    }

    // Rate limited - short backoff
    if declineCode == "rate_limit" || declineCode == "try_again_later" {
        return time.Duration(1<<attempt) * time.Minute // 1, 2, 4, 8 min
    }

    // Debit cards - retry on common pay dates
    if req.PaymentMethod != nil && req.PaymentMethod.Type == "debit" {
        return alignToPayDate(req.CustomerTimezone)
    }

    // Default exponential backoff with cap
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
        nextMonth := now.AddDate(0, 1, 0)
        nextPayday = time.Date(nextMonth.Year(), nextMonth.Month(), 1, 6, 0, 0, 0, loc)
    }

    // Skip weekends
    for nextPayday.Weekday() == time.Saturday || nextPayday.Weekday() == time.Sunday {
        nextPayday = nextPayday.AddDate(0, 0, 1)
    }

    delay := nextPayday.Sub(now)
    if delay < time.Hour {
        delay = time.Hour
    }
    return delay
}
```

## Idempotency

Prevent duplicate charges using idempotency keys:

```go
func (a *PaymentActivities) ProcessCardPayment(ctx context.Context, req PaymentRequest) (*ChargeResult, error) {
    info := activity.GetInfo(ctx)

    // Deterministic key: workflow ID + payment ID + attempt number
    idempotencyKey := fmt.Sprintf("%s-%s-%d",
        info.WorkflowExecution.ID,
        req.PaymentID,
        info.Attempt,
    )

    params := &stripe.ChargeParams{
        Amount:         stripe.Int64(req.Amount.Mul(decimal.NewFromInt(100)).IntPart()),
        Currency:       stripe.String(req.Currency),
        IdempotencyKey: stripe.String(idempotencyKey),
    }
    // ...
}
```

For database operations, use upserts or check-then-insert with unique constraints:

```go
func (a *PaymentActivities) UpdateLedger(ctx context.Context, req LedgerRequest) error {
    // Use transaction ID as idempotency key
    _, err := a.db.ExecContext(ctx, `
        INSERT INTO ledger_entries (id, account_id, payment_id, type, amount)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (id) DO NOTHING
    `, req.TransactionID+"-debit", req.SourceAccountID, req.PaymentID, "DEBIT", req.Amount)
    return err
}
```

## Activity Patterns

### Activity struct with dependencies

```go
type PaymentActivities struct {
    stripeClient *stripe.Client
    db           *sql.DB
    eventBus     EventPublisher
}

func NewPaymentActivities(stripeClient *stripe.Client, db *sql.DB, eventBus EventPublisher) *PaymentActivities {
    return &PaymentActivities{
        stripeClient: stripeClient,
        db:           db,
        eventBus:     eventBus,
    }
}

// Register in worker
activities := NewPaymentActivities(stripeClient, db, eventBus)
w.RegisterActivity(activities.ProcessCardPayment)
w.RegisterActivity(activities.UpdateLedger)
w.RegisterActivity(activities.PublishEvent)
```

### Return soft declines as results, not errors

```go
func (a *PaymentActivities) ProcessCardPayment(ctx context.Context, req PaymentRequest) (*ChargeResult, error) {
    charge, err := a.stripeClient.Charges.Create(params)
    if err != nil {
        var stripeErr *stripe.Error
        if errors.As(err, &stripeErr) {
            switch stripeErr.DeclineCode {
            case stripe.DeclineCodeInsufficientFunds:
                // Soft decline - return result, let workflow handle retry
                return &ChargeResult{
                    Success:     false,
                    DeclineCode: string(stripeErr.DeclineCode),
                }, nil

            case stripe.DeclineCodeStolenCard:
                // Hard decline - return non-retryable error
                return nil, temporal.NewApplicationError(
                    stripeErr.Message, "HardDeclineError",
                )
            }
        }
        return nil, err // Retryable by default
    }

    return &ChargeResult{
        Success:       charge.Status == "succeeded",
        TransactionID: charge.ID,
    }, nil
}
```

## Scheduled Payments

Pattern for recurring payments using child workflows:

```go
func ScheduledPaymentWorkflow(ctx workflow.Context, schedule ScheduledPayment) error {
    state := &ScheduledState{ScheduleID: schedule.ID, Status: "waiting"}
    cancelCh := workflow.GetSignalChannel(ctx, "cancel-schedule")

    _ = workflow.SetQueryHandler(ctx, "get-schedule-status", func() (*ScheduledState, error) {
        return state, nil
    })

    for {
        nextPaymentTime := calculateNextPaymentTime(schedule, workflow.Now(ctx))

        if schedule.EndDate != nil && nextPaymentTime.After(*schedule.EndDate) {
            state.Status = "completed"
            return nil
        }

        // Wait until scheduled time with cancellation support
        selector := workflow.NewSelector(ctx)
        timerFired := false

        selector.AddFuture(workflow.NewTimer(ctx, nextPaymentTime.Sub(workflow.Now(ctx))), func(f workflow.Future) {
            timerFired = true
        })

        selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
            var reason string
            c.Receive(ctx, &reason)
            state.Status = "cancelled"
        })

        selector.Select(ctx)

        if state.Status == "cancelled" {
            return nil
        }

        if !timerFired {
            continue
        }

        // Spawn child workflow for this payment
        childOpts := workflow.ChildWorkflowOptions{
            WorkflowID: fmt.Sprintf("payment-%s-%d", schedule.ID, len(state.Payments)),
        }
        ctx = workflow.WithChildOptions(ctx, childOpts)

        var result PaymentResult
        _ = workflow.ExecuteChildWorkflow(ctx, PaymentWorkflow, PaymentRequest{
            PaymentID:       fmt.Sprintf("%s-%d", schedule.ID, len(state.Payments)),
            SourceAccountID: schedule.SourceAccountID,
            DestAccountID:   schedule.DestAccountID,
            Amount:          schedule.Amount,
            PaymentType:     schedule.PaymentType,
        }).Get(ctx, &result)

        state.Payments = append(state.Payments, result.TransactionID)

        // One-time scheduled payments exit after first execution
        if schedule.Frequency == FrequencyOnce {
            state.Status = "completed"
            return nil
        }
    }
}
```
