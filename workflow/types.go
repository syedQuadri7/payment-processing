package workflow

import (
	"time"

	"github.com/shopspring/decimal"

	"payment-processing/pkg/domain"
)

// PaymentWorkflowInput contains all data needed to start a payment workflow
type PaymentWorkflowInput struct {
	PaymentIntentID string               `json:"payment_intent_id"`
	CustomerID      string               `json:"customer_id"`
	Amount          decimal.Decimal      `json:"amount"`
	Currency        string               `json:"currency"`
	Provider        domain.Provider      `json:"provider"`
	CaptureMethod   domain.CaptureMethod `json:"capture_method"`
	PaymentMethodID *string              `json:"payment_method_id,omitempty"`
	IdempotencyKey  string               `json:"idempotency_key"`
	Metadata        map[string]any       `json:"metadata,omitempty"`
}

// PaymentWorkflowResult is the final result of a payment workflow
type PaymentWorkflowResult struct {
	PaymentIntentID   string                     `json:"payment_intent_id"`
	Status            domain.PaymentIntentStatus `json:"status"`
	ProviderPaymentID *string                    `json:"provider_payment_id,omitempty"`
	AuthorizationCode *string                    `json:"authorization_code,omitempty"`
	CapturedAmount    *decimal.Decimal           `json:"captured_amount,omitempty"`
	DeclineCode       *string                    `json:"decline_code,omitempty"`
	DeclineType       *domain.DeclineType        `json:"decline_type,omitempty"`
	ErrorMessage      *string                    `json:"error_message,omitempty"`
	AttemptCount      int                        `json:"attempt_count"`
	CompletedAt       time.Time                  `json:"completed_at"`
}
