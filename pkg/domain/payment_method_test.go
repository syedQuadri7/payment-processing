package domain

import (
	"testing"
	"time"
)

func TestPaymentMethod_IsExpired(t *testing.T) {
	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())

	tests := []struct {
		name        string
		methodType  PaymentMethodType
		expiryMonth *int
		expiryYear  *int
		expected    bool
	}{
		{
			name:        "non-card method is never expired",
			methodType:  PaymentMethodACH,
			expiryMonth: nil,
			expiryYear:  nil,
			expected:    false,
		},
		{
			name:        "card with nil expiry is not expired",
			methodType:  PaymentMethodCard,
			expiryMonth: nil,
			expiryYear:  nil,
			expected:    false,
		},
		{
			name:        "card expired last year",
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(12),
			expiryYear:  intPtr(currentYear - 1),
			expected:    true,
		},
		{
			name:        "card expires this month",
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(currentMonth),
			expiryYear:  intPtr(currentYear),
			expected:    false,
		},
		{
			name:        "card expired earlier this year",
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(max(1, currentMonth-1)),
			expiryYear:  intPtr(currentYear),
			expected:    currentMonth > 1, // Only expired if we're past January
		},
		{
			name:        "card valid next year",
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(1),
			expiryYear:  intPtr(currentYear + 1),
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				Type:        tt.methodType,
				ExpiryMonth: tt.expiryMonth,
				ExpiryYear:  tt.expiryYear,
			}
			if got := pm.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentMethod_IsActive(t *testing.T) {
	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())

	tests := []struct {
		name        string
		status      PaymentMethodStatus
		methodType  PaymentMethodType
		expiryMonth *int
		expiryYear  *int
		expected    bool
	}{
		{
			name:        "active non-expired card",
			status:      PaymentMethodStatusActive,
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(12),
			expiryYear:  intPtr(currentYear + 1),
			expected:    true,
		},
		{
			name:        "active expired card",
			status:      PaymentMethodStatusActive,
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(1),
			expiryYear:  intPtr(currentYear - 1),
			expected:    false,
		},
		{
			name:        "deleted status",
			status:      PaymentMethodStatusDeleted,
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(currentMonth),
			expiryYear:  intPtr(currentYear + 1),
			expected:    false,
		},
		{
			name:        "expired status",
			status:      PaymentMethodStatusExpired,
			methodType:  PaymentMethodCard,
			expiryMonth: intPtr(currentMonth),
			expiryYear:  intPtr(currentYear + 1),
			expected:    false,
		},
		{
			name:       "active ACH (no expiry)",
			status:     PaymentMethodStatusActive,
			methodType: PaymentMethodACH,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PaymentMethod{
				Status:      tt.status,
				Type:        tt.methodType,
				ExpiryMonth: tt.expiryMonth,
				ExpiryYear:  tt.expiryYear,
			}
			if got := pm.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
