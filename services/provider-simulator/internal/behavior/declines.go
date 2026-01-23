package behavior

// DeclineType categorizes decline codes by retry behavior.
type DeclineType string

const (
	DeclineTypeSoft  DeclineType = "soft"  // Retry may succeed
	DeclineTypeHard  DeclineType = "hard"  // Don't retry
	DeclineTypeFraud DeclineType = "fraud" // Don't retry, fraud concern
)

// CanonicalDecline represents a normalized decline code.
type CanonicalDecline struct {
	Code        string
	Type        DeclineType
	Description string
}

// CanonicalDeclines are the 18 standard decline codes used across all providers.
var CanonicalDeclines = map[string]CanonicalDecline{
	// Soft declines - retry may succeed
	"INSUFFICIENT_FUNDS": {
		Code:        "INSUFFICIENT_FUNDS",
		Type:        DeclineTypeSoft,
		Description: "Card has insufficient funds",
	},
	"OVER_LIMIT": {
		Code:        "OVER_LIMIT",
		Type:        DeclineTypeSoft,
		Description: "Card is over its credit limit",
	},
	"GENERIC_DECLINE": {
		Code:        "GENERIC_DECLINE",
		Type:        DeclineTypeSoft,
		Description: "Card was declined for an unspecified reason",
	},
	"DO_NOT_HONOR": {
		Code:        "DO_NOT_HONOR",
		Type:        DeclineTypeSoft,
		Description: "Issuer declined without specific reason",
	},
	"TRY_AGAIN_LATER": {
		Code:        "TRY_AGAIN_LATER",
		Type:        DeclineTypeSoft,
		Description: "Temporary issue, retry later",
	},
	"PROCESSOR_ERROR": {
		Code:        "PROCESSOR_ERROR",
		Type:        DeclineTypeSoft,
		Description: "Processing error occurred",
	},

	// Hard declines - don't retry
	"CARD_EXPIRED": {
		Code:        "CARD_EXPIRED",
		Type:        DeclineTypeHard,
		Description: "Card has expired",
	},
	"INVALID_CARD_NUMBER": {
		Code:        "INVALID_CARD_NUMBER",
		Type:        DeclineTypeHard,
		Description: "Card number is invalid",
	},
	"INVALID_CVC": {
		Code:        "INVALID_CVC",
		Type:        DeclineTypeHard,
		Description: "CVC/CVV is incorrect",
	},
	"INVALID_EXPIRY": {
		Code:        "INVALID_EXPIRY",
		Type:        DeclineTypeHard,
		Description: "Expiry date is invalid",
	},
	"INVALID_ACCOUNT": {
		Code:        "INVALID_ACCOUNT",
		Type:        DeclineTypeHard,
		Description: "Account number is invalid",
	},
	"ACCOUNT_CLOSED": {
		Code:        "ACCOUNT_CLOSED",
		Type:        DeclineTypeHard,
		Description: "Account has been closed",
	},
	"ACCOUNT_RESTRICTED": {
		Code:        "ACCOUNT_RESTRICTED",
		Type:        DeclineTypeHard,
		Description: "Account has restrictions",
	},
	"NOT_PERMITTED": {
		Code:        "NOT_PERMITTED",
		Type:        DeclineTypeHard,
		Description: "Transaction type not permitted",
	},

	// Fraud declines - don't retry
	"FRAUD_SUSPICION": {
		Code:        "FRAUD_SUSPICION",
		Type:        DeclineTypeFraud,
		Description: "Suspected fraudulent transaction",
	},
	"STOLEN_CARD": {
		Code:        "STOLEN_CARD",
		Type:        DeclineTypeFraud,
		Description: "Card reported stolen",
	},
	"LOST_CARD": {
		Code:        "LOST_CARD",
		Type:        DeclineTypeFraud,
		Description: "Card reported lost",
	},
	"PICKUP_CARD": {
		Code:        "PICKUP_CARD",
		Type:        DeclineTypeFraud,
		Description: "Merchant should retain card",
	},
}

// ProviderDeclineMapping maps canonical codes to provider-specific codes.
type ProviderDeclineMapping struct {
	Stripe string
	Adyen  string
	PayPal string
}

