package server

import (
	"time"

	"github.com/shopspring/decimal"
)

// CreatePaymentIntentRequest represents a request to create a payment intent
type CreatePaymentIntentRequest struct {
	Amount          decimal.Decimal   `json:"amount"`
	Currency        string            `json:"currency"`
	CustomerID      string            `json:"customer_id"`
	Provider        string            `json:"provider"`
	CaptureMethod   string            `json:"capture_method,omitempty"`
	PaymentMethodID *string           `json:"payment_method_id,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// CapturePaymentRequest represents a request to capture a payment
type CapturePaymentRequest struct {
	Amount *decimal.Decimal `json:"amount,omitempty"`
}

// PaymentIntentResponse represents a payment intent in API responses
type PaymentIntentResponse struct {
	Amount            decimal.Decimal   `json:"amount"`
	ID                string            `json:"id"`
	Status            string            `json:"status"`
	Currency          string            `json:"currency"`
	CustomerID        string            `json:"customer_id"`
	Provider          string            `json:"provider"`
	CaptureMethod     string            `json:"capture_method"`
	PaymentMethodID   *string           `json:"payment_method_id,omitempty"`
	ProviderPaymentID *string           `json:"provider_payment_id,omitempty"`
	WorkflowID        string            `json:"workflow_id"`
	IdempotencyKey    string            `json:"idempotency_key"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	AuthorizedAt      *time.Time        `json:"authorized_at,omitempty"`
	CapturedAt        *time.Time        `json:"captured_at,omitempty"`
	Hold              *HoldResponse     `json:"hold,omitempty"`
}

// HoldResponse represents an authorization hold in API responses
type HoldResponse struct {
	Amount            decimal.Decimal `json:"amount"`
	ID                string          `json:"id"`
	Status            string          `json:"status"`
	AuthorizationCode string          `json:"authorization_code,omitempty"`
	ExpiresAt         time.Time       `json:"expires_at"`
	CreatedAt         time.Time       `json:"created_at"`
}

// AccountResponse represents an account in API responses
type AccountResponse struct {
	LedgerBalance    decimal.Decimal `json:"ledger_balance"`
	PendingBalance   decimal.Decimal `json:"pending_balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	ID               string          `json:"id"`
	Type             string          `json:"type"`
	Name             string          `json:"name"`
	Currency         string          `json:"currency"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// LedgerEntryResponse represents a ledger entry in API responses
type LedgerEntryResponse struct {
	Amount         decimal.Decimal `json:"amount"`
	BalanceAfter   decimal.Decimal `json:"balance_after"`
	ID             string          `json:"id"`
	JournalEntryID string          `json:"journal_entry_id"`
	Direction      string          `json:"direction"`
	CreatedAt      time.Time       `json:"created_at"`
}

// LedgerEntriesListResponse represents a paginated list of ledger entries
type LedgerEntriesListResponse struct {
	Data       []LedgerEntryResponse `json:"data"`
	HasMore    bool                  `json:"has_more"`
	NextCursor *string               `json:"next_cursor,omitempty"`
}
