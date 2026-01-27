package workflow

import (
	"payment-processing/shared/workflowtypes"
)

// Re-export types from shared workflowtypes for internal use
type PaymentWorkflowInput = workflowtypes.PaymentWorkflowInput
type PaymentWorkflowResult = workflowtypes.PaymentWorkflowResult
type LegacyPaymentInput = workflowtypes.LegacyPaymentInput
type LegacyPaymentResult = workflowtypes.LegacyPaymentResult
