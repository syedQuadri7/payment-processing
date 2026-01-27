package workflowtypes

import (
	"time"
)

// Task queue name for payment processing workers
const TaskQueueName = "payment-processing"

// Workflow names for starting workflows from external services
const (
	PaymentIntentWorkflowName   = "PaymentIntentWorkflow"
	ProcessPaymentWorkflowName  = "ProcessPaymentWorkflow"
)

// Signal names for workflow communication
const (
	SignalWebhookEvent        = "webhook-event"
	SignalCancelPayment       = "cancel-payment"
	SignalUpdatePaymentMethod = "update-payment-method"
)

// Query names for workflow state inspection
const (
	QueryPaymentStatus = "get-status"
	QueryPaymentState  = "get-state"
)

// RetryIntervals defines fixed retry delays as per FD-015
// Soft declines are retried at 4h, 12h, 24h, and 48h intervals
var RetryIntervals = []time.Duration{
	4 * time.Hour,
	12 * time.Hour,
	24 * time.Hour,
	48 * time.Hour,
}
