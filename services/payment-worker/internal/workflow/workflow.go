package workflow

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// Legacy workflow - kept for backwards compatibility during migration
// TODO: Remove after migration to PaymentIntentWorkflow is complete

func ProcessPaymentWorkflow(ctx workflow.Context, input LegacyPaymentInput) (*LegacyPaymentResult, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var result LegacyPaymentResult

	err := workflow.ExecuteActivity(ctx, ValidatePayment, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}

	if result.Status == "invalid" {
		return &result, nil
	}

	err = workflow.ExecuteActivity(ctx, ExecutePayment, input).Get(ctx, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func ValidatePayment(ctx context.Context, input LegacyPaymentInput) (*LegacyPaymentResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Validating payment", "id", input.ID, "amount", input.Amount)

	if input.Amount.LessThanOrEqual(decimal.Zero) {
		return &LegacyPaymentResult{
			ID:        input.ID,
			Status:    "invalid",
			Message:   "Amount must be positive",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	if input.From == "" || input.To == "" {
		return &LegacyPaymentResult{
			ID:        input.ID,
			Status:    "invalid",
			Message:   "From and To fields are required",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	return &LegacyPaymentResult{
		ID:        input.ID,
		Status:    "validated",
		Message:   "Payment validated successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func ExecutePayment(ctx context.Context, input LegacyPaymentInput) (*LegacyPaymentResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Executing payment", "id", input.ID, "amount", input.Amount, "from", input.From, "to", input.To)

	// Simulate payment processing
	time.Sleep(100 * time.Millisecond)

	return &LegacyPaymentResult{
		ID:        input.ID,
		Status:    "completed",
		Message:   "Payment processed successfully",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
