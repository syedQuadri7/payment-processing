// Package adyen provides Adyen API simulation for the provider simulator.
package adyen

import "time"

// PaymentRequest is the request body for creating a payment.
type PaymentRequest struct {
	Amount          *Amount           `json:"amount"`
	MerchantAccount string            `json:"merchantAccount"`
	Reference       string            `json:"reference"`
	PaymentMethod   *PaymentMethod    `json:"paymentMethod"`
	ReturnURL       string            `json:"returnUrl,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	ShopperEmail    string            `json:"shopperEmail,omitempty"`
	ShopperReference string           `json:"shopperReference,omitempty"`
}

// Amount represents an Adyen amount.
type Amount struct {
	Currency string `json:"currency"`
	Value    int64  `json:"value"`
}

// PaymentMethod represents an Adyen payment method.
type PaymentMethod struct {
	Type           string `json:"type"`
	Number         string `json:"number,omitempty"`
	ExpiryMonth    string `json:"expiryMonth,omitempty"`
	ExpiryYear     string `json:"expiryYear,omitempty"`
	CVC            string `json:"cvc,omitempty"`
	HolderName     string `json:"holderName,omitempty"`
	EncryptedCardNumber string `json:"encryptedCardNumber,omitempty"`
	EncryptedExpiryMonth string `json:"encryptedExpiryMonth,omitempty"`
	EncryptedExpiryYear string `json:"encryptedExpiryYear,omitempty"`
	EncryptedSecurityCode string `json:"encryptedSecurityCode,omitempty"`
}

// PaymentResponse is the response from a payment request.
type PaymentResponse struct {
	PspReference   string            `json:"pspReference"`
	ResultCode     string            `json:"resultCode"`
	MerchantReference string         `json:"merchantReference,omitempty"`
	Amount         *Amount           `json:"amount,omitempty"`
	RefusalReason  string            `json:"refusalReason,omitempty"`
	RefusalReasonCode string         `json:"refusalReasonCode,omitempty"`
	AdditionalData map[string]string `json:"additionalData,omitempty"`
}

// CaptureRequest is the request body for capturing a payment.
type CaptureRequest struct {
	MerchantAccount string  `json:"merchantAccount"`
	Amount          *Amount `json:"amount"`
	Reference       string  `json:"reference,omitempty"`
}

// CaptureResponse is the response from a capture request.
type CaptureResponse struct {
	PspReference       string `json:"pspReference"`
	PaymentPspReference string `json:"paymentPspReference"`
	Status             string `json:"status"`
	Amount             *Amount `json:"amount,omitempty"`
	Reference          string `json:"reference,omitempty"`
}

// CancelRequest is the request body for canceling a payment.
type CancelRequest struct {
	MerchantAccount string `json:"merchantAccount"`
	Reference       string `json:"reference,omitempty"`
}

// CancelResponse is the response from a cancel request.
type CancelResponse struct {
	PspReference       string `json:"pspReference"`
	PaymentPspReference string `json:"paymentPspReference"`
	Status             string `json:"status"`
	Reference          string `json:"reference,omitempty"`
}

// RefundRequest is the request body for creating a refund.
type RefundRequest struct {
	MerchantAccount string  `json:"merchantAccount"`
	Amount          *Amount `json:"amount"`
	Reference       string  `json:"reference,omitempty"`
}

// RefundResponse is the response from a refund request.
type RefundResponse struct {
	PspReference       string  `json:"pspReference"`
	PaymentPspReference string `json:"paymentPspReference"`
	Status             string  `json:"status"`
	Amount             *Amount `json:"amount,omitempty"`
	Reference          string  `json:"reference,omitempty"`
}

// AdyenError represents an Adyen API error response.
type AdyenError struct {
	Status    int    `json:"status"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
	ErrorType string `json:"errorType"`
}

// Result codes for Adyen responses.
const (
	ResultCodeAuthorised          = "Authorised"
	ResultCodeRefused             = "Refused"
	ResultCodePending             = "Pending"
	ResultCodeError               = "Error"
	ResultCodeCancelled           = "Cancelled"
	ResultCodeReceived            = "received"
)

// Modification statuses.
const (
	ModificationStatusReceived = "received"
)

// nowUnix returns the current Unix timestamp.
func nowUnix() int64 {
	return time.Now().Unix()
}

// statusMap maps internal states to Adyen result codes.
var statusMap = map[string]string{
	"created":                 "Pending",
	"requires_payment_method": "Pending",
	"requires_confirmation":   "Pending",
	"processing":              "Pending",
	"authorized":              "Authorised",
	"requires_capture":        "Authorised",
	"captured":                "Authorised",
	"canceled":                "Cancelled",
	"failed":                  "Refused",
}

// ToAdyenStatus converts internal status to Adyen result code.
func ToAdyenStatus(internal string) string {
	if status, ok := statusMap[internal]; ok {
		return status
	}
	return "Pending"
}
