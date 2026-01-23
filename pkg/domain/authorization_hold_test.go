package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestAuthorizationHold_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired - future",
			expiresAt: time.Now().Add(24 * time.Hour),
			expected:  false,
		},
		{
			name:      "expired - past",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hold := &AuthorizationHold{ExpiresAt: tt.expiresAt}
			if got := hold.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthorizationHold_CanCapture(t *testing.T) {
	futureExpiry := time.Now().Add(24 * time.Hour)
	pastExpiry := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name      string
		status    HoldStatus
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "active and not expired",
			status:    HoldStatusActive,
			expiresAt: futureExpiry,
			expected:  true,
		},
		{
			name:      "active but expired",
			status:    HoldStatusActive,
			expiresAt: pastExpiry,
			expected:  false,
		},
		{
			name:      "already captured",
			status:    HoldStatusCaptured,
			expiresAt: futureExpiry,
			expected:  false,
		},
		{
			name:      "voided",
			status:    HoldStatusVoided,
			expiresAt: futureExpiry,
			expected:  false,
		},
		{
			name:      "expired status",
			status:    HoldStatusExpired,
			expiresAt: futureExpiry,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hold := &AuthorizationHold{
				Status:    tt.status,
				ExpiresAt: tt.expiresAt,
			}
			if got := hold.CanCapture(); got != tt.expected {
				t.Errorf("CanCapture() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthorizationHold_RemainingAmount(t *testing.T) {
	tests := []struct {
		name           string
		amount         string
		capturedAmount string
		expected       string
	}{
		{
			name:           "nothing captured",
			amount:         "100",
			capturedAmount: "0",
			expected:       "100",
		},
		{
			name:           "partial capture",
			amount:         "100",
			capturedAmount: "75",
			expected:       "25",
		},
		{
			name:           "full capture",
			amount:         "100",
			capturedAmount: "100",
			expected:       "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hold := &AuthorizationHold{
				Amount:         decimal.RequireFromString(tt.amount),
				CapturedAmount: decimal.RequireFromString(tt.capturedAmount),
			}
			expected := decimal.RequireFromString(tt.expected)
			if got := hold.RemainingAmount(); !got.Equal(expected) {
				t.Errorf("RemainingAmount() = %v, want %v", got, expected)
			}
		})
	}
}

func TestAuthorizationHold_CanCaptureAmount(t *testing.T) {
	futureExpiry := time.Now().Add(24 * time.Hour)

	tests := []struct {
		name           string
		status         HoldStatus
		expiresAt      time.Time
		amount         string
		capturedAmount string
		captureAmount  string
		expected       bool
	}{
		{
			name:           "can capture full amount",
			status:         HoldStatusActive,
			expiresAt:      futureExpiry,
			amount:         "100",
			capturedAmount: "0",
			captureAmount:  "100",
			expected:       true,
		},
		{
			name:           "can capture partial amount",
			status:         HoldStatusActive,
			expiresAt:      futureExpiry,
			amount:         "100",
			capturedAmount: "0",
			captureAmount:  "50",
			expected:       true,
		},
		{
			name:           "cannot exceed remaining",
			status:         HoldStatusActive,
			expiresAt:      futureExpiry,
			amount:         "100",
			capturedAmount: "75",
			captureAmount:  "50",
			expected:       false,
		},
		{
			name:           "cannot capture if not active",
			status:         HoldStatusCaptured,
			expiresAt:      futureExpiry,
			amount:         "100",
			capturedAmount: "0",
			captureAmount:  "50",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hold := &AuthorizationHold{
				Status:         tt.status,
				ExpiresAt:      tt.expiresAt,
				Amount:         decimal.RequireFromString(tt.amount),
				CapturedAmount: decimal.RequireFromString(tt.capturedAmount),
			}
			captureAmount := decimal.RequireFromString(tt.captureAmount)
			if got := hold.CanCaptureAmount(captureAmount); got != tt.expected {
				t.Errorf("CanCaptureAmount() = %v, want %v", got, tt.expected)
			}
		})
	}
}
