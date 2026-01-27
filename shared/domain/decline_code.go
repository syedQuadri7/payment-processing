package domain

import (
	"time"
)

// CanonicalDeclineCode represents normalized decline codes across all providers
type CanonicalDeclineCode string

// Soft declines - retry eligible
const (
	DeclineInsufficientFunds CanonicalDeclineCode = "INSUFFICIENT_FUNDS"
	DeclineOverLimit         CanonicalDeclineCode = "OVER_LIMIT"
	DeclineGenericDecline    CanonicalDeclineCode = "GENERIC_DECLINE"
	DeclineDoNotHonor        CanonicalDeclineCode = "DO_NOT_HONOR"
	DeclineTryAgainLater     CanonicalDeclineCode = "TRY_AGAIN_LATER"
	DeclineProcessorError    CanonicalDeclineCode = "PROCESSOR_ERROR"
)

// Hard declines - not retry eligible
const (
	DeclineCardExpired      CanonicalDeclineCode = "CARD_EXPIRED"
	DeclineInvalidNumber    CanonicalDeclineCode = "INVALID_CARD_NUMBER"
	DeclineInvalidCVC       CanonicalDeclineCode = "INVALID_CVC"
	DeclineInvalidExpiry    CanonicalDeclineCode = "INVALID_EXPIRY"
	DeclineInvalidAccount   CanonicalDeclineCode = "INVALID_ACCOUNT"
	DeclineAccountClosed    CanonicalDeclineCode = "ACCOUNT_CLOSED"
	DeclineAccountRestricted CanonicalDeclineCode = "ACCOUNT_RESTRICTED"
	DeclineNotPermitted     CanonicalDeclineCode = "NOT_PERMITTED"
)

// Fraud declines - not retry eligible, requires review
const (
	DeclineFraudSuspicion CanonicalDeclineCode = "FRAUD_SUSPICION"
	DeclineStolenCard     CanonicalDeclineCode = "STOLEN_CARD"
	DeclineLostCard       CanonicalDeclineCode = "LOST_CARD"
	DeclinePickupCard     CanonicalDeclineCode = "PICKUP_CARD"
)

// DeclineCodeInfo provides metadata about a canonical decline code
type DeclineCodeInfo struct {
	Code            CanonicalDeclineCode
	Type            DeclineType
	RetryEligible   bool
	Description     string
	SuggestedAction string
}

