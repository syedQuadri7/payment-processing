// Package paypal provides PayPal API simulation for the provider simulator.
package paypal

import "time"

// CreateOrderRequest is the request body for creating an order.
type CreateOrderRequest struct {
	Intent        string          `json:"intent"` // CAPTURE or AUTHORIZE
	PurchaseUnits []PurchaseUnit  `json:"purchase_units"`
	PaymentSource *PaymentSource  `json:"payment_source,omitempty"`
	Payer         *Payer          `json:"payer,omitempty"`
}

// PurchaseUnit represents items in an order.
type PurchaseUnit struct {
	ReferenceID string  `json:"reference_id,omitempty"`
	Amount      *Amount `json:"amount"`
	Description string  `json:"description,omitempty"`
	CustomID    string  `json:"custom_id,omitempty"`
	InvoiceID   string  `json:"invoice_id,omitempty"`
}

// Amount represents a PayPal amount.
type Amount struct {
	CurrencyCode string `json:"currency_code"`
	Value        string `json:"value"`
}

// PaymentSource represents a payment source.
type PaymentSource struct {
	Card *CardSource `json:"card,omitempty"`
}

// CardSource represents card payment details.
type CardSource struct {
	Number         string `json:"number,omitempty"`
	Expiry         string `json:"expiry,omitempty"`
	SecurityCode   string `json:"security_code,omitempty"`
	Name           string `json:"name,omitempty"`
	BillingAddress *Address `json:"billing_address,omitempty"`
}

// Address represents a physical address.
type Address struct {
	AddressLine1 string `json:"address_line_1,omitempty"`
	AddressLine2 string `json:"address_line_2,omitempty"`
	AdminArea1   string `json:"admin_area_1,omitempty"`
	AdminArea2   string `json:"admin_area_2,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
	CountryCode  string `json:"country_code,omitempty"`
}

// Payer represents a payer.
type Payer struct {
	Name         *PayerName `json:"name,omitempty"`
	EmailAddress string     `json:"email_address,omitempty"`
	PayerID      string     `json:"payer_id,omitempty"`
}

// PayerName represents a payer's name.
type PayerName struct {
	GivenName string `json:"given_name,omitempty"`
	Surname   string `json:"surname,omitempty"`
}

// Order represents a PayPal order.
type Order struct {
	ID            string            `json:"id"`
	Status        string            `json:"status"`
	Intent        string            `json:"intent"`
	PurchaseUnits []PurchaseUnitResponse `json:"purchase_units"`
	CreateTime    string            `json:"create_time"`
	UpdateTime    string            `json:"update_time,omitempty"`
	Links         []Link            `json:"links"`
	Payer         *Payer            `json:"payer,omitempty"`
}

// PurchaseUnitResponse represents a purchase unit in a response.
type PurchaseUnitResponse struct {
	ReferenceID string        `json:"reference_id,omitempty"`
	Amount      *Amount       `json:"amount"`
	Payments    *PaymentCollection `json:"payments,omitempty"`
}

// PaymentCollection contains authorizations and captures.
type PaymentCollection struct {
	Authorizations []Authorization `json:"authorizations,omitempty"`
	Captures       []Capture       `json:"captures,omitempty"`
	Refunds        []Refund        `json:"refunds,omitempty"`
}

// Authorization represents a PayPal authorization.
type Authorization struct {
	ID               string  `json:"id"`
	Status           string  `json:"status"`
	Amount           *Amount `json:"amount"`
	InvoiceID        string  `json:"invoice_id,omitempty"`
	CreateTime       string  `json:"create_time"`
	UpdateTime       string  `json:"update_time,omitempty"`
	ExpirationTime   string  `json:"expiration_time,omitempty"`
	Links            []Link  `json:"links,omitempty"`
}

// Capture represents a PayPal capture.
type Capture struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	Amount        *Amount `json:"amount"`
	InvoiceID     string  `json:"invoice_id,omitempty"`
	CreateTime    string  `json:"create_time"`
	UpdateTime    string  `json:"update_time,omitempty"`
	FinalCapture  bool    `json:"final_capture"`
	StatusDetails *StatusDetails `json:"status_details,omitempty"`
	Links         []Link  `json:"links,omitempty"`
}

// Refund represents a PayPal refund.
type Refund struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`
	Amount     *Amount `json:"amount"`
	CreateTime string  `json:"create_time"`
	Links      []Link  `json:"links,omitempty"`
}

