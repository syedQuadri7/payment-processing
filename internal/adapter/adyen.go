package adapter

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"payment-processing/internal/domain"
)

const (
	adyenSignatureHeader = "HmacSignature"
)

// AdyenAdapter handles Adyen webhook (notification) processing
type AdyenAdapter struct {
	hmacKey []byte
}

// NewAdyenAdapter creates a new Adyen webhook adapter
func NewAdyenAdapter(hmacKeyHex string) *AdyenAdapter {
	// Adyen HMAC keys are typically base64-encoded
	hmacKey, _ := base64.StdEncoding.DecodeString(hmacKeyHex)
	if len(hmacKey) == 0 {
		// If not base64, try using as-is
		hmacKey = []byte(hmacKeyHex)
	}
	return &AdyenAdapter{
		hmacKey: hmacKey,
	}
}

// Provider returns the provider this adapter handles
func (a *AdyenAdapter) Provider() domain.Provider {
	return domain.ProviderAdyen
}

// VerifySignature verifies the Adyen webhook HMAC signature
func (a *AdyenAdapter) VerifySignature(ctx context.Context, headers http.Header, body []byte) error {
	sigHeader := headers.Get(adyenSignatureHeader)
	if sigHeader == "" {
		return ErrMissingSignature
	}

	// Compute HMAC-SHA256 of the body
	h := hmac.New(sha256.New, a.hmacKey)
	h.Write(body)
	expectedSig := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Compare signatures using constant-time comparison
	if !hmac.Equal([]byte(sigHeader), []byte(expectedSig)) {
		return ErrInvalidSignature
	}

	return nil
}

// ParseWebhook parses Adyen notifications into canonical events
// Note: Adyen may send multiple notifications in a single request
func (a *AdyenAdapter) ParseWebhook(ctx context.Context, headers http.Header, body []byte) ([]*domain.CanonicalEvent, error) {
	var notification adyenNotificationRequest
	if err := json.Unmarshal(body, &notification); err != nil {
		return nil, &AdapterError{Code: "PARSE_ERROR", Message: "failed to parse Adyen notification", Err: err}
	}

	var events []*domain.CanonicalEvent

	for _, item := range notification.NotificationItems {
		event, err := a.parseNotificationItem(item.NotificationRequestItem, body)
		if err != nil {
			// Log warning but continue processing other items
			continue
		}
		if event != nil {
			events = append(events, event)
		}
	}

	return events, nil
}

// FormatResponse returns the Adyen success response format
// Adyen requires exactly "[accepted]" as the response body
func (a *AdyenAdapter) FormatResponse() (int, string) {
	return http.StatusOK, "[accepted]"
}

// parseNotificationItem parses a single Adyen notification item
func (a *AdyenAdapter) parseNotificationItem(item adyenNotificationItem, rawPayload []byte) (*domain.CanonicalEvent, error) {
	// Determine canonical event type based on eventCode and success
	canonicalType := a.mapEventType(item.EventCode, item.Success)
	if canonicalType == "" {
		// Unknown or unmapped event type
		return nil, nil
	}

	// Build canonical event
	canonicalEvent := &domain.CanonicalEvent{
		ID:                item.PspReference,
		Type:              canonicalType,
		Provider:          domain.ProviderAdyen,
		ProviderEvent:     item.EventCode,
		ProviderPaymentID: item.PspReference,
		RawPayload:        rawPayload,
		ReceivedAt:        time.Now(),
	}

	// Parse event date
	if item.EventDate != "" {
		if t, err := time.Parse(time.RFC3339, item.EventDate); err == nil {
			canonicalEvent.Timestamp = t
		}
	}

	// Extract internal payment ID from merchant reference
	if item.MerchantReference != "" {
		canonicalEvent.PaymentIntentID = &item.MerchantReference
	}

	// Set amount and currency
	if item.Amount.Value > 0 {
		// Adyen amounts are in smallest currency unit
		amount := decimal.NewFromInt(item.Amount.Value).Div(decimal.NewFromInt(100))
		canonicalEvent.Amount = &amount
		canonicalEvent.Currency = strings.ToUpper(item.Amount.Currency)
	}

	// Handle failure events
	if canonicalType == domain.EventAuthorizationFailed || canonicalType == domain.EventCaptureFailed {
		if item.Reason != "" {
			canonicalEvent.DeclineCode = &item.Reason
			declineType := mapAdyenDeclineType(item.Reason)
			canonicalEvent.DeclineType = &declineType
			canonicalEvent.ErrorMessage = &item.Reason
		}
	}

	// Handle authorization events
	if canonicalType == domain.EventAuthorizationSucceeded {
		if item.AdditionalData != nil {
			if authCode, ok := item.AdditionalData["authCode"]; ok {
				canonicalEvent.AuthorizationCode = &authCode
			}
		}
	}

	// Handle chargeback events
	if canonicalType == domain.EventDisputeOpened {
		canonicalEvent.DisputeID = &item.PspReference
		if item.Reason != "" {
			canonicalEvent.DisputeReason = &item.Reason
		}
		if item.Amount.Value > 0 {
			disputeAmount := decimal.NewFromInt(item.Amount.Value).Div(decimal.NewFromInt(100))
			canonicalEvent.DisputeAmount = &disputeAmount
		}
	}

	return canonicalEvent, nil
}

