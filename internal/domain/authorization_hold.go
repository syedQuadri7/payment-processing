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
	VoidedAt        *time.Time
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

// CanVoid checks if the hold can be voided
func (ah *AuthorizationHold) CanVoid() bool {
	return ah.Status == HoldStatusActive && !ah.IsExpired()
}

// IsFullyCaptured returns true if the entire authorized amount has been captured
func (ah *AuthorizationHold) IsFullyCaptured() bool {
	return ah.CapturedAmount.Equal(ah.Amount)
}

// IsPartiallyCaptured returns true if some but not all of the amount has been captured
func (ah *AuthorizationHold) IsPartiallyCaptured() bool {
	return ah.CapturedAmount.GreaterThan(decimal.Zero) &&
		ah.CapturedAmount.LessThan(ah.Amount)
}

// TimeUntilExpiry returns the duration until the hold expires
func (ah *AuthorizationHold) TimeUntilExpiry() time.Duration {
	return time.Until(ah.ExpiresAt)
}

// IsExpiringSoon returns true if the hold will expire within the given duration
func (ah *AuthorizationHold) IsExpiringSoon(within time.Duration) bool {
	return ah.Status == HoldStatusActive && ah.TimeUntilExpiry() <= within
}

// Capture marks the hold as captured with the given amount
func (ah *AuthorizationHold) Capture(amount decimal.Decimal) error {
	if !ah.CanCaptureAmount(amount) {
		return ErrCannotCapture
	}

	now := time.Now()
	ah.CapturedAmount = ah.CapturedAmount.Add(amount)
	ah.CapturedAt = &now
	ah.UpdatedAt = now

	// Mark as captured if fully captured
	if ah.IsFullyCaptured() {
		ah.Status = HoldStatusCaptured
	}

	return nil
}

// Void marks the hold as voided
func (ah *AuthorizationHold) Void() error {
	if !ah.CanVoid() {
		return ErrCannotVoid
	}

	now := time.Now()
	ah.Status = HoldStatusVoided
	ah.VoidedAt = &now
	ah.UpdatedAt = now

	return nil
}

// Expire marks the hold as expired
func (ah *AuthorizationHold) Expire() error {
	if ah.Status != HoldStatusActive {
		return ErrHoldNotActive
	}

	ah.Status = HoldStatusExpired
	ah.UpdatedAt = time.Now()

	return nil
}

// CanTransitionTo checks if a status transition is valid for the hold
func (ah *AuthorizationHold) CanTransitionTo(newStatus HoldStatus) bool {
	validTransitions := map[HoldStatus][]HoldStatus{
		HoldStatusActive: {
			HoldStatusCaptured,
			HoldStatusVoided,
			HoldStatusExpired,
		},
		HoldStatusCaptured: {},
		HoldStatusVoided:   {},
		HoldStatusExpired:  {},
	}

	allowed, ok := validTransitions[ah.Status]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}

// Authorization hold errors
var (
	ErrCannotCapture = &HoldError{Message: "cannot capture: hold is not active or has expired"}
	ErrCannotVoid    = &HoldError{Message: "cannot void: hold is not active or has expired"}
	ErrHoldNotActive = &HoldError{Message: "hold is not active"}
)

// HoldError represents an authorization hold error
type HoldError struct {
	Message string
}

func (e *HoldError) Error() string {
	return e.Message
}
