package config

// DeclineCode represents a decline code mapping between providers.
type DeclineCode struct {
	Provider  string
	Code      string
	Canonical string
	Type      DeclineType
}

// DeclineType categorizes the decline.
type DeclineType string

const (
	DeclineTypeSoft  DeclineType = "SOFT"
	DeclineTypeHard  DeclineType = "HARD"
	DeclineTypeFraud DeclineType = "FRAUD"
)

// StripeDeclineCodes maps Stripe decline reasons to canonical codes.
var StripeDeclineCodes = map[string]DeclineCode{
	"insufficient_funds": {
		Provider:  "stripe",
		Code:      "insufficient_funds",
		Canonical: "INSUFFICIENT_FUNDS",
		Type:      DeclineTypeSoft,
	},
	"expired_card": {
		Provider:  "stripe",
		Code:      "expired_card",
		Canonical: "CARD_EXPIRED",
		Type:      DeclineTypeHard,
	},
	"fraudulent": {
		Provider:  "stripe",
		Code:      "fraudulent",
		Canonical: "FRAUD_SUSPICION",
		Type:      DeclineTypeFraud,
	},
	"card_declined": {
		Provider:  "stripe",
		Code:      "card_declined",
		Canonical: "GENERIC_DECLINE",
		Type:      DeclineTypeSoft,
	},
	"processing_error": {
		Provider:  "stripe",
		Code:      "processing_error",
		Canonical: "PROCESSING_ERROR",
		Type:      DeclineTypeSoft,
	},
}

// AdyenDeclineCodes maps Adyen decline reasons to canonical codes.
var AdyenDeclineCodes = map[string]DeclineCode{
	"Refused:51": {
		Provider:  "adyen",
		Code:      "Refused:51",
		Canonical: "INSUFFICIENT_FUNDS",
		Type:      DeclineTypeSoft,
	},
	"Refused:33": {
		Provider:  "adyen",
		Code:      "Refused:33",
		Canonical: "CARD_EXPIRED",
		Type:      DeclineTypeHard,
	},
	"Refused:59": {
		Provider:  "adyen",
		Code:      "Refused:59",
		Canonical: "FRAUD_SUSPICION",
		Type:      DeclineTypeFraud,
	},
	"Refused:05": {
		Provider:  "adyen",
		Code:      "Refused:05",
		Canonical: "DO_NOT_HONOR",
		Type:      DeclineTypeSoft,
	},
	"Refused:14": {
		Provider:  "adyen",
		Code:      "Refused:14",
		Canonical: "INVALID_CARD_NUMBER",
		Type:      DeclineTypeHard,
	},
}

// PayPalDeclineCodes maps PayPal decline reasons to canonical codes.
var PayPalDeclineCodes = map[string]DeclineCode{
	"INSUFFICIENT_FUNDS": {
		Provider:  "paypal",
		Code:      "INSUFFICIENT_FUNDS",
		Canonical: "INSUFFICIENT_FUNDS",
		Type:      DeclineTypeSoft,
	},
	"CREDIT_CARD_EXPIRED": {
		Provider:  "paypal",
		Code:      "CREDIT_CARD_EXPIRED",
		Canonical: "CARD_EXPIRED",
		Type:      DeclineTypeHard,
	},
	"PAYMENT_DENIED": {
		Provider:  "paypal",
		Code:      "PAYMENT_DENIED",
		Canonical: "GENERIC_DECLINE",
		Type:      DeclineTypeSoft,
	},
	"TRANSACTION_REFUSED": {
		Provider:  "paypal",
		Code:      "TRANSACTION_REFUSED",
		Canonical: "FRAUD_SUSPICION",
		Type:      DeclineTypeFraud,
	},
}
