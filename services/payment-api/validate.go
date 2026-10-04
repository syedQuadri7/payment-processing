package server

import (
	"github.com/shopspring/decimal"

	"payment-processing/shared/domain"
)

const (
	MaxMetadataKeys        = 50
	MaxMetadataKeyLength   = 40
	MaxMetadataValueLength = 500
	MaxCustomerIDLength    = 255
	MinCustomerIDLength    = 1
)

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

	if r.CustomerID == "" {
		return NewValidationError("customer_id", "customer_id is required")
	}
	if len(r.CustomerID) < MinCustomerIDLength || len(r.CustomerID) > MaxCustomerIDLength {
		return NewValidationError("customer_id", "customer_id must be between 1 and 255 characters")
	}
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

	if err := validateMetadata(r.Metadata); err != nil {
		return err
	}

	return nil
}

func (r *CapturePaymentRequest) Validate() *APIError {
	if r.Amount != nil && r.Amount.LessThanOrEqual(decimal.Zero) {
		return NewValidationError("amount", "amount must be greater than zero")
	}
	return nil
}

func (r *AttachPaymentMethodRequest) Validate() *APIError {
	if r.PaymentMethodID == "" {
		return NewValidationError("payment_method_id", "payment_method_id is required")
	}
	return nil
}

func isAllowedIDChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

func validateMetadata(metadata map[string]string) *APIError {
	if metadata == nil {
		return nil
	}

	if len(metadata) > MaxMetadataKeys {
		return NewValidationError("metadata", "metadata cannot have more than 50 keys")
	}

	for key, value := range metadata {
		if key == "" {
			return NewValidationError("metadata", "metadata keys cannot be empty")
		}
		if len(key) > MaxMetadataKeyLength {
			return NewValidationError("metadata", "metadata key '"+key+"' exceeds maximum length of 40 characters")
		}
		for _, c := range key {
			if !isAllowedIDChar(c) {
				return NewValidationError("metadata", "metadata key '"+key+"' contains invalid characters (allowed: a-z, A-Z, 0-9, _, -)")
			}
		}
		if len(value) > MaxMetadataValueLength {
			return NewValidationError("metadata", "metadata value for key '"+key+"' exceeds maximum length of 500 characters")
		}
	}

	return nil
}
