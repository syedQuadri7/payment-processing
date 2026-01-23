package generator

import (
	"encoding/json"
	"fmt"
	"time"
)

// AdyenGenerator creates Adyen webhook notification payloads.
type AdyenGenerator struct{}

// NewAdyenGenerator creates a new Adyen payload generator.
func NewAdyenGenerator() *AdyenGenerator {
	return &AdyenGenerator{}
}

// Provider returns the provider name.
func (g *AdyenGenerator) Provider() string {
	return "adyen"
}

// Endpoint returns the webhook endpoint path.
func (g *AdyenGenerator) Endpoint() string {
	return "/webhooks/adyen"
}

// SupportedEvents returns supported Adyen event types.
func (g *AdyenGenerator) SupportedEvents() []string {
	return []string{
		"AUTHORISATION",
		"CAPTURE",
		"CAPTURE_FAILED",
		"REFUND",
		"REFUND_FAILED",
		"CHARGEBACK",
		"CHARGEBACK_REVERSED",
		"CANCELLATION",
	}
}

// Generate creates an Adyen webhook notification payload.
func (g *AdyenGenerator) Generate(eventType string, data map[string]any) ([]byte, error) {
	item := g.buildNotificationItem(eventType, data)

	payload := map[string]any{
		"live":              "false",
		"notificationItems": []map[string]any{
			{
				"NotificationRequestItem": item,
			},
		},
	}

	return json.MarshalIndent(payload, "", "  ")
}

func (g *AdyenGenerator) buildNotificationItem(eventType string, data map[string]any) map[string]any {
	now := time.Now()

	pspReference := getStringOrDefault(data, "psp_reference", fmt.Sprintf("ADYEN_%s", generateID()))
	merchantRef := getStringOrDefault(data, "merchant_reference", fmt.Sprintf("ORDER_%s", generateID()))
	amount := getIntOrDefault(data, "amount", 10000)
	currency := getStringOrDefault(data, "currency", "USD")

	item := map[string]any{
		"eventCode":         eventType,
		"eventDate":         now.Format(time.RFC3339),
		"merchantAccountCode": getStringOrDefault(data, "merchant_account", "TestMerchantAccount"),
		"merchantReference": merchantRef,
		"pspReference":      pspReference,
		"amount": map[string]any{
			"currency": currency,
			"value":    amount,
		},
		"success": true,
	}

	// Handle event-specific fields
	switch eventType {
	case "AUTHORISATION":
		item["paymentMethod"] = getStringOrDefault(data, "payment_method", "visa")
		if reason, ok := data["reason"]; ok {
			item["reason"] = reason
			item["success"] = false
		}

	case "CAPTURE", "CAPTURE_FAILED":
		originalRef := getStringOrDefault(data, "original_reference", fmt.Sprintf("ADYEN_%s", generateID()))
		item["originalReference"] = originalRef
		if eventType == "CAPTURE_FAILED" {
			item["success"] = false
			item["reason"] = getStringOrDefault(data, "reason", "Capture failed")
		}

	case "REFUND", "REFUND_FAILED":
		originalRef := getStringOrDefault(data, "original_reference", fmt.Sprintf("ADYEN_%s", generateID()))
		item["originalReference"] = originalRef
		if eventType == "REFUND_FAILED" {
			item["success"] = false
			item["reason"] = getStringOrDefault(data, "reason", "Refund failed")
		}

	case "CHARGEBACK", "CHARGEBACK_REVERSED":
		originalRef := getStringOrDefault(data, "original_reference", fmt.Sprintf("ADYEN_%s", generateID()))
		item["originalReference"] = originalRef
		item["reason"] = getStringOrDefault(data, "reason", "Chargeback")

	case "CANCELLATION":
		originalRef := getStringOrDefault(data, "original_reference", fmt.Sprintf("ADYEN_%s", generateID()))
		item["originalReference"] = originalRef
	}

	// Handle decline codes
	if declineCode, ok := data["decline_code"]; ok {
		item["success"] = false
		item["reason"] = declineCode
	}

	// Add additional data if present
	if additionalData, ok := data["additional_data"].(map[string]any); ok {
		item["additionalData"] = additionalData
	}

	return item
}
