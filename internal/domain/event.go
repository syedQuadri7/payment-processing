package domain

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

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

// CanonicalEvent represents a normalized webhook event from any provider
// This is the internal representation that workflows receive
type CanonicalEvent struct {
	// Event identification
	ID            string             `json:"id"`
	Type          CanonicalEventType `json:"type"`
	Provider      Provider           `json:"provider"`
	ProviderEvent string             `json:"provider_event"` // Original event type

	// Payment reference
	PaymentIntentID   *string `json:"payment_intent_id,omitempty"`
	ProviderPaymentID string  `json:"provider_payment_id"`

	// Event data
	Amount       *decimal.Decimal `json:"amount,omitempty"`
	Currency     string           `json:"currency,omitempty"`
	DeclineCode  *string          `json:"decline_code,omitempty"`
	DeclineType  *DeclineType     `json:"decline_type,omitempty"`
	ErrorMessage *string          `json:"error_message,omitempty"`

	// Authorization details (for auth events)
	AuthorizationCode *string    `json:"authorization_code,omitempty"`
	NetworkTxnID      *string    `json:"network_txn_id,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`

	// Capture details
	CapturedAmount *decimal.Decimal `json:"captured_amount,omitempty"`

	// Dispute details
	DisputeID     *string          `json:"dispute_id,omitempty"`
	DisputeReason *string          `json:"dispute_reason,omitempty"`
	DisputeAmount *decimal.Decimal `json:"dispute_amount,omitempty"`

	// Metadata
	RawPayload json.RawMessage `json:"raw_payload"` // Original webhook payload
	ReceivedAt time.Time       `json:"received_at"`
	Timestamp  time.Time       `json:"timestamp"` // Provider's event timestamp
}

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
		"payment_intent.succeeded":       EventAuthorizationSucceeded,
		"payment_intent.payment_failed":  EventAuthorizationFailed,
		"charge.captured":                EventCaptureSucceeded,
		"charge.failed":                  EventCaptureFailed,
		"charge.refunded":                EventRefundSucceeded,
		"charge.refund_updated":          EventRefundSucceeded,
		"charge.dispute.created":         EventDisputeOpened,
		"charge.dispute.closed":          EventDisputeClosed,
		"charge.dispute.updated":         EventDisputeUpdated,
		"payment_intent.canceled":        EventVoidSucceeded,
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
