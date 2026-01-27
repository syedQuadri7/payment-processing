package domain

import (
	"testing"
	"time"
)

func TestPaymentAttempt_IsRetryable(t *testing.T) {
	soft := DeclineTypeSoft
	hard := DeclineTypeHard
	fraud := DeclineTypeFraud
	temporary := DeclineTypeTemporary

	tests := []struct {
		name        string
		status      AttemptStatus
		declineType *DeclineType
		expected    bool
	}{
		{
			name:        "soft decline is retryable",
			status:      AttemptStatusFailed,
			declineType: &soft,
			expected:    true,
		},
		{
			name:        "temporary decline is retryable",
			status:      AttemptStatusFailed,
			declineType: &temporary,
			expected:    true,
		},
		{
			name:        "hard decline is not retryable",
			status:      AttemptStatusFailed,
			declineType: &hard,
			expected:    false,
		},
		{
			name:        "fraud is not retryable",
			status:      AttemptStatusFailed,
			declineType: &fraud,
			expected:    false,
		},
		{
			name:        "succeeded attempt is not retryable",
			status:      AttemptStatusSucceeded,
			declineType: nil,
			expected:    false,
		},
		{
			name:        "pending attempt is not retryable",
			status:      AttemptStatusPending,
			declineType: nil,
			expected:    false,
		},
		{
			name:        "failed without decline type is not retryable",
			status:      AttemptStatusFailed,
			declineType: nil,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa := &PaymentAttempt{
				Status:      tt.status,
				DeclineType: tt.declineType,
			}
			if got := pa.IsRetryable(); got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPaymentAttempt_Duration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		createdAt   time.Time
		completedAt *time.Time
		expectNil   bool
		expectDur   time.Duration
	}{
		{
			name:        "incomplete attempt has no duration",
			createdAt:   now,
			completedAt: nil,
			expectNil:   true,
		},
		{
			name:        "completed attempt has duration",
			createdAt:   now,
			completedAt: timePtr(now.Add(5 * time.Second)),
			expectNil:   false,
			expectDur:   5 * time.Second,
		},
		{
			name:        "fast completion",
			createdAt:   now,
			completedAt: timePtr(now.Add(100 * time.Millisecond)),
			expectNil:   false,
			expectDur:   100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pa := &PaymentAttempt{
				CreatedAt:   tt.createdAt,
				CompletedAt: tt.completedAt,
			}
			got := pa.Duration()
			if tt.expectNil {
				if got != nil {
					t.Errorf("Duration() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Errorf("Duration() = nil, want %v", tt.expectDur)
				return
			}
			if *got != tt.expectDur {
				t.Errorf("Duration() = %v, want %v", *got, tt.expectDur)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