// DeclineMappings maps canonical decline codes to provider-specific codes.
var DeclineMappings = map[string]ProviderDeclineMapping{
	// Soft declines
	"INSUFFICIENT_FUNDS": {
		Stripe: "insufficient_funds",
		Adyen:  "Refused:51",
		PayPal: "INSTRUMENT_DECLINED",
	},
	"OVER_LIMIT": {
		Stripe: "card_declined",
		Adyen:  "Refused:61",
		PayPal: "INSTRUMENT_DECLINED",
	},
	"GENERIC_DECLINE": {
		Stripe: "card_declined",
		Adyen:  "Refused:05",
		PayPal: "INSTRUMENT_DECLINED",
	},
	"DO_NOT_HONOR": {
		Stripe: "do_not_honor",
		Adyen:  "Refused:05",
		PayPal: "INSTRUMENT_DECLINED",
	},
	"TRY_AGAIN_LATER": {
		Stripe: "try_again_later",
		Adyen:  "Refused:96",
		PayPal: "INTERNAL_SERVICE_ERROR",
	},
	"PROCESSOR_ERROR": {
		Stripe: "processing_error",
		Adyen:  "Refused:96",
		PayPal: "INTERNAL_SERVICE_ERROR",
	},

	// Hard declines
	"CARD_EXPIRED": {
		Stripe: "expired_card",
		Adyen:  "Refused:54",
		PayPal: "CREDIT_CARD_EXPIRED",
	},
	"INVALID_CARD_NUMBER": {
		Stripe: "invalid_number",
		Adyen:  "Refused:14",
		PayPal: "INVALID_CARD_NUMBER",
	},
	"INVALID_CVC": {
		Stripe: "incorrect_cvc",
		Adyen:  "CVC Declined",
		PayPal: "INVALID_SECURITY_CODE",
	},
	"INVALID_EXPIRY": {
		Stripe: "invalid_expiry_month",
		Adyen:  "Refused:80",
		PayPal: "CARD_EXPIRED",
	},
	"INVALID_ACCOUNT": {
		Stripe: "invalid_account",
		Adyen:  "Refused:12",
		PayPal: "INVALID_ACCOUNT",
	},
	"ACCOUNT_CLOSED": {
		Stripe: "card_declined",
		Adyen:  "Refused:46",
		PayPal: "ACCOUNT_CLOSED",
	},
	"ACCOUNT_RESTRICTED": {
		Stripe: "card_declined",
		Adyen:  "Refused:36",
		PayPal: "ACCOUNT_RESTRICTED",
	},
	"NOT_PERMITTED": {
		Stripe: "card_declined",
		Adyen:  "Refused:57",
		PayPal: "TRANSACTION_REFUSED",
	},

	// Fraud declines
	"FRAUD_SUSPICION": {
		Stripe: "fraudulent",
		Adyen:  "Refused:59",
		PayPal: "TRANSACTION_REFUSED",
	},
	"STOLEN_CARD": {
		Stripe: "stolen_card",
		Adyen:  "Refused:43",
		PayPal: "CARD_STOLEN",
	},
	"LOST_CARD": {
		Stripe: "lost_card",
		Adyen:  "Refused:41",
		PayPal: "CARD_LOST",
	},
	"PICKUP_CARD": {
		Stripe: "pickup_card",
		Adyen:  "Refused:04",
		PayPal: "CARD_RESTRICTED",
	},
}

// GetStripeDeclineCode returns the Stripe-specific decline code.
func GetStripeDeclineCode(canonical string) string {
	if mapping, ok := DeclineMappings[canonical]; ok {
		return mapping.Stripe
	}
	return "card_declined"
}

// GetAdyenDeclineCode returns the Adyen-specific decline code.
func GetAdyenDeclineCode(canonical string) string {
	if mapping, ok := DeclineMappings[canonical]; ok {
		return mapping.Adyen
	}
	return "Refused:05"
}

// GetPayPalDeclineCode returns the PayPal-specific decline code.
func GetPayPalDeclineCode(canonical string) string {
	if mapping, ok := DeclineMappings[canonical]; ok {
		return mapping.PayPal
	}
	return "INSTRUMENT_DECLINED"
}

// GetDeclineType returns the type of a canonical decline code.
func GetDeclineType(canonical string) DeclineType {
	if decline, ok := CanonicalDeclines[canonical]; ok {
		return decline.Type
	}
	return DeclineTypeSoft
}

// IsRetryable returns true if the decline code indicates a retry may succeed.
func IsRetryable(canonical string) bool {
	return GetDeclineType(canonical) == DeclineTypeSoft
}
