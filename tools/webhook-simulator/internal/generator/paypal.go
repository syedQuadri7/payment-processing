package generator

import (
	"encoding/json"
	"fmt"
	"time"
)

// PayPalGenerator creates PayPal webhook payloads.
type PayPalGenerator struct{}

// NewPayPalGenerator creates a new PayPal payload generator.
func NewPayPalGenerator() *PayPalGenerator {
	return &PayPalGenerator{}
}

// Provider returns the provider name.
func (g *PayPalGenerator) Provider() string {
	return "paypal"
}

// Endpoint returns the webhook endpoint path.
func (g *PayPalGenerator) Endpoint() string {
	return "/webhooks/paypal"
}

// SupportedEvents returns supported PayPal event types.
func (g *PayPalGenerator) SupportedEvents() []string {
	return []string{
		"PAYMENT.AUTHORIZATION.CREATED",
		"PAYMENT.AUTHORIZATION.VOIDED",
		"PAYMENT.CAPTURE.COMPLETED",
		"PAYMENT.CAPTURE.DENIED",
		"PAYMENT.CAPTURE.REFUNDED",
		"CUSTOMER.DISPUTE.CREATED",
		"CUSTOMER.DISPUTE.RESOLVED",
	}
}

// Generate creates a PayPal webhook payload.
func (g *PayPalGenerator) Generate(eventType string, data map[string]any) ([]byte, error) {
	now := time.Now()

	resource := g.buildResource(eventType, data)

	payload := map[string]any{
		"id":            fmt.Sprintf("WH-%s", generateID()),
		"event_version": "1.0",
		"create_time":   now.Format(time.RFC3339),
		"resource_type": g.getResourceType(eventType),
		"event_type":    eventType,
		"summary":       g.getSummary(eventType, data),
		"resource":      resource,
		"links": []map[string]any{
			{
				"href":   "https://api.sandbox.paypal.com/v1/notifications/webhooks-events/" + generateID(),
				"rel":    "self",
				"method": "GET",
			},
			{
				"href":   "https://api.sandbox.paypal.com/v1/notifications/webhooks-events/" + generateID() + "/resend",
				"rel":    "resend",
				"method": "POST",
			},
		},
	}

	return json.MarshalIndent(payload, "", "  ")
}

func (g *PayPalGenerator) getResourceType(eventType string) string {
	switch {
	case eventType == "PAYMENT.AUTHORIZATION.CREATED" || eventType == "PAYMENT.AUTHORIZATION.VOIDED":
		return "authorization"
	case eventType == "PAYMENT.CAPTURE.COMPLETED" || eventType == "PAYMENT.CAPTURE.DENIED" || eventType == "PAYMENT.CAPTURE.REFUNDED":
		return "capture"
	case eventType == "CUSTOMER.DISPUTE.CREATED" || eventType == "CUSTOMER.DISPUTE.RESOLVED":
		return "dispute"
	default:
		return "payment"
	}
}

func (g *PayPalGenerator) getSummary(eventType string, data map[string]any) string {
	amount := getIntOrDefault(data, "amount", 10000)
	currency := getStringOrDefault(data, "currency", "USD")
	amountStr := fmt.Sprintf("%.2f %s", float64(amount)/100, currency)

	switch eventType {
	case "PAYMENT.AUTHORIZATION.CREATED":
		return fmt.Sprintf("A payment authorization for %s was created", amountStr)
	case "PAYMENT.AUTHORIZATION.VOIDED":
		return fmt.Sprintf("A payment authorization for %s was voided", amountStr)
	case "PAYMENT.CAPTURE.COMPLETED":
		return fmt.Sprintf("A payment capture for %s was completed", amountStr)
	case "PAYMENT.CAPTURE.DENIED":
		return fmt.Sprintf("A payment capture for %s was denied", amountStr)
	case "PAYMENT.CAPTURE.REFUNDED":
		return fmt.Sprintf("A payment capture for %s was refunded", amountStr)
	case "CUSTOMER.DISPUTE.CREATED":
		return fmt.Sprintf("A dispute for %s was created", amountStr)
	case "CUSTOMER.DISPUTE.RESOLVED":
		return fmt.Sprintf("A dispute for %s was resolved", amountStr)
	default:
		return "A PayPal event occurred"
	}
}

func (g *PayPalGenerator) buildResource(eventType string, data map[string]any) map[string]any {
	switch {
	case eventType == "PAYMENT.AUTHORIZATION.CREATED" || eventType == "PAYMENT.AUTHORIZATION.VOIDED":
		return g.buildAuthorization(eventType, data)
	case eventType == "PAYMENT.CAPTURE.COMPLETED" || eventType == "PAYMENT.CAPTURE.DENIED" || eventType == "PAYMENT.CAPTURE.REFUNDED":
		return g.buildCapture(eventType, data)
	case eventType == "CUSTOMER.DISPUTE.CREATED" || eventType == "CUSTOMER.DISPUTE.RESOLVED":
		return g.buildDispute(eventType, data)
	default:
		return data
	}
}