// CanonicalDeclineCodes maps canonical codes to their metadata
var CanonicalDeclineCodes = map[CanonicalDeclineCode]DeclineCodeInfo{
	// Soft declines
	DeclineInsufficientFunds: {
		Code:            DeclineInsufficientFunds,
		Type:            DeclineTypeSoft,
		RetryEligible:   true,
		Description:     "The card has insufficient funds",
		SuggestedAction: "Retry after a delay or request alternate payment method",
	},
	DeclineOverLimit: {
		Code:            DeclineOverLimit,
		Type:            DeclineTypeSoft,
		RetryEligible:   true,
		Description:     "The transaction exceeds the card's limit",
		SuggestedAction: "Retry with a smaller amount or request alternate payment method",
	},
	DeclineGenericDecline: {
		Code:            DeclineGenericDecline,
		Type:            DeclineTypeSoft,
		RetryEligible:   true,
		Description:     "The card was declined for an unspecified reason",
		SuggestedAction: "Retry or request alternate payment method",
	},
	DeclineDoNotHonor: {
		Code:            DeclineDoNotHonor,
		Type:            DeclineTypeSoft,
		RetryEligible:   true,
		Description:     "The issuer declined without specifying a reason",
		SuggestedAction: "Retry or request alternate payment method",
	},
	DeclineTryAgainLater: {
		Code:            DeclineTryAgainLater,
		Type:            DeclineTypeTemporary,
		RetryEligible:   true,
		Description:     "Temporary issue, try again later",
		SuggestedAction: "Retry after a short delay",
	},
	DeclineProcessorError: {
		Code:            DeclineProcessorError,
		Type:            DeclineTypeTemporary,
		RetryEligible:   true,
		Description:     "Processing error occurred",
		SuggestedAction: "Retry after a short delay",
	},

	// Hard declines
	DeclineCardExpired: {
		Code:            DeclineCardExpired,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "The card has expired",
		SuggestedAction: "Request updated card details",
	},
	DeclineInvalidNumber: {
		Code:            DeclineInvalidNumber,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "The card number is invalid",
		SuggestedAction: "Request correct card details",
	},
	DeclineInvalidCVC: {
		Code:            DeclineInvalidCVC,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "The CVC is incorrect",
		SuggestedAction: "Request customer to re-enter CVC",
	},
	DeclineInvalidExpiry: {
		Code:            DeclineInvalidExpiry,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "The expiration date is invalid",
		SuggestedAction: "Request correct expiration date",
	},
	DeclineInvalidAccount: {
		Code:            DeclineInvalidAccount,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "The account is invalid",
		SuggestedAction: "Request alternate payment method",
	},
	DeclineAccountClosed: {
		Code:            DeclineAccountClosed,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "The account has been closed",
		SuggestedAction: "Request alternate payment method",
	},
	DeclineAccountRestricted: {
		Code:            DeclineAccountRestricted,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "The account is restricted",
		SuggestedAction: "Request alternate payment method",
	},
	DeclineNotPermitted: {
		Code:            DeclineNotPermitted,
		Type:            DeclineTypeHard,
		RetryEligible:   false,
		Description:     "Transaction not permitted",
		SuggestedAction: "Request alternate payment method",
	},

	// Fraud declines
	DeclineFraudSuspicion: {
		Code:            DeclineFraudSuspicion,
		Type:            DeclineTypeFraud,
		RetryEligible:   false,
		Description:     "The payment was flagged as potentially fraudulent",
		SuggestedAction: "Do not retry, investigate",
	},
	DeclineStolenCard: {
		Code:            DeclineStolenCard,
		Type:            DeclineTypeFraud,
		RetryEligible:   false,
		Description:     "The card has been reported stolen",
		SuggestedAction: "Do not retry, report to fraud team",
	},
	DeclineLostCard: {
		Code:            DeclineLostCard,
		Type:            DeclineTypeFraud,
		RetryEligible:   false,
		Description:     "The card has been reported lost",
		SuggestedAction: "Do not retry, report to fraud team",
	},
	DeclinePickupCard: {
		Code:            DeclinePickupCard,
		Type:            DeclineTypeFraud,
		RetryEligible:   false,
		Description:     "Issuer requested the card be retained",
		SuggestedAction: "Do not retry, report to fraud team",
	},
}

// GetDeclineInfo returns metadata for a canonical decline code
func GetDeclineInfo(code CanonicalDeclineCode) (DeclineCodeInfo, bool) {
	info, ok := CanonicalDeclineCodes[code]
	return info, ok
}

// IsRetryEligible checks if a decline code allows retry
func (c CanonicalDeclineCode) IsRetryEligible() bool {
	if info, ok := CanonicalDeclineCodes[c]; ok {
		return info.RetryEligible
	}
	// Unknown codes default to soft decline (retry eligible)
	return true
}

// GetDeclineType returns the decline type for a canonical code
func (c CanonicalDeclineCode) GetDeclineType() DeclineType {
	if info, ok := CanonicalDeclineCodes[c]; ok {
		return info.Type
	}
	// Unknown codes default to soft decline
	return DeclineTypeSoft
}

// DeclineCodeMapping maps provider-specific decline codes to canonical codes
type DeclineCodeMapping struct {
	ID              string
	Provider        Provider
	ProviderCode    string
	CanonicalCode   string
	DeclineType     DeclineType
	Description     *string
	RetryEligible   bool
	SuggestedAction *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ToCanonicalDeclineCode converts the mapping's canonical code string to the typed constant
func (m *DeclineCodeMapping) ToCanonicalDeclineCode() CanonicalDeclineCode {
	return CanonicalDeclineCode(m.CanonicalCode)
}
