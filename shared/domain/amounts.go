package domain

import (
	"encoding/json"
	"time"
)

// CanonicalEvent is the normalized webhook event workflows receive.
type CanonicalEvent struct {
	Amount         *int64 `json:"amount,omitempty"`
	CapturedAmount *int64 `json:"captured_amount,omitempty"`
	DisputeAmount  *int64 `json:"dispute_amount,omitempty"`

	ID                string             `json:"id"`
	Type              CanonicalEventType `json:"type"`
	Provider          Provider           `json:"provider"`
	ProviderEvent     string             `json:"provider_event"`
	PaymentIntentID   *string            `json:"payment_intent_id,omitempty"`
	ProviderPaymentID string             `json:"provider_payment_id"`
	Currency          string             `json:"currency,omitempty"`
	DeclineCode       *string            `json:"decline_code,omitempty"`
	DeclineType       *DeclineType       `json:"decline_type,omitempty"`
	ErrorMessage      *string            `json:"error_message,omitempty"`
	AuthorizationCode *string            `json:"authorization_code,omitempty"`
	NetworkTxnID      *string            `json:"network_txn_id,omitempty"`
	ExpiresAt         *time.Time         `json:"expires_at,omitempty"`
	DisputeID         *string            `json:"dispute_id,omitempty"`
	DisputeReason     *string            `json:"dispute_reason,omitempty"`
	RawPayload        json.RawMessage    `json:"raw_payload"`
	ReceivedAt        time.Time          `json:"received_at"`
	Timestamp         time.Time          `json:"timestamp"`
}
