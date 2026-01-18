package domain

import (
	"time"
)

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
