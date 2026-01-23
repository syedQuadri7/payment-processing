package adapter

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"payment-processing/pkg/domain"
)

const (
	stripeSignatureHeader = "Stripe-Signature"
	stripeTimestampTolerance = 5 * time.Minute // Reject timestamps older than 5 minutes
)

// StripeAdapter handles Stripe webhook processing
type StripeAdapter struct {
	webhookSecret string
}

// NewStripeAdapter creates a new Stripe webhook adapter
func NewStripeAdapter(webhookSecret string) *StripeAdapter {
	return &StripeAdapter{
		webhookSecret: webhookSecret,
	}
}

// Provider returns the provider this adapter handles
func (a *StripeAdapter) Provider() domain.Provider {
	return domain.ProviderStripe
}

// VerifySignature verifies the Stripe webhook signature
func (a *StripeAdapter) VerifySignature(ctx context.Context, headers http.Header, body []byte) error {
	sigHeader := headers.Get(stripeSignatureHeader)
	if sigHeader == "" {
		return ErrMissingSignature
	}

	// Parse the signature header
	timestamp, signatures := parseStripeSignature(sigHeader)
	if timestamp == 0 {
		return ErrInvalidSignature
	}

	// Check timestamp for replay attack protection
	ts := time.Unix(timestamp, 0)
	if time.Since(ts) > stripeTimestampTolerance {
		return ErrTimestampExpired
	}

	// Construct the signed payload
	signedPayload := fmt.Sprintf("%d.%s", timestamp, string(body))

	// Compute expected signature
	expectedSig := computeHMACSHA256(signedPayload, a.webhookSecret)

	// Compare signatures using constant-time comparison
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expectedSig)) {
			return nil
		}
	}

	return ErrInvalidSignature
}

// ParseWebhook parses a Stripe webhook into canonical events
func (a *StripeAdapter) ParseWebhook(ctx context.Context, headers http.Header, body []byte) ([]*domain.CanonicalEvent, error) {
	var event stripeEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, &AdapterError{Code: "PARSE_ERROR", Message: "failed to parse Stripe event", Err: err}
	}

	// Map to canonical event type
	canonicalType, ok := domain.MapProviderEvent(domain.ProviderStripe, event.Type)
	if !ok {
		// Unknown event type - log but don't fail
		return nil, nil
	}

	// Parse the data object
	var dataObject stripeDataObject
	if err := json.Unmarshal(event.Data.Object, &dataObject); err != nil {
		return nil, &AdapterError{Code: "PARSE_ERROR", Message: "failed to parse Stripe data object", Err: err}
	}

	// Build canonical event
	canonicalEvent := &domain.CanonicalEvent{
		ID:                event.ID,
		Type:              canonicalType,
		Provider:          domain.ProviderStripe,
		ProviderEvent:     event.Type,
		ProviderPaymentID: dataObject.ID,
		RawPayload:        body,
		ReceivedAt:        time.Now(),
		Timestamp:         time.Unix(event.Created, 0),
	}

	// Extract internal payment ID from metadata
	if internalID, ok := dataObject.Metadata["internal_id"]; ok {
		canonicalEvent.PaymentIntentID = &internalID
	}

	// Set amount and currency
	if dataObject.Amount > 0 {
		// Stripe amounts are in smallest currency unit
		amount := decimal.NewFromInt(dataObject.Amount).Div(decimal.NewFromInt(100))
		canonicalEvent.Amount = &amount
		canonicalEvent.Currency = strings.ToUpper(dataObject.Currency)
	}

	// Handle failure events
	if canonicalType == domain.EventAuthorizationFailed || canonicalType == domain.EventCaptureFailed {
		if dataObject.LastPaymentError != nil {
			if dataObject.LastPaymentError.DeclineCode != "" {
				canonicalEvent.DeclineCode = &dataObject.LastPaymentError.DeclineCode
				declineType := mapStripeDeclineType(dataObject.LastPaymentError.DeclineCode)
				canonicalEvent.DeclineType = &declineType
			}
			if dataObject.LastPaymentError.Message != "" {
				canonicalEvent.ErrorMessage = &dataObject.LastPaymentError.Message
			}
		}
	}

	// Handle authorization events
	if canonicalType == domain.EventAuthorizationSucceeded {
		// For Adyen compatibility and tracking
		if dataObject.LatestCharge != "" {
			canonicalEvent.NetworkTxnID = &dataObject.LatestCharge
		}
	}

	// Handle dispute events
	if canonicalType == domain.EventDisputeOpened || canonicalType == domain.EventDisputeClosed {
		// Parse dispute from the raw payload
		var disputeEvent stripeDisputeEvent
		if err := json.Unmarshal(event.Data.Object, &disputeEvent); err == nil {
			canonicalEvent.DisputeID = &disputeEvent.ID
			canonicalEvent.DisputeReason = &disputeEvent.Reason
			if disputeEvent.Amount > 0 {
				disputeAmount := decimal.NewFromInt(disputeEvent.Amount).Div(decimal.NewFromInt(100))
				canonicalEvent.DisputeAmount = &disputeAmount
			}
		}
	}

	return []*domain.CanonicalEvent{canonicalEvent}, nil
}