// StatusDetails contains details about a status.
type StatusDetails struct {
	Reason string `json:"reason,omitempty"`
}

// Link represents a HATEOAS link.
type Link struct {
	Href   string `json:"href"`
	Rel    string `json:"rel"`
	Method string `json:"method,omitempty"`
}

// CaptureAuthorizationRequest is the request for capturing an authorization.
type CaptureAuthorizationRequest struct {
	Amount    *Amount `json:"amount,omitempty"`
	InvoiceID string  `json:"invoice_id,omitempty"`
	NoteToPayer string `json:"note_to_payer,omitempty"`
	FinalCapture bool  `json:"final_capture,omitempty"`
}

// VoidAuthorizationRequest is the request for voiding an authorization.
type VoidAuthorizationRequest struct {
	// Empty - no body required
}

// RefundCaptureRequest is the request for refunding a capture.
type RefundCaptureRequest struct {
	Amount      *Amount `json:"amount,omitempty"`
	InvoiceID   string  `json:"invoice_id,omitempty"`
	NoteToPayer string  `json:"note_to_payer,omitempty"`
}

// TokenRequest is the request for OAuth token.
type TokenRequest struct {
	GrantType string `json:"grant_type"`
}

// TokenResponse is the OAuth token response.
type TokenResponse struct {
	Scope       string `json:"scope"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	AppID       string `json:"app_id"`
	ExpiresIn   int    `json:"expires_in"`
	Nonce       string `json:"nonce"`
}

// VerifyWebhookRequest is the request for verifying a webhook signature.
type VerifyWebhookRequest struct {
	AuthAlgo         string `json:"auth_algo"`
	CertURL          string `json:"cert_url"`
	TransmissionID   string `json:"transmission_id"`
	TransmissionSig  string `json:"transmission_sig"`
	TransmissionTime string `json:"transmission_time"`
	WebhookID        string `json:"webhook_id"`
	WebhookEvent     any    `json:"webhook_event"`
}

// VerifyWebhookResponse is the response for webhook verification.
type VerifyWebhookResponse struct {
	VerificationStatus string `json:"verification_status"` // SUCCESS or FAILURE
}

// PayPalError represents a PayPal API error.
type PayPalError struct {
	Name    string         `json:"name"`
	Message string         `json:"message"`
	Details []ErrorDetail  `json:"details,omitempty"`
	Links   []Link         `json:"links,omitempty"`
}

// ErrorDetail contains details about an error.
type ErrorDetail struct {
	Issue       string `json:"issue"`
	Description string `json:"description"`
	Field       string `json:"field,omitempty"`
}

// Order statuses.
const (
	OrderStatusCreated   = "CREATED"
	OrderStatusApproved  = "APPROVED"
	OrderStatusCompleted = "COMPLETED"
	OrderStatusVoided    = "VOIDED"
)

// Authorization statuses.
const (
	AuthStatusCreated       = "CREATED"
	AuthStatusCaptured      = "CAPTURED"
	AuthStatusDenied        = "DENIED"
	AuthStatusExpired       = "EXPIRED"
	AuthStatusPartiallyCapt = "PARTIALLY_CAPTURED"
	AuthStatusVoided        = "VOIDED"
	AuthStatusPending       = "PENDING"
)

// Capture statuses.
const (
	CaptureStatusCompleted = "COMPLETED"
	CaptureStatusDeclined  = "DECLINED"
	CaptureStatusPending   = "PENDING"
	CaptureStatusRefunded  = "REFUNDED"
)

// Refund statuses.
const (
	RefundStatusPending   = "PENDING"
	RefundStatusCompleted = "COMPLETED"
	RefundStatusFailed    = "FAILED"
)

// nowRFC3339 returns the current time in RFC3339 format.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// statusMap maps internal states to PayPal order statuses.
var statusMap = map[string]string{
	"created":                 OrderStatusCreated,
	"requires_payment_method": OrderStatusCreated,
	"requires_confirmation":   OrderStatusCreated,
	"processing":              OrderStatusApproved,
	"authorized":              OrderStatusApproved,
	"requires_capture":        OrderStatusApproved,
	"captured":                OrderStatusCompleted,
	"canceled":                OrderStatusVoided,
	"failed":                  OrderStatusCreated,
}

// ToPayPalStatus converts internal status to PayPal order status.
func ToPayPalStatus(internal string) string {
	if status, ok := statusMap[internal]; ok {
		return status
	}
	return OrderStatusCreated
}
