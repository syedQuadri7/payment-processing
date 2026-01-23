// Package stripe provides Stripe API simulation for the provider simulator.
package stripe

import "time"

// PaymentIntent represents a Stripe payment intent.
type PaymentIntent struct {
	ID                    string            `json:"id"`
	Object                string            `json:"object"`
	Amount                int64             `json:"amount"`
	AmountCapturable      int64             `json:"amount_capturable"`
	AmountReceived        int64             `json:"amount_received"`
	CaptureMethod         string            `json:"capture_method"`
	ClientSecret          string            `json:"client_secret"`
	ConfirmationMethod    string            `json:"confirmation_method"`
	Created               int64             `json:"created"`
	Currency              string            `json:"currency"`
	Description           string            `json:"description,omitempty"`
	Livemode              bool              `json:"livemode"`
	Metadata              map[string]string `json:"metadata,omitempty"`
	PaymentMethodTypes    []string          `json:"payment_method_types"`
	Status                string            `json:"status"`
	PaymentMethod         string            `json:"payment_method,omitempty"`
	LastPaymentError      *PaymentError     `json:"last_payment_error,omitempty"`
	CanceledAt            *int64            `json:"canceled_at,omitempty"`
	CancellationReason    string            `json:"cancellation_reason,omitempty"`
	LatestCharge          string            `json:"latest_charge,omitempty"`
}

// PaymentError represents a Stripe payment error.
type PaymentError struct {
	Code        string `json:"code"`
	DeclineCode string `json:"decline_code,omitempty"`
	Message     string `json:"message"`
	Type        string `json:"type"`
}

// Charge represents a Stripe charge.
type Charge struct {
	ID                string            `json:"id"`
	Object            string            `json:"object"`
	Amount            int64             `json:"amount"`
	AmountCaptured    int64             `json:"amount_captured"`
	AmountRefunded    int64             `json:"amount_refunded"`
	BalanceTransaction string           `json:"balance_transaction,omitempty"`
	Captured          bool              `json:"captured"`
	Created           int64             `json:"created"`
	Currency          string            `json:"currency"`
	Livemode          bool              `json:"livemode"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	Paid              bool              `json:"paid"`
	PaymentIntent     string            `json:"payment_intent,omitempty"`
	PaymentMethod     string            `json:"payment_method,omitempty"`
	Refunded          bool              `json:"refunded"`
	Status            string            `json:"status"`
	FailureCode       string            `json:"failure_code,omitempty"`
	FailureMessage    string            `json:"failure_message,omitempty"`
}

// Refund represents a Stripe refund.
type Refund struct {
	ID       string            `json:"id"`
	Object   string            `json:"object"`
	Amount   int64             `json:"amount"`
	Charge   string            `json:"charge"`
	Created  int64             `json:"created"`
	Currency string            `json:"currency"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Reason   string            `json:"reason,omitempty"`
	Status   string            `json:"status"`
}

// CreatePaymentIntentRequest is the request body for creating a payment intent.
type CreatePaymentIntentRequest struct {
	Amount             int64             `json:"amount"`
	Currency           string            `json:"currency"`
	CaptureMethod      string            `json:"capture_method,omitempty"` // automatic or manual
	ConfirmationMethod string            `json:"confirmation_method,omitempty"`
	Confirm            bool              `json:"confirm,omitempty"`
	Description        string            `json:"description,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	PaymentMethod      string            `json:"payment_method,omitempty"`
	PaymentMethodData  *PaymentMethodData `json:"payment_method_data,omitempty"`
}

// PaymentMethodData contains payment method details.
type PaymentMethodData struct {
	Type string    `json:"type"`
	Card *CardData `json:"card,omitempty"`
}

// CardData contains card details for testing.
type CardData struct {
	Number   string `json:"number"`
	ExpMonth int    `json:"exp_month"`
	ExpYear  int    `json:"exp_year"`
	CVC      string `json:"cvc"`
}

// ConfirmPaymentIntentRequest is the request body for confirming a payment intent.
type ConfirmPaymentIntentRequest struct {
	PaymentMethod     string             `json:"payment_method,omitempty"`
	PaymentMethodData *PaymentMethodData `json:"payment_method_data,omitempty"`
}

// CapturePaymentIntentRequest is the request body for capturing a payment intent.
type CapturePaymentIntentRequest struct {
	AmountToCapture int64 `json:"amount_to_capture,omitempty"`
}

// CancelPaymentIntentRequest is the request body for canceling a payment intent.
type CancelPaymentIntentRequest struct {
	CancellationReason string `json:"cancellation_reason,omitempty"`
}

// CreateRefundRequest is the request body for creating a refund.
type CreateRefundRequest struct {
	PaymentIntent string            `json:"payment_intent,omitempty"`
	Charge        string            `json:"charge,omitempty"`
	Amount        int64             `json:"amount,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// StripeError represents a Stripe API error response.
type StripeError struct {
	Error StripeErrorBody `json:"error"`
}

// StripeErrorBody is the body of a Stripe error.
type StripeErrorBody struct {
	Type        string `json:"type"`
	Code        string `json:"code,omitempty"`
	DeclineCode string `json:"decline_code,omitempty"`
	Message     string `json:"message"`
	Param       string `json:"param,omitempty"`
}

// statusMap maps internal states to Stripe statuses.
var statusMap = map[string]string{
	"created":                  "requires_payment_method",
	"requires_payment_method":  "requires_payment_method",
	"requires_confirmation":    "requires_confirmation",
	"requires_action":          "requires_action",
	"processing":               "processing",
	"requires_capture":         "requires_capture",
	"authorized":               "requires_capture",
	"captured":                 "succeeded",
	"canceled":                 "canceled",
	"failed":                   "requires_payment_method",
}

// ToStripeStatus converts internal status to Stripe status.
func ToStripeStatus(internal string) string {
	if status, ok := statusMap[internal]; ok {
		return status
	}
	return internal
}

// nowUnix returns the current Unix timestamp.
func nowUnix() int64 {
	return time.Now().Unix()
}
