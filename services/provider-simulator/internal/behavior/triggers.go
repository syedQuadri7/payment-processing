package behavior

import (
	"strconv"
	"strings"
)

// MetadataTrigger represents a behavior trigger from metadata.
type MetadataTrigger struct {
	DeclineCode   string
	DelayMs       int
	ShouldTimeout bool
}

// MetadataKeys are the recognized metadata keys for behavior triggers.
const (
	MetaKeyDecline = "x-sim-decline"  // Decline code to trigger
	MetaKeyDelay   = "x-sim-delay"    // Delay in milliseconds
	MetaKeyTimeout = "x-sim-timeout"  // Set to "true" for timeout
)

// ParseMetadataTriggers extracts behavior triggers from metadata.
// This allows tests to control payment behavior via API request metadata.
func ParseMetadataTriggers(metadata map[string]string) *MetadataTrigger {
	if metadata == nil {
		return nil
	}

	trigger := &MetadataTrigger{}
	hasAny := false

	// Check for decline code
	if code, ok := metadata[MetaKeyDecline]; ok && code != "" {
		trigger.DeclineCode = normalizeDeclineCode(code)
		hasAny = true
	}

	// Check for delay
	if delayStr, ok := metadata[MetaKeyDelay]; ok && delayStr != "" {
		if delay, err := strconv.Atoi(delayStr); err == nil && delay > 0 {
			trigger.DelayMs = delay
			hasAny = true
		}
	}

	// Check for timeout
	if timeout, ok := metadata[MetaKeyTimeout]; ok {
		if strings.ToLower(timeout) == "true" || timeout == "1" {
			trigger.ShouldTimeout = true
			hasAny = true
		}
	}

	if !hasAny {
		return nil
	}
	return trigger
}

// normalizeDeclineCode converts common decline code variations to canonical form.
func normalizeDeclineCode(code string) string {
	// Convert to uppercase for matching
	upper := strings.ToUpper(strings.TrimSpace(code))

	// Map common variations to canonical codes
	variations := map[string]string{
		// Insufficient funds variations
		"NSF":               "INSUFFICIENT_FUNDS",
		"INSUFFICIENT":      "INSUFFICIENT_FUNDS",
		"NO_FUNDS":          "INSUFFICIENT_FUNDS",

		// Expired card variations
		"EXPIRED":           "CARD_EXPIRED",

		// Generic decline variations
		"DECLINE":           "GENERIC_DECLINE",
		"DECLINED":          "GENERIC_DECLINE",

		// Fraud variations
		"FRAUD":             "FRAUD_SUSPICION",
		"FRAUDULENT":        "FRAUD_SUSPICION",

		// Stolen/lost variations
		"STOLEN":            "STOLEN_CARD",
		"LOST":              "LOST_CARD",

		// CVC variations
		"CVC":               "INVALID_CVC",
		"CVV":               "INVALID_CVC",

		// Do not honor
		"DNH":               "DO_NOT_HONOR",
	}

	if canonical, ok := variations[upper]; ok {
		return canonical
	}

	// If already canonical, return as-is
	if _, ok := CanonicalDeclines[upper]; ok {
		return upper
	}

	// Return as-is for unknown codes
	return upper
}

// MergeBehaviors combines card behavior with metadata triggers.
// Metadata triggers take precedence over card-based triggers.
func MergeBehaviors(cardBehavior CardBehavior, metaTrigger *MetadataTrigger) CardBehavior {
	if metaTrigger == nil {
		return cardBehavior
	}

	result := cardBehavior

	// Metadata overrides card behavior
	if metaTrigger.DeclineCode != "" {
		result.DeclineCode = metaTrigger.DeclineCode
	}
	if metaTrigger.DelayMs > 0 {
		result.DelayMs = metaTrigger.DelayMs
	}
	if metaTrigger.ShouldTimeout {
		result.ShouldTimeout = true
	}

	return result
}

// GetEffectiveBehavior returns the combined behavior from card number and metadata.
func GetEffectiveBehavior(cardNumber string, metadata map[string]string) CardBehavior {
	cardBehavior := GetBehavior(cardNumber)
	metaTrigger := ParseMetadataTriggers(metadata)
	return MergeBehaviors(cardBehavior, metaTrigger)
}
