package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"payment-processing/shared/domain"
)

const (
	paypalTransmissionID   = "PAYPAL-TRANSMISSION-ID"
	paypalTransmissionTime = "PAYPAL-TRANSMISSION-TIME"
	paypalTransmissionSig  = "PAYPAL-TRANSMISSION-SIG"
	paypalCertURL          = "PAYPAL-CERT-URL"
	paypalAuthAlgo         = "PAYPAL-AUTH-ALGO"

	paypalDefaultAPIURL = "api-m.sandbox.paypal.com"
	paypalTokenEndpoint = "/v1/oauth2/token"
	paypalVerifyEndpoint = "/v1/notifications/verify-webhook-signature"
)

// PayPalAdapter handles PayPal webhook processing
type PayPalAdapter struct {
	clientID     string
	clientSecret string
	webhookID    string
	apiURL       string

	// Token caching
	tokenMu      sync.RWMutex
	accessToken  string
	tokenExpires time.Time
}

// NewPayPalAdapter creates a new PayPal webhook adapter
func NewPayPalAdapter(clientID, clientSecret, webhookID, apiURL string) *PayPalAdapter {
	if apiURL == "" {
		apiURL = paypalDefaultAPIURL
	}
	return &PayPalAdapter{
		clientID:     clientID,
		clientSecret: clientSecret,
		webhookID:    webhookID,
		apiURL:       apiURL,
	}
}

// Provider returns the provider this adapter handles
func (a *PayPalAdapter) Provider() domain.Provider {
	return domain.ProviderPayPal
}

// VerifySignature verifies the PayPal webhook by calling PayPal's verification API
func (a *PayPalAdapter) VerifySignature(ctx context.Context, headers http.Header, body []byte) error {
	// Extract required headers
	transmissionID := headers.Get(paypalTransmissionID)
	transmissionTime := headers.Get(paypalTransmissionTime)
	transmissionSig := headers.Get(paypalTransmissionSig)
	certURL := headers.Get(paypalCertURL)
	authAlgo := headers.Get(paypalAuthAlgo)

	if transmissionID == "" || transmissionTime == "" || transmissionSig == "" {
		return ErrMissingSignature
	}

	// Get access token
	token, err := a.getAccessToken(ctx)
	if err != nil {
		return &AdapterError{Code: "TOKEN_ERROR", Message: "failed to get PayPal access token", Err: err}
	}

	// Build verification request
	verifyRequest := paypalVerifyRequest{
		AuthAlgo:         authAlgo,
		CertURL:          certURL,
		TransmissionID:   transmissionID,
		TransmissionSig:  transmissionSig,
		TransmissionTime: transmissionTime,
		WebhookID:        a.webhookID,
		WebhookEvent:     json.RawMessage(body),
	}

	reqBody, err := json.Marshal(verifyRequest)
	if err != nil {
		return &AdapterError{Code: "MARSHAL_ERROR", Message: "failed to marshal verify request", Err: err}
	}

	// Call PayPal verification API
	url := fmt.Sprintf("https://%s%s", a.apiURL, paypalVerifyEndpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return &AdapterError{Code: "REQUEST_ERROR", Message: "failed to create verify request", Err: err}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrVerificationFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ErrVerificationFailed
	}

	var verifyResponse paypalVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResponse); err != nil {
		return &AdapterError{Code: "DECODE_ERROR", Message: "failed to decode verify response", Err: err}
	}

	if verifyResponse.VerificationStatus != "SUCCESS" {
		return ErrInvalidSignature
	}

	return nil
}

// ParseWebhook parses a PayPal webhook into canonical events
func (a *PayPalAdapter) ParseWebhook(ctx context.Context, headers http.Header, body []byte) ([]*domain.CanonicalEvent, error) {
	var event paypalWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, &AdapterError{Code: "PARSE_ERROR", Message: "failed to parse PayPal event", Err: err}
	}

	// Map to canonical event type
	canonicalType, ok := domain.MapProviderEvent(domain.ProviderPayPal, event.EventType)
	if !ok {
		// Unknown event type - log but don't fail
		return nil, nil
	}

	// Build canonical event
	canonicalEvent := &domain.CanonicalEvent{
		ID:                event.ID,
		Type:              canonicalType,
		Provider:          domain.ProviderPayPal,
		ProviderEvent:     event.EventType,
		ProviderPaymentID: event.Resource.ID,
		RawPayload:        body,
		ReceivedAt:        time.Now(),
	}

	// Parse event timestamp
	if event.CreateTime != "" {
		if t, err := time.Parse(time.RFC3339, event.CreateTime); err == nil {
			canonicalEvent.Timestamp = t
		}
	}

	// Extract internal payment ID from invoice_id
	if event.Resource.InvoiceID != "" {
		canonicalEvent.PaymentIntentID = &event.Resource.InvoiceID
	}

	// Set amount and currency
	if event.Resource.Amount.Value != "" {
		if amount, err := decimal.NewFromString(event.Resource.Amount.Value); err == nil {
			minor := amount.Mul(decimal.NewFromInt(100)).IntPart()
			canonicalEvent.Amount = &minor
			canonicalEvent.Currency = strings.ToUpper(event.Resource.Amount.CurrencyCode)
		}
	}

	// Handle failure events
	if canonicalType == domain.EventCaptureFailed {
		if event.Resource.StatusDetails != nil {
			canonicalEvent.DeclineCode = &event.Resource.StatusDetails.Reason
			declineType := mapPayPalDeclineType(event.Resource.StatusDetails.Reason)
			canonicalEvent.DeclineType = &declineType
			canonicalEvent.ErrorMessage = &event.Resource.StatusDetails.Reason
		}
	}

	// Handle authorization events
	if canonicalType == domain.EventAuthorizationSucceeded {
		if event.Resource.ExpirationTime != "" {
			if t, err := time.Parse(time.RFC3339, event.Resource.ExpirationTime); err == nil {
				canonicalEvent.ExpiresAt = &t
			}
		}
	}

	// Handle dispute events
	if canonicalType == domain.EventDisputeOpened || canonicalType == domain.EventDisputeClosed {
		canonicalEvent.DisputeID = &event.Resource.ID
		if event.Resource.Reason != "" {
			canonicalEvent.DisputeReason = &event.Resource.Reason
		}
		if event.Resource.DisputeAmount.Value != "" {
			if disputeAmount, err := decimal.NewFromString(event.Resource.DisputeAmount.Value); err == nil {
				minor := disputeAmount.Mul(decimal.NewFromInt(100)).IntPart()
				canonicalEvent.DisputeAmount = &minor
			}
		}
	}

	return []*domain.CanonicalEvent{canonicalEvent}, nil
}