// mapEventType maps Adyen event code and success flag to canonical event type
func (a *AdyenAdapter) mapEventType(eventCode string, success string) domain.CanonicalEventType {
	isSuccess := strings.ToLower(success) == "true"

	switch eventCode {
	case "AUTHORISATION":
		if isSuccess {
			return domain.EventAuthorizationSucceeded
		}
		return domain.EventAuthorizationFailed
	case "CAPTURE":
		if isSuccess {
			return domain.EventCaptureSucceeded
		}
		return domain.EventCaptureFailed
	case "CAPTURE_FAILED":
		return domain.EventCaptureFailed
	case "CANCELLATION":
		if isSuccess {
			return domain.EventVoidSucceeded
		}
		return domain.EventVoidFailed
	case "REFUND":
		if isSuccess {
			return domain.EventRefundSucceeded
		}
		return domain.EventRefundFailed
	case "REFUND_FAILED":
		return domain.EventRefundFailed
	case "CHARGEBACK":
		return domain.EventDisputeOpened
	case "CHARGEBACK_REVERSED":
		return domain.EventDisputeClosed
	default:
		return ""
	}
}

// mapAdyenDeclineType maps Adyen reason codes to decline types
func mapAdyenDeclineType(reason string) domain.DeclineType {
	// Hard declines - not retry eligible (check these first for specificity)
	hardDeclines := map[string]bool{
		"Refused:33": true, // Card expired
		"Refused:14": true, // Invalid number
		"Refused:82": true, // Invalid CVV
		"Refused:62": true, // Card restricted
		"Refused:63": true, // Card restricted
		"Expired Card": true,
		"Invalid Card Number": true,
		"CVC Declined": true,
	}

	// Fraud declines (check before soft for specificity)
	fraudDeclines := map[string]bool{
		"Refused:59": true, // Fraud suspicion
		"Refused:41": true, // Lost card
		"Refused:43": true, // Stolen card
		"FRAUD":      true,
		"Fraud":      true,
	}

	// Soft declines - retry eligible
	softDeclines := map[string]bool{
		"Refused:51":  true, // Insufficient funds
		"Refused:05":  true, // Generic decline
		"Refused:57":  true, // Do not honor
		"Refused:91":  true, // Try again
		"Refused:96":  true, // Processing error
	}

	// Check for exact matches first for specific codes
	if hardDeclines[reason] {
		return domain.DeclineTypeHard
	}
	if fraudDeclines[reason] {
		return domain.DeclineTypeFraud
	}
	if softDeclines[reason] {
		return domain.DeclineTypeSoft
	}

	// Check for partial matches (for complex reason codes like "Refused:33 Expired Card")
	for code := range hardDeclines {
		if strings.Contains(reason, code) {
			return domain.DeclineTypeHard
		}
	}
	for code := range fraudDeclines {
		if strings.Contains(reason, code) {
			return domain.DeclineTypeFraud
		}
	}
	for code := range softDeclines {
		if strings.Contains(reason, code) {
			return domain.DeclineTypeSoft
		}
	}

	// Default to soft for unknown codes (retry eligible)
	return domain.DeclineTypeSoft
}

// Adyen notification payload structures

type adyenNotificationRequest struct {
	Live              string                       `json:"live"`
	NotificationItems []adyenNotificationContainer `json:"notificationItems"`
}

type adyenNotificationContainer struct {
	NotificationRequestItem adyenNotificationItem `json:"NotificationRequestItem"`
}

type adyenNotificationItem struct {
	EventCode         string            `json:"eventCode"`
	Success           string            `json:"success"`
	PspReference      string            `json:"pspReference"`
	OriginalReference string            `json:"originalReference,omitempty"`
	MerchantReference string            `json:"merchantReference"`
	MerchantAccountCode string          `json:"merchantAccountCode"`
	Amount            adyenAmount       `json:"amount"`
	Reason            string            `json:"reason,omitempty"`
	EventDate         string            `json:"eventDate"`
	AdditionalData    map[string]string `json:"additionalData,omitempty"`
	Operations        []string          `json:"operations,omitempty"`
}

type adyenAmount struct {
	Currency string `json:"currency"`
	Value    int64  `json:"value"`
}
