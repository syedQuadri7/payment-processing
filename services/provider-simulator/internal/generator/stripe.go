package generator

import (
	"encoding/json"
	"fmt"
	"time"
)

// StripeGenerator creates Stripe webhook payloads.
type StripeGenerator struct{}

// NewStripeGenerator creates a new Stripe payload generator.
func NewStripeGenerator() *StripeGenerator {
	return &StripeGenerator{}
}

// Provider returns the provider name.
func (g *StripeGenerator) Provider() string {
	return "stripe"
}

// Endpoint returns the webhook endpoint path.
func (g *StripeGenerator) Endpoint() string {
	return "/webhooks/stripe"
}

// SupportedEvents returns supported Stripe event types.
func (g *StripeGenerator) SupportedEvents() []string {
	return []string{
		"payment_intent.succeeded",
		"payment_intent.payment_failed",
		"charge.captured",
		"charge.failed",
		"charge.refunded",
		"charge.dispute.created",
		"charge.dispute.closed",
	}
}

// Generate creates a Stripe webhook payload.
func (g *StripeGenerator) Generate(eventType string, data map[string]any) ([]byte, error) {
	now := time.Now().Unix()

	// Build the object based on event type
	obj := g.buildObject(eventType, data)

	payload := map[string]any{
		"id":       fmt.Sprintf("evt_%s", generateID()),
		"object":   "event",
		"api_version": "2023-10-16",
		"created":  now,
		"type":     eventType,
		"livemode": false,
		"pending_webhooks": 1,
		"request": map[string]any{
			"id":              nil,
			"idempotency_key": nil,
		},
		"data": map[string]any{
			"object": obj,
		},
	}

	return json.MarshalIndent(payload, "", "  ")
}

func (g *StripeGenerator) buildObject(eventType string, data map[string]any) map[string]any {
	switch eventType {
	case "payment_intent.succeeded", "payment_intent.payment_failed":
		return g.buildPaymentIntent(eventType, data)
	case "charge.captured", "charge.failed", "charge.refunded":
		return g.buildCharge(eventType, data)
	case "charge.dispute.created", "charge.dispute.closed":
		return g.buildDispute(eventType, data)
	default:
		return data
	}
}

func (g *StripeGenerator) buildPaymentIntent(eventType string, data map[string]any) map[string]any {
	now := time.Now().Unix()

	piID := getStringOrDefault(data, "payment_id", fmt.Sprintf("pi_%s", generateID()))
	amount := getIntOrDefault(data, "amount", 10000)
	currency := getStringOrDefault(data, "currency", "usd")
	status := "succeeded"

	obj := map[string]any{
		"id":                          piID,
		"object":                      "payment_intent",
		"amount":                      amount,
		"amount_capturable":           0,
		"amount_received":             amount,
		"capture_method":              "automatic",
		"client_secret":               fmt.Sprintf("%s_secret_%s", piID, generateID()),
		"confirmation_method":         "automatic",
		"created":                     now - 60,
		"currency":                    currency,
		"livemode":                    false,
		"payment_method_types":        []string{"card"},
		"status":                      status,
	}

	if eventType == "payment_intent.payment_failed" {
		obj["status"] = "requires_payment_method"
		obj["amount_received"] = 0

		declineCode := getStringOrDefault(data, "decline_code", "card_declined")
		obj["last_payment_error"] = map[string]any{
			"code":         declineCode,
			"decline_code": declineCode,
			"message":      getDeclineMessage(declineCode),
			"type":         "card_error",
		}
	}

	// Include metadata if present
	if metadata, ok := data["metadata"].(map[string]string); ok {
		obj["metadata"] = metadata
	}

	return obj
}

func (g *StripeGenerator) buildCharge(eventType string, data map[string]any) map[string]any {
	now := time.Now().Unix()

	chargeID := getStringOrDefault(data, "charge_id", fmt.Sprintf("ch_%s", generateID()))
	amount := getIntOrDefault(data, "amount", 10000)
	currency := getStringOrDefault(data, "currency", "usd")

	obj := map[string]any{
		"id":                     chargeID,
		"object":                 "charge",
		"amount":                 amount,
		"amount_captured":        amount,
		"amount_refunded":        0,
		"balance_transaction":    fmt.Sprintf("txn_%s", generateID()),
		"captured":               true,
		"created":                now - 30,
		"currency":               currency,
		"livemode":               false,
		"paid":                   true,
		"payment_method":         fmt.Sprintf("pm_%s", generateID()),
		"status":                 "succeeded",
	}

	switch eventType {
	case "charge.failed":
		obj["status"] = "failed"
		obj["paid"] = false
		obj["captured"] = false
		obj["amount_captured"] = 0

		declineCode := getStringOrDefault(data, "decline_code", "card_declined")
		obj["failure_code"] = declineCode
		obj["failure_message"] = getDeclineMessage(declineCode)
		obj["outcome"] = map[string]any{
			"network_status": "declined_by_network",
			"reason":         declineCode,
			"type":           "issuer_declined",
		}

	case "charge.refunded":
		refundAmount := getIntOrDefault(data, "refund_amount", amount)
		obj["amount_refunded"] = refundAmount
		obj["refunded"] = refundAmount == amount
		obj["refunds"] = map[string]any{
			"object":     "list",
			"has_more":   false,
			"total_count": 1,
			"data": []map[string]any{
				{
					"id":       fmt.Sprintf("re_%s", generateID()),
					"object":   "refund",
					"amount":   refundAmount,
					"charge":   chargeID,
					"created":  now,
					"currency": currency,
					"status":   "succeeded",
				},
			},
		}
	}

	// Merge any additional data
	for k, v := range data {
		if k != "charge_id" && k != "amount" && k != "currency" && k != "decline_code" && k != "refund_amount" {
			obj[k] = v
		}
	}

	return obj
}

func (g *StripeGenerator) buildDispute(eventType string, data map[string]any) map[string]any {
	now := time.Now().Unix()

	disputeID := getStringOrDefault(data, "dispute_id", fmt.Sprintf("dp_%s", generateID()))
	chargeID := getStringOrDefault(data, "charge_id", fmt.Sprintf("ch_%s", generateID()))
	amount := getIntOrDefault(data, "amount", 10000)
	reason := getStringOrDefault(data, "reason", "fraudulent")

	status := "needs_response"
	if eventType == "charge.dispute.closed" {
		status = getStringOrDefault(data, "status", "lost")
	}

	obj := map[string]any{
		"id":               disputeID,
		"object":           "dispute",
		"amount":           amount,
		"charge":           chargeID,
		"created":          now - 3600,
		"currency":         getStringOrDefault(data, "currency", "usd"),
		"is_charge_refundable": status != "lost",
		"livemode":         false,
		"reason":           reason,
		"status":           status,
		"evidence_details": map[string]any{
			"due_by":            now + 86400*7,
			"has_evidence":      false,
			"submission_count":  0,
		},
	}

	return obj
}

func getDeclineMessage(code string) string {
	messages := map[string]string{
		"insufficient_funds": "Your card has insufficient funds.",
		"expired_card":       "Your card has expired.",
		"fraudulent":         "Your card was declined.",
		"card_declined":      "Your card was declined.",
		"processing_error":   "An error occurred while processing your card.",
	}
	if msg, ok := messages[code]; ok {
		return msg
	}
	return "Your card was declined."
}

func getStringOrDefault(data map[string]any, key, defaultVal string) string {
	if v, ok := data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getIntOrDefault(data map[string]any, key string, defaultVal int) int {
	if v, ok := data[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return defaultVal
}
