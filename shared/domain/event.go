package domain

// CanonicalEventType represents normalized webhook event types
// All provider-specific events are mapped to these canonical types
type CanonicalEventType string

const (
	// Authorization events
	EventAuthorizationSucceeded CanonicalEventType = "AUTHORIZATION_SUCCEEDED"
	EventAuthorizationFailed    CanonicalEventType = "AUTHORIZATION_FAILED"

	// Capture events
	EventCaptureSucceeded CanonicalEventType = "CAPTURE_SUCCEEDED"
	EventCaptureFailed    CanonicalEventType = "CAPTURE_FAILED"

	// Void events
	EventVoidSucceeded CanonicalEventType = "VOID_SUCCEEDED"
	EventVoidFailed    CanonicalEventType = "VOID_FAILED"

	// Refund events
	EventRefundSucceeded CanonicalEventType = "REFUND_SUCCEEDED"
	EventRefundFailed    CanonicalEventType = "REFUND_FAILED"

	// Dispute events
	EventDisputeOpened  CanonicalEventType = "DISPUTE_OPENED"
	EventDisputeClosed  CanonicalEventType = "DISPUTE_CLOSED"
	EventDisputeUpdated CanonicalEventType = "DISPUTE_UPDATED"

	// Expiration events
	EventAuthorizationExpired CanonicalEventType = "AUTHORIZATION_EXPIRED"
)

// IsSuccess returns true if this is a successful event
func (e *CanonicalEvent) IsSuccess() bool {
	switch e.Type {
	case EventAuthorizationSucceeded, EventCaptureSucceeded,
		EventVoidSucceeded, EventRefundSucceeded, EventDisputeClosed:
		return true
	default:
		return false
	}
}

// IsFailure returns true if this is a failure event
func (e *CanonicalEvent) IsFailure() bool {
	switch e.Type {
	case EventAuthorizationFailed, EventCaptureFailed,
		EventVoidFailed, EventRefundFailed:
		return true
	default:
		return false
	}
}

// IsRetryable returns true if the failure can be retried
func (e *CanonicalEvent) IsRetryable() bool {
	if !e.IsFailure() || e.DeclineType == nil {
		return false
	}
	return *e.DeclineType == DeclineTypeSoft || *e.DeclineType == DeclineTypeTemporary
}

// RequiresAction returns true if this event requires workflow action
func (e *CanonicalEvent) RequiresAction() bool {
	switch e.Type {
	case EventAuthorizationSucceeded, EventAuthorizationFailed,
		EventCaptureSucceeded, EventCaptureFailed,
		EventVoidSucceeded, EventRefundSucceeded,
		EventDisputeOpened, EventAuthorizationExpired:
		return true
	default:
		return false
	}
}

// ProviderEventMapping maps provider event types to canonical types
var ProviderEventMapping = map[Provider]map[string]CanonicalEventType{
	ProviderStripe: {
		"payment_intent.succeeded":      EventAuthorizationSucceeded,
		"payment_intent.payment_failed": EventAuthorizationFailed,
		"charge.captured":               EventCaptureSucceeded,
		"charge.failed":                 EventCaptureFailed,
		"charge.refunded":               EventRefundSucceeded,
		"charge.refund_updated":         EventRefundSucceeded,
		"charge.dispute.created":        EventDisputeOpened,
		"charge.dispute.closed":         EventDisputeClosed,
		"charge.dispute.updated":        EventDisputeUpdated,
		"payment_intent.canceled":       EventVoidSucceeded,
	},
	ProviderAdyen: {
		"AUTHORISATION":       EventAuthorizationSucceeded, // success field determines actual outcome
		"CAPTURE":             EventCaptureSucceeded,
		"CAPTURE_FAILED":      EventCaptureFailed,
		"CANCELLATION":        EventVoidSucceeded,
		"REFUND":              EventRefundSucceeded,
		"REFUND_FAILED":       EventRefundFailed,
		"CHARGEBACK":          EventDisputeOpened,
		"CHARGEBACK_REVERSED": EventDisputeClosed,
	},
	ProviderPayPal: {
		"PAYMENT.AUTHORIZATION.CREATED": EventAuthorizationSucceeded,
		"PAYMENT.AUTHORIZATION.VOIDED":  EventVoidSucceeded,
		"PAYMENT.CAPTURE.COMPLETED":     EventCaptureSucceeded,
		"PAYMENT.CAPTURE.DENIED":        EventCaptureFailed,
		"PAYMENT.CAPTURE.REFUNDED":      EventRefundSucceeded,
		"CUSTOMER.DISPUTE.CREATED":      EventDisputeOpened,
		"CUSTOMER.DISPUTE.RESOLVED":     EventDisputeClosed,
		"CUSTOMER.DISPUTE.UPDATED":      EventDisputeUpdated,
	},
}

// MapProviderEvent maps a provider-specific event type to a canonical type
func MapProviderEvent(provider Provider, providerEvent string) (CanonicalEventType, bool) {
	if mapping, ok := ProviderEventMapping[provider]; ok {
		if eventType, ok := mapping[providerEvent]; ok {
			return eventType, true
		}
	}
	return "", false
}
