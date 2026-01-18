package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// AuthorizationHold represents a hold on funds for a payment
type AuthorizationHold struct {
	ID              string
	PaymentIntentID string
	Amount          decimal.Decimal
	Currency        string
	Status          HoldStatus
	Provider        Provider
	AuthCode        *string
	NetworkTxnID    *string
	ExpiresAt       time.Time
	CapturedAmount  decimal.Decimal
	CapturedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsExpired checks if the authorization hold has expired
func (ah *AuthorizationHold) IsExpired() bool {
	return time.Now().After(ah.ExpiresAt)
}

// CanCapture checks if the hold can be captured
func (ah *AuthorizationHold) CanCapture() bool {
	return ah.Status == HoldStatusActive && !ah.IsExpired()
}

// RemainingAmount returns the amount that can still be captured
func (ah *AuthorizationHold) RemainingAmount() decimal.Decimal {
	return ah.Amount.Sub(ah.CapturedAmount)
}

// CanCaptureAmount checks if a specific amount can be captured
func (ah *AuthorizationHold) CanCaptureAmount(amount decimal.Decimal) bool {
	if !ah.CanCapture() {
		return false
	}
	return amount.LessThanOrEqual(ah.RemainingAmount())
}