func (g *PayPalGenerator) buildAuthorization(eventType string, data map[string]any) map[string]any {
	now := time.Now()
	authID := getStringOrDefault(data, "authorization_id", fmt.Sprintf("AUTH-%s", generateID()))
	amount := getIntOrDefault(data, "amount", 10000)
	currency := getStringOrDefault(data, "currency", "USD")

	status := "CREATED"
	if eventType == "PAYMENT.AUTHORIZATION.VOIDED" {
		status = "VOIDED"
	}

	resource := map[string]any{
		"id":           authID,
		"status":       status,
		"create_time":  now.Add(-time.Minute).Format(time.RFC3339),
		"update_time":  now.Format(time.RFC3339),
		"expiration_time": now.Add(72 * time.Hour).Format(time.RFC3339),
		"amount": map[string]any{
			"currency_code": currency,
			"value":         fmt.Sprintf("%.2f", float64(amount)/100),
		},
		"invoice_id": getStringOrDefault(data, "invoice_id", fmt.Sprintf("INV-%s", generateID())),
	}

	return resource
}

func (g *PayPalGenerator) buildCapture(eventType string, data map[string]any) map[string]any {
	now := time.Now()
	captureID := getStringOrDefault(data, "capture_id", fmt.Sprintf("CAP-%s", generateID()))
	amount := getIntOrDefault(data, "amount", 10000)
	currency := getStringOrDefault(data, "currency", "USD")

	status := "COMPLETED"
	switch eventType {
	case "PAYMENT.CAPTURE.DENIED":
		status = "DECLINED"
	case "PAYMENT.CAPTURE.REFUNDED":
		status = "REFUNDED"
	}

	resource := map[string]any{
		"id":            captureID,
		"status":        status,
		"create_time":   now.Add(-time.Minute).Format(time.RFC3339),
		"update_time":   now.Format(time.RFC3339),
		"final_capture": true,
		"amount": map[string]any{
			"currency_code": currency,
			"value":         fmt.Sprintf("%.2f", float64(amount)/100),
		},
		"seller_protection": map[string]any{
			"status": "ELIGIBLE",
		},
		"invoice_id": getStringOrDefault(data, "invoice_id", fmt.Sprintf("INV-%s", generateID())),
	}

	// Handle decline codes
	if eventType == "PAYMENT.CAPTURE.DENIED" {
		declineCode := getStringOrDefault(data, "decline_code", "PAYMENT_DENIED")
		resource["status_details"] = map[string]any{
			"reason": declineCode,
		}
	}

	return resource
}

func (g *PayPalGenerator) buildDispute(eventType string, data map[string]any) map[string]any {
	now := time.Now()
	disputeID := getStringOrDefault(data, "dispute_id", fmt.Sprintf("PP-%s", generateID()))
	amount := getIntOrDefault(data, "amount", 10000)
	currency := getStringOrDefault(data, "currency", "USD")

	status := "OPEN"
	outcome := ""
	if eventType == "CUSTOMER.DISPUTE.RESOLVED" {
		status = "RESOLVED"
		outcome = getStringOrDefault(data, "outcome", "RESOLVED_BUYER_FAVOUR")
	}

	resource := map[string]any{
		"dispute_id":   disputeID,
		"status":       status,
		"reason":       getStringOrDefault(data, "reason", "MERCHANDISE_OR_SERVICE_NOT_RECEIVED"),
		"create_time":  now.Add(-24 * time.Hour).Format(time.RFC3339),
		"update_time":  now.Format(time.RFC3339),
		"dispute_amount": map[string]any{
			"currency_code": currency,
			"value":         fmt.Sprintf("%.2f", float64(amount)/100),
		},
		"dispute_life_cycle_stage": "INQUIRY",
		"dispute_channel":          "INTERNAL",
	}

	if outcome != "" {
		resource["dispute_outcome"] = map[string]any{
			"outcome_code": outcome,
		}
	}

	// Add disputed transactions
	resource["disputed_transactions"] = []map[string]any{
		{
			"seller_transaction_id": getStringOrDefault(data, "transaction_id", fmt.Sprintf("TXN-%s", generateID())),
			"create_time":           now.Add(-48 * time.Hour).Format(time.RFC3339),
			"transaction_status":    "COMPLETED",
		},
	}

	return resource
}