// FormatResponse returns the Stripe success response format
func (a *StripeAdapter) FormatResponse() (int, string) {
	return http.StatusOK, ""
}

// parseStripeSignature parses the Stripe-Signature header
// Format: t=timestamp,v1=signature1,v0=signature0
func parseStripeSignature(header string) (int64, []string) {
	var timestamp int64
	var signatures []string

	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "t":
			ts, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				timestamp = ts
			}
		case "v1":
			signatures = append(signatures, value)
		}
	}

	return timestamp, signatures
}

// computeHMACSHA256 computes HMAC-SHA256 and returns hex-encoded result
func computeHMACSHA256(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// mapStripeDeclineType maps Stripe decline codes to decline types
func mapStripeDeclineType(declineCode string) domain.DeclineType {
	// Soft declines - retry eligible
	softDeclines := map[string]bool{
		"insufficient_funds": true,
		"card_declined":      true,
		"do_not_honor":       true,
		"try_again_later":    true,
		"processing_error":   true,
		"generic_decline":    true,
	}

	// Hard declines - not retry eligible
	hardDeclines := map[string]bool{
		"expired_card":        true,
		"incorrect_cvc":       true,
		"invalid_number":      true,
		"card_not_supported":  true,
		"invalid_expiry_year": true,
		"invalid_expiry_month": true,
	}

	// Fraud declines
	fraudDeclines := map[string]bool{
		"fraudulent":  true,
		"lost_card":   true,
		"stolen_card": true,
	}

	if softDeclines[declineCode] {
		return domain.DeclineTypeSoft
	}
	if hardDeclines[declineCode] {
		return domain.DeclineTypeHard
	}
	if fraudDeclines[declineCode] {
		return domain.DeclineTypeFraud
	}

	// Default to soft for unknown codes (retry eligible)
	return domain.DeclineTypeSoft
}

// Stripe webhook payload structures

type stripeEvent struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Created int64           `json:"created"`
	Data    stripeEventData `json:"data"`
}

type stripeEventData struct {
	Object json.RawMessage `json:"object"`
}

type stripeDataObject struct {
	ID               string            `json:"id"`
	Amount           int64             `json:"amount"`
	Currency         string            `json:"currency"`
	Status           string            `json:"status"`
	Metadata         map[string]string `json:"metadata"`
	LastPaymentError *stripePaymentError `json:"last_payment_error"`
	LatestCharge     string            `json:"latest_charge"`
}

type stripePaymentError struct {
	Code        string `json:"code"`
	DeclineCode string `json:"decline_code"`
	Message     string `json:"message"`
}

type stripeDisputeEvent struct {
	ID       string `json:"id"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Reason   string `json:"reason"`
	Status   string `json:"status"`
}
