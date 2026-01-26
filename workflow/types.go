package workflow

import (
	"fmt"
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

// Validate performs shift-left validation on workflow input.
// This catches invalid data at workflow entry before any activities run.
func (i *PaymentWorkflowInput) Validate() error {
	if i.PaymentIntentID == "" {
		return fmt.Errorf("validation error: payment_intent_id is required")
	}

	if i.CustomerID == "" {
		return fmt.Errorf("validation error: customer_id is required")
	}

	if i.Amount.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("validation error: amount must be greater than zero")
	}

	if i.Currency == "" {
		return fmt.Errorf("validation error: currency is required")
	}
	if len(i.Currency) != 3 {
		return fmt.Errorf("validation error: currency must be a 3-letter ISO code")
	}

	if i.Provider == "" {
		return fmt.Errorf("validation error: provider is required")
	}
	if i.Provider != domain.ProviderStripe && i.Provider != domain.ProviderAdyen && i.Provider != domain.ProviderPayPal {
		return fmt.Errorf("validation error: invalid provider '%s'", i.Provider)
	}

	if i.CaptureMethod == "" {
		return fmt.Errorf("validation error: capture_method is required")
	}
	if i.CaptureMethod != domain.CaptureMethodAutomatic && i.CaptureMethod != domain.CaptureMethodManual {
		return fmt.Errorf("validation error: invalid capture_method '%s'", i.CaptureMethod)
	}

	if i.IdempotencyKey == "" {
		return fmt.Errorf("validation error: idempotency_key is required")
	}

	return nil
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
