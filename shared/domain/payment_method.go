package domain

import (
	"time"
)

// PaymentMethod represents a customer's stored payment instrument
type PaymentMethod struct {
	ID          string
	CustomerID  string
	Type        PaymentMethodType
	Provider    Provider
	Token       string
	LastFour    *string
	ExpiryMonth *int
	ExpiryYear  *int
	CardBrand   *string // VISA, MASTERCARD, AMEX, etc.
	IsDefault   bool
	Status      PaymentMethodStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsExpired checks if a card payment method has expired
func (pm *PaymentMethod) IsExpired() bool {
	if pm.Type != PaymentMethodCard || pm.ExpiryMonth == nil || pm.ExpiryYear == nil {
		return false
	}

	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())

	if *pm.ExpiryYear < currentYear {
		return true
	}
	if *pm.ExpiryYear == currentYear && *pm.ExpiryMonth < currentMonth {
		return true
	}
	return false
}

// IsActive checks if the payment method is usable
func (pm *PaymentMethod) IsActive() bool {
	return pm.Status == PaymentMethodStatusActive && !pm.IsExpired()
}
