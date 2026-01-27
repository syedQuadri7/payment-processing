package workflow

import (
	"time"

	"payment-processing/shared/workflowtypes"
)

// Re-export constants from shared workflowtypes for internal use
const TaskQueueName = workflowtypes.TaskQueueName
const PaymentIntentWorkflowName = workflowtypes.PaymentIntentWorkflowName
const ProcessPaymentWorkflowName = workflowtypes.ProcessPaymentWorkflowName

// Signal names for workflow communication
const (
	SignalWebhookEvent        = workflowtypes.SignalWebhookEvent
	SignalCancelPayment       = workflowtypes.SignalCancelPayment
	SignalUpdatePaymentMethod = workflowtypes.SignalUpdatePaymentMethod
)

// Query names for workflow state inspection
const (
	QueryPaymentStatus = workflowtypes.QueryPaymentStatus
	QueryPaymentState  = workflowtypes.QueryPaymentState
)

// RetryIntervals defines fixed retry delays as per FD-015
// Soft declines are retried at 4h, 12h, 24h, and 48h intervals
var RetryIntervals = []time.Duration{
	4 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
	48 * time.Hour,
}