// FormatResponse returns the PayPal success response format
func (a *PayPalAdapter) FormatResponse() (int, string) {
	return http.StatusOK, ""
}

// getAccessToken gets a valid access token, refreshing if necessary
func (a *PayPalAdapter) getAccessToken(ctx context.Context) (string, error) {
	a.tokenMu.RLock()
	if a.accessToken != "" && time.Now().Before(a.tokenExpires) {
		token := a.accessToken
		a.tokenMu.RUnlock()
		return token, nil
	}
	a.tokenMu.RUnlock()

	// Need to refresh token
	return a.refreshAccessToken(ctx)
}

// refreshAccessToken obtains a new access token from PayPal
func (a *PayPalAdapter) refreshAccessToken(ctx context.Context) (string, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	// Double-check in case another goroutine refreshed while we waited
	if a.accessToken != "" && time.Now().Before(a.tokenExpires) {
		return a.accessToken, nil
	}

	url := fmt.Sprintf("https://%s%s", a.apiURL, paypalTokenEndpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(a.clientID, a.clientSecret)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed: %s", string(bodyBytes))
	}

	var tokenResp paypalTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	a.accessToken = tokenResp.AccessToken
	// Expire 5 minutes early to avoid edge cases
	a.tokenExpires = time.Now().Add(time.Duration(tokenResp.ExpiresIn-300) * time.Second)

	return a.accessToken, nil
}

// mapPayPalDeclineType maps PayPal decline codes to decline types
func mapPayPalDeclineType(reason string) domain.DeclineType {
	// Soft declines - retry eligible
	softDeclines := map[string]bool{
		"INSUFFICIENT_FUNDS":      true,
		"INSTRUMENT_DECLINED":     true,
		"DO_NOT_HONOR":            true,
		"PAYER_ACTION_REQUIRED":   true,
		"INTERNAL_SERVICE_ERROR":  true,
		"PAYMENT_DENIED":          true,
	}

	// Hard declines - not retry eligible
	hardDeclines := map[string]bool{
		"CREDIT_CARD_EXPIRED":           true,
		"INVALID_ACCOUNT":               true,
		"CARD_TYPE_NOT_SUPPORTED":       true,
		"CREDIT_CARD_CVV_CHECK_FAILED":  true,
		"ACCOUNT_CLOSED":                true,
		"CARD_BRAND_NOT_SUPPORTED":      true,
	}

	// Fraud declines
	fraudDeclines := map[string]bool{
		"TRANSACTION_REFUSED":     true,
		"PAYER_ACCOUNT_LOCKED":    true,
		"PAYER_ACCOUNT_RESTRICTED": true,
	}

	if softDeclines[reason] {
		return domain.DeclineTypeSoft
	}
	if hardDeclines[reason] {
		return domain.DeclineTypeHard
	}
	if fraudDeclines[reason] {
		return domain.DeclineTypeFraud
	}

	// Default to soft for unknown codes (retry eligible)
	return domain.DeclineTypeSoft
}

// PayPal webhook payload structures

type paypalWebhookEvent struct {
	ID         string           `json:"id"`
	EventType  string           `json:"event_type"`
	CreateTime string           `json:"create_time"`
	Resource   paypalResource   `json:"resource"`
}

type paypalResource struct {
	ID             string              `json:"id"`
	Status         string              `json:"status"`
	InvoiceID      string              `json:"invoice_id,omitempty"`
	CustomID       string              `json:"custom_id,omitempty"`
	Amount         paypalAmount        `json:"amount"`
	ExpirationTime string              `json:"expiration_time,omitempty"`
	StatusDetails  *paypalStatusDetails `json:"status_details,omitempty"`
	Reason         string              `json:"reason,omitempty"`       // For disputes
	DisputeAmount  paypalAmount        `json:"dispute_amount,omitempty"` // For disputes
}

type paypalAmount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

type paypalStatusDetails struct {
	Reason string `json:"reason"`
}

type paypalVerifyRequest struct {
	AuthAlgo         string          `json:"auth_algo"`
	CertURL          string          `json:"cert_url"`
	TransmissionID   string          `json:"transmission_id"`
	TransmissionSig  string          `json:"transmission_sig"`
	TransmissionTime string          `json:"transmission_time"`
	WebhookID        string          `json:"webhook_id"`
	WebhookEvent     json.RawMessage `json:"webhook_event"`
}

type paypalVerifyResponse struct {
	VerificationStatus string `json:"verification_status"`
}

type paypalTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}
