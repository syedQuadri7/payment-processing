package domain

import (
	"testing"
)

func TestCanonicalDeclineCode_IsRetryEligible(t *testing.T) {
	tests := []struct {
		code     CanonicalDeclineCode
		expected bool
	}{
		// Soft declines - retry eligible
		{DeclineInsufficientFunds, true},
		{DeclineOverLimit, true},
		{DeclineGenericDecline, true},
		{DeclineDoNotHonor, true},
		{DeclineTryAgainLater, true},
		{DeclineProcessorError, true},

		// Hard declines - not retry eligible
		{DeclineCardExpired, false},
		{DeclineInvalidNumber, false},
		{DeclineInvalidCVC, false},
		{DeclineAccountClosed, false},

		// Fraud declines - not retry eligible
		{DeclineFraudSuspicion, false},
		{DeclineStolenCard, false},
		{DeclineLostCard, false},

		// Unknown code defaults to retry eligible
		{CanonicalDeclineCode("UNKNOWN_CODE"), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.IsRetryEligible(); got != tt.expected {
				t.Errorf("IsRetryEligible() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCanonicalDeclineCode_GetDeclineType(t *testing.T) {
	tests := []struct {
		code     CanonicalDeclineCode
		expected DeclineType
	}{
		{DeclineInsufficientFunds, DeclineTypeSoft},
		{DeclineGenericDecline, DeclineTypeSoft},
		{DeclineTryAgainLater, DeclineTypeTemporary},
		{DeclineProcessorError, DeclineTypeTemporary},
		{DeclineCardExpired, DeclineTypeHard},
		{DeclineInvalidNumber, DeclineTypeHard},
		{DeclineFraudSuspicion, DeclineTypeFraud},
		{DeclineStolenCard, DeclineTypeFraud},
		// Unknown code defaults to soft
		{CanonicalDeclineCode("UNKNOWN"), DeclineTypeSoft},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.GetDeclineType(); got != tt.expected {
				t.Errorf("GetDeclineType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetDeclineInfo(t *testing.T) {
	tests := []struct {
		code      CanonicalDeclineCode
		expectOK  bool
		checkType DeclineType
	}{
		{DeclineInsufficientFunds, true, DeclineTypeSoft},
		{DeclineCardExpired, true, DeclineTypeHard},
		{DeclineFraudSuspicion, true, DeclineTypeFraud},
		{CanonicalDeclineCode("UNKNOWN"), false, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			info, ok := GetDeclineInfo(tt.code)
			if ok != tt.expectOK {
				t.Errorf("GetDeclineInfo() ok = %v, want %v", ok, tt.expectOK)
			}
			if ok && info.Type != tt.checkType {
				t.Errorf("GetDeclineInfo().Type = %v, want %v", info.Type, tt.checkType)
			}
		})
	}
}

func TestDeclineCodeMapping_ToCanonicalDeclineCode(t *testing.T) {
	mapping := &DeclineCodeMapping{
		CanonicalCode: "INSUFFICIENT_FUNDS",
	}

	got := mapping.ToCanonicalDeclineCode()
	if got != DeclineInsufficientFunds {
		t.Errorf("ToCanonicalDeclineCode() = %v, want %v", got, DeclineInsufficientFunds)
	}
}
