package server

import (
	"time"

	"github.com/shopspring/decimal"

	"payment-processing/shared/domain"
)

// Validation constants
const (
	// MaxMetadataKeys is the maximum number of metadata keys allowed
	MaxMetadataKeys = 50
	// MaxMetadataKeyLength is the maximum length of a metadata key
	MaxMetadataKeyLength = 40
	// MaxMetadataValueLength is the maximum length of a metadata value
	MaxMetadataValueLength = 500
	// MaxCustomerIDLength is the maximum length of a customer ID
	MaxCustomerIDLength = 255
	// MinCustomerIDLength is the minimum length of a customer ID
	MinCustomerIDLength = 1
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

// Validate validates the create payment intent request
func (r *CreatePaymentIntentRequest) Validate() *APIError {
	if r.Amount.LessThanOrEqual(decimal.Zero) {
		return NewValidationError("amount", "amount must be greater than zero")
	}

	if r.Currency == "" {
		return NewValidationError("currency", "currency is required")
	}
	if len(r.Currency) != 3 {
		return NewValidationError("currency", "currency must be a 3-letter ISO 4217 code")
	}

	// M6: CustomerID format validation
	if r.CustomerID == "" {
		return NewValidationError("customer_id", "customer_id is required")
	}
	if len(r.CustomerID) < MinCustomerIDLength || len(r.CustomerID) > MaxCustomerIDLength {
		return NewValidationError("customer_id", "customer_id must be between 1 and 255 characters")
	}
	// Validate CustomerID contains only allowed characters (alphanumeric, underscore, dash)
	for _, c := range r.CustomerID {
		if !isAllowedIDChar(c) {
			return NewValidationError("customer_id", "customer_id contains invalid characters (allowed: a-z, A-Z, 0-9, _, -)")
		}
	}

	if r.Provider == "" {
		return NewValidationError("provider", "provider is required")
	}
	provider := domain.Provider(r.Provider)
	if provider != domain.ProviderStripe && provider != domain.ProviderAdyen && provider != domain.ProviderPayPal {
		return NewValidationError("provider", "provider must be STRIPE, ADYEN, or PAYPAL")
	}

	if r.CaptureMethod != "" {
		cm := domain.CaptureMethod(r.CaptureMethod)
		if cm != domain.CaptureMethodAutomatic && cm != domain.CaptureMethodManual {
			return NewValidationError("capture_method", "capture_method must be automatic or manual")
		}
	}

	// M5: Metadata validation
	if err := validateMetadata(r.Metadata); err != nil {
		return err
	}

	return nil
}

// isAllowedIDChar checks if a character is allowed in IDs (alphanumeric, underscore, dash)
func isAllowedIDChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

// validateMetadata validates metadata map keys and values
func validateMetadata(metadata map[string]string) *APIError {
	if metadata == nil {
		return nil
	}

	if len(metadata) > MaxMetadataKeys {
		return NewValidationError("metadata", "metadata cannot have more than 50 keys")
	}

	for key, value := range metadata {
		// Validate key
		if key == "" {
			return NewValidationError("metadata", "metadata keys cannot be empty")
		}
		if len(key) > MaxMetadataKeyLength {
			return NewValidationError("metadata", "metadata key '"+key+"' exceeds maximum length of 40 characters")
		}
		// Keys must be alphanumeric with underscores
		for _, c := range key {
			if !isAllowedIDChar(c) {
				return NewValidationError("metadata", "metadata key '"+key+"' contains invalid characters (allowed: a-z, A-Z, 0-9, _, -)")
			}
		}

		// Validate value length
		if len(value) > MaxMetadataValueLength {
			return NewValidationError("metadata", "metadata value for key '"+key+"' exceeds maximum length of 500 characters")
		}
	}

	return nil
}

// PaymentIntentResponse represents a payment intent in API responses
type PaymentIntentResponse struct {
	ID                string            `json:"id"`
	Status            string            `json:"status"`
	Amount            decimal.Decimal   `json:"amount"`
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
	ID                string          `json:"id"`
	Amount            decimal.Decimal `json:"amount"`
	Status            string          `json:"status"`
	AuthorizationCode string          `json:"authorization_code,omitempty"`
	ExpiresAt         time.Time       `json:"expires_at"`
	CreatedAt         time.Time       `json:"created_at"`
}

// AttachPaymentMethodRequest represents a request to attach a payment method
type AttachPaymentMethodRequest struct {
	PaymentMethodID string `json:"payment_method_id"`
}

// Validate validates the attach payment method request
func (r *AttachPaymentMethodRequest) Validate() *APIError {
	if r.PaymentMethodID == "" {
		return NewValidationError("payment_method_id", "payment_method_id is required")
	}
	return nil
}

// CapturePaymentRequest represents a request to capture a payment
type CapturePaymentRequest struct {
	Amount *decimal.Decimal `json:"amount,omitempty"`
}

// Validate validates the capture payment request
func (r *CapturePaymentRequest) Validate() *APIError {
	if r.Amount != nil && r.Amount.LessThanOrEqual(decimal.Zero) {
		return NewValidationError("amount", "amount must be greater than zero")
	}
	return nil
}

// CancelPaymentRequest represents a request to cancel a payment
type CancelPaymentRequest struct {
	Reason string `json:"reason,omitempty"`
}

// PaymentAttemptResponse represents a payment attempt in API responses
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

// AttemptsListResponse represents a list of payment attempts
type AttemptsListResponse struct {
	Data    []PaymentAttemptResponse `json:"data"`
	HasMore bool                     `json:"has_more"`
}

// AccountResponse represents an account in API responses
type AccountResponse struct {
	ID               string          `json:"id"`
	Type             string          `json:"type"`
	Name             string          `json:"name"`
	Currency         string          `json:"currency"`
	LedgerBalance    decimal.Decimal `json:"ledger_balance"`
	PendingBalance   decimal.Decimal `json:"pending_balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// LedgerEntryResponse represents a ledger entry in API responses
type LedgerEntryResponse struct {
	ID             string          `json:"id"`
	JournalEntryID string          `json:"journal_entry_id"`
	Amount         decimal.Decimal `json:"amount"`
	Direction      string          `json:"direction"`
	BalanceAfter   decimal.Decimal `json:"balance_after"`
	CreatedAt      time.Time       `json:"created_at"`
}

// LedgerEntriesListResponse represents a paginated list of ledger entries
type LedgerEntriesListResponse struct {
	Data       []LedgerEntryResponse `json:"data"`
	HasMore    bool                  `json:"has_more"`
	NextCursor *string               `json:"next_cursor,omitempty"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version,omitempty"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// CheckResult represents the result of a health check
type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// WebhookResponse represents the response to webhook providers
type WebhookResponse struct {
	Received bool   `json:"received"`
	Message  string `json:"message,omitempty"`
}

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Limit  int
	Cursor string
}

// ParsePaginationParams parses pagination query parameters with defaults
func ParsePaginationParams(limitStr, cursor string) PaginationParams {
	limit := 50 // default
	if limitStr != "" {
		// Simple parsing, production would validate more
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
