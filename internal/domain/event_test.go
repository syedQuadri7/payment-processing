package domain

import (
	"testing"
)

func TestCanonicalEvent_IsSuccess(t *testing.T) {
	tests := []struct {
		eventType CanonicalEventType
		expected  bool
	}{
		{EventAuthorizationSucceeded, true},
		{EventCaptureSucceeded, true},
		{EventVoidSucceeded, true},
		{EventRefundSucceeded, true},
		{EventDisputeClosed, true},
		{EventAuthorizationFailed, false},
		{EventCaptureFailed, false},
		{EventDisputeOpened, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			event := &CanonicalEvent{Type: tt.eventType}
			if got := event.IsSuccess(); got != tt.expected {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCanonicalEvent_IsFailure(t *testing.T) {
	tests := []struct {
		eventType CanonicalEventType
		expected  bool
	}{
		{EventAuthorizationFailed, true},
		{EventCaptureFailed, true},
		{EventVoidFailed, true},
		{EventRefundFailed, true},
		{EventAuthorizationSucceeded, false},
		{EventDisputeOpened, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			event := &CanonicalEvent{Type: tt.eventType}
			if got := event.IsFailure(); got != tt.expected {
				t.Errorf("IsFailure() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCanonicalEvent_IsRetryable(t *testing.T) {
	soft := DeclineTypeSoft
	hard := DeclineTypeHard
	temporary := DeclineTypeTemporary

	tests := []struct {
		name        string
		eventType   CanonicalEventType
		declineType *DeclineType
		expected    bool
	}{
		{"soft decline is retryable", EventAuthorizationFailed, &soft, true},
		{"temporary decline is retryable", EventAuthorizationFailed, &temporary, true},
		{"hard decline not retryable", EventAuthorizationFailed, &hard, false},
		{"success not retryable", EventAuthorizationSucceeded, nil, false},
		{"failure without decline not retryable", EventAuthorizationFailed, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &CanonicalEvent{Type: tt.eventType, DeclineType: tt.declineType}
			if got := event.IsRetryable(); got != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMapProviderEvent(t *testing.T) {
	tests := []struct {
		provider      Provider
		providerEvent string
		expected      CanonicalEventType
		found         bool
	}{
		{ProviderStripe, "payment_intent.succeeded", EventAuthorizationSucceeded, true},
		{ProviderStripe, "charge.captured", EventCaptureSucceeded, true},
		{ProviderStripe, "charge.dispute.created", EventDisputeOpened, true},
		{ProviderAdyen, "AUTHORISATION", EventAuthorizationSucceeded, true},
		{ProviderAdyen, "CAPTURE", EventCaptureSucceeded, true},
		{ProviderPayPal, "PAYMENT.CAPTURE.COMPLETED", EventCaptureSucceeded, true},
		{ProviderStripe, "unknown_event", "", false},
		{"UNKNOWN", "some_event", "", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider)+"_"+tt.providerEvent, func(t *testing.T) {
			got, found := MapProviderEvent(tt.provider, tt.providerEvent)
			if found != tt.found {
				t.Errorf("MapProviderEvent() found = %v, want %v", found, tt.found)
			}
			if got != tt.expected {
				t.Errorf("MapProviderEvent() = %v, want %v", got, tt.expected)
			}
		})
	}
}
