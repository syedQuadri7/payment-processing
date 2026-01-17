# Temporal Testing Patterns

## Table of Contents
- [Test Setup](#test-setup)
- [Mocking Activities](#mocking-activities)
- [Testing Signals](#testing-signals)
- [Testing Queries](#testing-queries)
- [Testing Retry Behavior](#testing-retry-behavior)
- [Integration Testing](#integration-testing)

## Test Setup

Use the Temporal test suite for workflow unit tests:

```go
import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
    "go.temporal.io/sdk/testsuite"
)

func TestPaymentWorkflow_Success(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    // Mock activities
    env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
        Return(&ValidationResult{Valid: true}, nil)

    env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(&ChargeResult{Success: true, TransactionID: "txn_123"}, nil)

    env.OnActivity(activities.UpdateLedger, mock.Anything, mock.Anything).
        Return(nil)

    env.OnActivity(activities.PublishEvent, mock.Anything, mock.Anything).
        Return(nil)

    // Execute workflow
    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID:       "pay_test",
        Amount:          decimal.NewFromFloat(100.00),
        SourceAccountID: "acc_source",
        DestAccountID:   "acc_dest",
        PaymentType:     workflow.PaymentTypeCard,
    })

    // Verify completion
    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())

    // Verify result
    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.True(t, result.Success)
    assert.Equal(t, "txn_123", result.TransactionID)
}
```

## Mocking Activities

### Static return values

```go
env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
    Return(&ValidationResult{Valid: true}, nil)
```

### Dynamic return values based on call count

```go
attemptCount := 0
env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
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
```

### Returning errors

```go
// Retryable error (activity will be retried)
env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
    Return(nil, errors.New("network timeout"))

// Non-retryable error
env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
    Return(nil, temporal.NewApplicationError("Card stolen", "HardDeclineError"))
```

### Verifying activity inputs

```go
env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
    Return(func(ctx context.Context, req workflow.PaymentRequest) (*workflow.ChargeResult, error) {
        assert.Equal(t, "pay_123", req.PaymentID)
        assert.Equal(t, decimal.NewFromFloat(100.00), req.Amount)
        return &workflow.ChargeResult{Success: true}, nil
    })
```

## Testing Signals

### Send signal during workflow execution

```go
func TestPaymentWorkflow_Cancellation(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
        Return(&ValidationResult{Valid: true}, nil)

    env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(&ChargeResult{Success: false, DeclineCode: "insufficient_funds"}, nil)

    // Send cancel signal after 30 minutes (during retry wait)
    env.RegisterDelayedCallback(func() {
        env.SignalWorkflow(workflow.SignalCancelPayment, "user_requested")
    }, 30*time.Minute)

    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID: "pay_cancel_test",
        Amount:    decimal.NewFromFloat(100.00),
    })

    require.True(t, env.IsWorkflowCompleted())

    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.False(t, result.Success)
    assert.Equal(t, "cancelled", result.Reason)
}
```

### Test payment method update signal

```go
func TestPaymentWorkflow_PaymentMethodUpdate(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    firstCall := true

    env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
        Return(&ValidationResult{Valid: true}, nil)

    env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(func(ctx context.Context, req workflow.PaymentRequest) (*workflow.ChargeResult, error) {
            if firstCall {
                firstCall = false
                return &ChargeResult{Success: false, DeclineCode: "insufficient_funds"}, nil
            }
            // Verify new payment method after signal
            assert.Equal(t, "pm_new_card", req.PaymentMethodID)
            return &ChargeResult{Success: true, TransactionID: "txn_new"}, nil
        })

    env.OnActivity(activities.UpdateLedger, mock.Anything, mock.Anything).Return(nil)
    env.OnActivity(activities.PublishEvent, mock.Anything, mock.Anything).Return(nil)

    // Send signal during retry wait
    env.RegisterDelayedCallback(func() {
        env.SignalWorkflow(workflow.SignalUpdatePaymentMethod, workflow.PaymentMethod{
            ID:   "pm_new_card",
            Type: "card",
        })
    }, 1*time.Hour)

    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID:       "pay_update",
        PaymentMethodID: "pm_old_card",
    })

    require.True(t, env.IsWorkflowCompleted())

    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.True(t, result.Success)
}
```

## Testing Queries

```go
func TestPaymentWorkflow_QueryStatus(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
        Return(&ValidationResult{Valid: true}, nil)

    // Make payment fail to keep workflow running
    env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(&ChargeResult{Success: false, DeclineCode: "insufficient_funds"}, nil)

    // Query status after some time
    env.RegisterDelayedCallback(func() {
        result, err := env.QueryWorkflow("get-status")
        require.NoError(t, err)

        var state workflow.PaymentState
        require.NoError(t, result.Get(&state))
        assert.Equal(t, workflow.StatusRecovering, state.Status)
        assert.Equal(t, 1, state.AttemptCount)
    }, 1*time.Hour)

    // Cancel to end the test
    env.RegisterDelayedCallback(func() {
        env.SignalWorkflow(workflow.SignalCancelPayment, "test_complete")
    }, 2*time.Hour)

    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID: "pay_query_test",
    })
}
```

## Testing Retry Behavior

### Verify soft decline triggers retry

```go
func TestPaymentWorkflow_SoftDecline_Retry(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    attemptCount := 0

    env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
        Return(&ValidationResult{Valid: true}, nil)

    env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(func(ctx context.Context, req workflow.PaymentRequest) (*workflow.ChargeResult, error) {
            attemptCount++
            if attemptCount < 3 {
                return &ChargeResult{Success: false, DeclineCode: "insufficient_funds"}, nil
            }
            return &ChargeResult{Success: true, TransactionID: "txn_success"}, nil
        })

    env.OnActivity(activities.UpdateLedger, mock.Anything, mock.Anything).Return(nil)
    env.OnActivity(activities.PublishEvent, mock.Anything, mock.Anything).Return(nil)

    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID: "pay_retry",
    })

    require.True(t, env.IsWorkflowCompleted())
    require.NoError(t, env.GetWorkflowError())

    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.True(t, result.Success)
    assert.Equal(t, 3, attemptCount, "Should take 3 attempts")
}
```

### Verify hard decline does not retry

```go
func TestPaymentWorkflow_HardDecline_NoRetry(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()

    attemptCount := 0

    env.OnActivity(activities.ValidatePayment, mock.Anything, mock.Anything).
        Return(&ValidationResult{Valid: true}, nil)

    env.OnActivity(activities.ProcessCardPayment, mock.Anything, mock.Anything).
        Return(func(ctx context.Context, req workflow.PaymentRequest) (*workflow.ChargeResult, error) {
            attemptCount++
            return &ChargeResult{Success: false, DeclineCode: "stolen_card"},
                temporal.NewApplicationError("Card stolen", "HardDeclineError")
        })

    env.OnActivity(activities.PublishEvent, mock.Anything, mock.Anything).Return(nil)

    env.ExecuteWorkflow(workflow.PaymentWorkflow, workflow.PaymentRequest{
        PaymentID: "pay_hard_decline",
    })

    require.True(t, env.IsWorkflowCompleted())

    var result workflow.PaymentResult
    require.NoError(t, env.GetWorkflowResult(&result))
    assert.False(t, result.Success)
    assert.Equal(t, "stolen_card", result.Reason)
    assert.Equal(t, 1, attemptCount, "Should not retry hard declines")
}
```

## Integration Testing

Run against real Temporal server with test accounts:

```go
//go:build integration

package integration_test

func TestIntegration_PaymentEndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup test accounts
    sourceAccount := createTestAccount(t, decimal.NewFromFloat(1000.00))
    destAccount := createTestAccount(t, decimal.NewFromFloat(0))

    // Submit payment via API
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

    // Wait for completion
    waitForPaymentStatus(t, paymentResp.ID, "SUCCEEDED", 30*time.Second)

    // Verify balances
    sourceAccount = getAccount(t, sourceAccount.ID)
    destAccount = getAccount(t, destAccount.ID)

    assert.True(t, sourceAccount.Balance.Equal(decimal.NewFromFloat(900.00)))
    assert.True(t, destAccount.Balance.Equal(decimal.NewFromFloat(100.00)))

    // Verify ledger entries
    entries := getLedgerEntries(t, paymentResp.ID)
    assert.Len(t, entries, 2) // debit + credit
}

func waitForPaymentStatus(t *testing.T, paymentID, expectedStatus string, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        resp, _ := http.Get(baseURL + "/api/v1/payments/" + paymentID)
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

### Run tests

```bash
# Unit tests
go test ./... -v -short

# Integration tests (requires running infrastructure)
docker-compose up -d
go test ./... -tags=integration -v
```
