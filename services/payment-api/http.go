package server

import (
	"time"

	"github.com/shopspring/decimal"
)

type AttachPaymentMethodRequest struct {
	PaymentMethodID string `json:"payment_method_id"`
}

type CancelPaymentRequest struct {
	Reason string `json:"reason,omitempty"`
}

type PaymentAttemptResponse struct {
	ID             string     `json:"id"`
	AttemptNumber  int        `json:"attempt_number"`
	Status         string     `json:"status"`
	Provider       string     `json:"provider"`
	CanonicalCode  *string    `json:"canonical_decline_code,omitempty"`
	DeclineType    *string    `json:"decline_type,omitempty"`
	ProcessorTxnID *string    `json:"processor_txn_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type AttemptsListResponse struct {
	Data    []PaymentAttemptResponse `json:"data"`
	HasMore bool                     `json:"has_more"`
}

type HealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version,omitempty"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type WebhookResponse struct {
	Received bool   `json:"received"`
	Message  string `json:"message,omitempty"`
}

type PaginationParams struct {
	Limit  int
	Cursor string
}

func ParsePaginationParams(limitStr, cursor string) PaginationParams {
	limit := 50
	if limitStr != "" {
		var parsed int
		if _, err := decimal.NewFromString(limitStr); err == nil {
			d, _ := decimal.NewFromString(limitStr)
			parsed = int(d.IntPart())
		}
		if parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	return PaginationParams{
		Limit:  limit,
		Cursor: cursor,
	}
}
