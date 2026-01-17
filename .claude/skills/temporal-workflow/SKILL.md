---
name: temporal-workflow
description: Patterns for writing Temporal workflows and activities in Go for the payment-processing service. Use when implementing new workflows, adding activities, handling signals/queries, writing retry logic, or testing Temporal code. Triggers on tasks involving workflow orchestration, durable execution, or payment processing logic.
---

# Temporal Workflow Development

Patterns for this payment-processing service built with Go and Temporal SDK.

## Core Rules

### Determinism in Workflows

Workflow code must be deterministic. On replay, the same inputs must produce the same outputs.

**Allowed in workflows:**
- `workflow.Now(ctx)` for current time
- `workflow.GetLogger(ctx)` for logging
- `workflow.SideEffect()` for one-time non-deterministic values
- `workflow.ExecuteActivity()` for side effects

**Forbidden in workflows:**
- `time.Now()`, `rand.Int()`, `uuid.New()`
- Direct database/API calls
- Global mutable state

```go
// WRONG
id := uuid.New().String()

// CORRECT
var id string
workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
    return uuid.New().String()
}).Get(&id)
```

## Activity Configuration

Standard activity options for this project:

```go
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
```

For database operations, use shorter timeouts:
```go
actOpts := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second}
```

## Query Handlers

Expose workflow state for external visibility:

```go
func PaymentWorkflow(ctx workflow.Context, req PaymentRequest) (*PaymentResult, error) {
    state := &PaymentState{PaymentID: req.PaymentID, Status: StatusPending}

    _ = workflow.SetQueryHandler(ctx, "get-status", func() (*PaymentState, error) {
        return state, nil
    })

    _ = workflow.SetQueryHandler(ctx, "get-attempts", func() ([]AttemptRecord, error) {
        return state.Attempts, nil
    })
    // ... workflow logic
}
```

Query from API:
```go
resp, err := temporalClient.QueryWorkflow(ctx, workflowID, "", "get-status")
var state PaymentState
resp.Get(&state)
```

## Signal Handlers

Handle external events during workflow execution:

```go
const (
    SignalUpdatePaymentMethod = "update-payment-method"
    SignalCancelPayment       = "cancel-payment"
)

func PaymentWorkflow(ctx workflow.Context, req PaymentRequest) (*PaymentResult, error) {
    updateMethodCh := workflow.GetSignalChannel(ctx, SignalUpdatePaymentMethod)
    cancelCh := workflow.GetSignalChannel(ctx, SignalCancelPayment)

    // Non-blocking check pattern
    selector := workflow.NewSelector(ctx)
    selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
        var reason string
        c.Receive(ctx, &reason)
        // Handle cancellation
    })
    selector.AddDefault(func() {}) // Non-blocking
    selector.Select(ctx)
}
```

Send signal from API:
```go
err := temporalClient.SignalWorkflow(ctx, workflowID, "", SignalCancelPayment, "user_requested")
```

## Timer with Signal Handling

Wait for duration while remaining responsive to signals:

```go
func waitWithSignals(ctx workflow.Context, duration time.Duration, cancelCh workflow.ReceiveChannel) error {
    timer := workflow.NewTimer(ctx, duration)
    selector := workflow.NewSelector(ctx)
    cancelled := false

    selector.AddFuture(timer, func(f workflow.Future) {
        // Timer completed
    })

    selector.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
        var reason string
        c.Receive(ctx, &reason)
        cancelled = true
    })

    selector.Select(ctx)

    if cancelled {
        return fmt.Errorf("cancelled")
    }
    return nil
}
```

## Child Workflows

For scheduled/recurring payments, spawn child workflows:

```go
childOpts := workflow.ChildWorkflowOptions{
    WorkflowID: fmt.Sprintf("payment-%s-%d", scheduleID, paymentNum),
}
ctx = workflow.WithChildOptions(ctx, childOpts)

var result PaymentResult
err := workflow.ExecuteChildWorkflow(ctx, PaymentWorkflow, paymentReq).Get(ctx, &result)
```

## Error Classification

Use typed errors for retry policy control:

```go
// In activity - non-retryable error
return nil, temporal.NewApplicationError("Card stolen", "HardDeclineError")

// In activity - soft decline (let workflow decide retry)
return &ChargeResult{Success: false, DeclineCode: "insufficient_funds"}, nil
```

See `references/patterns.md` for decline classification and retry timing strategies.

## Testing

Use Temporal test framework with mocked activities:

```go
func TestPaymentWorkflow_Success(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
        Return(&ValidationResult{Valid: true}, nil)

    env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(&ChargeResult{Success: true, TransactionID: "txn_123"}, nil)

    env.ExecuteWorkflow(PaymentWorkflow, PaymentRequest{...})

    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())
}
```

Test signals with delayed callbacks:
```go
env.RegisterDelayedCallback(func() {
    env.SignalWorkflow(SignalCancelPayment, "user_requested")
}, 30*time.Minute)
```

See `references/testing.md` for complete testing patterns.
