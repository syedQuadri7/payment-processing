// Package behavior provides test card behavior triggers for the provider simulator.
package behavior

// CardBehavior represents the behavior triggered by a test card number.
type CardBehavior struct {
	CardNumber    string
	Description   string
	DeclineCode   string // Empty for success
	DelayMs       int    // Response delay
	ShouldTimeout bool   // Simulate timeout
}

// TestCards maps card numbers to their behaviors.
// These are industry-standard test card numbers used by Stripe, Adyen, and others.
var TestCards = map[string]CardBehavior{
	// Success cards
	"4242424242424242": {
		CardNumber:  "4242424242424242",
		Description: "Success",
	},
	"4000000000003220": {
		CardNumber:  "4000000000003220",
		Description: "Success with 5s delay",
		DelayMs:     5000,
	},

	// Generic decline
	"4000000000000002": {
		CardNumber:  "4000000000000002",
		Description: "Generic decline",
		DeclineCode: "GENERIC_DECLINE",
	},

	// Insufficient funds
	"4000000000009995": {
		CardNumber:  "4000000000009995",
		Description: "Insufficient funds",
		DeclineCode: "INSUFFICIENT_FUNDS",
	},

	// Expired card
	"4000000000000069": {
		CardNumber:  "4000000000000069",
		Description: "Expired card",
		DeclineCode: "CARD_EXPIRED",
	},

	// Lost card
	"4000000000009987": {
		CardNumber:  "4000000000009987",
		Description: "Lost card",
		DeclineCode: "LOST_CARD",
	},

	// Stolen card
	"4000000000009979": {
		CardNumber:  "4000000000009979",
		Description: "Stolen card",
		DeclineCode: "STOLEN_CARD",
	},

	// Invalid CVC
	"4000000000000127": {
		CardNumber:  "4000000000000127",
		Description: "Invalid CVC",
		DeclineCode: "INVALID_CVC",
	},

	// Timeout
	"4000000000000341": {
		CardNumber:    "4000000000000341",
		Description:   "Timeout (30s)",
		ShouldTimeout: true,
	},

	// Over limit
	"4000000000009010": {
		CardNumber:  "4000000000009010",
		Description: "Over limit",
		DeclineCode: "OVER_LIMIT",
	},

	// Do not honor
	"4000000000000044": {
		CardNumber:  "4000000000000044",
		Description: "Do not honor",
		DeclineCode: "DO_NOT_HONOR",
	},

	// Fraud suspicion
	"4100000000000019": {
		CardNumber:  "4100000000000019",
		Description: "Fraud suspicion",
		DeclineCode: "FRAUD_SUSPICION",
	},

	// Invalid card number
	"4000000000000101": {
		CardNumber:  "4000000000000101",
		Description: "Invalid card number",
		DeclineCode: "INVALID_CARD_NUMBER",
	},

	// Invalid expiry
	"4000000000000119": {
		CardNumber:  "4000000000000119",
		Description: "Invalid expiry",
		DeclineCode: "INVALID_EXPIRY",
	},

	// Invalid account
	"4000000000000036": {
		CardNumber:  "4000000000000036",
		Description: "Invalid account",
		DeclineCode: "INVALID_ACCOUNT",
	},

	// Account closed
	"4000000000000077": {
		CardNumber:  "4000000000000077",
		Description: "Account closed",
		DeclineCode: "ACCOUNT_CLOSED",
	},

	// Account restricted
	"4000000000000085": {
		CardNumber:  "4000000000000085",
		Description: "Account restricted",
		DeclineCode: "ACCOUNT_RESTRICTED",
	},

	// Not permitted
	"4000000000000028": {
		CardNumber:  "4000000000000028",
		Description: "Not permitted",
		DeclineCode: "NOT_PERMITTED",
	},

	// Try again later
	"4000000000000051": {
		CardNumber:  "4000000000000051",
		Description: "Try again later",
		DeclineCode: "TRY_AGAIN_LATER",
	},

	// Processor error
	"4000000000000093": {
		CardNumber:  "4000000000000093",
		Description: "Processor error",
		DeclineCode: "PROCESSOR_ERROR",
	},

	// Pickup card
	"4000000000000135": {
		CardNumber:  "4000000000000135",
		Description: "Pickup card",
		DeclineCode: "PICKUP_CARD",
	},
}

// GetBehavior returns the behavior for a card number, or a default success behavior.
func GetBehavior(cardNumber string) CardBehavior {
	if behavior, ok := TestCards[cardNumber]; ok {
		return behavior
	}
	// Default to success for unknown cards
	return CardBehavior{
		CardNumber:  cardNumber,
		Description: "Success (default)",
	}
}

// IsSuccessCard returns true if the card number should succeed.
func IsSuccessCard(cardNumber string) bool {
	behavior := GetBehavior(cardNumber)
	return behavior.DeclineCode == "" && !behavior.ShouldTimeout
}
