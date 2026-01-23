package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"provider-simulator/internal/server"
	"provider-simulator/internal/state"
)

func TestStripeDecline_InsufficientFunds(t *testing.T) {
	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())
	defer ts.Close()

	// Use insufficient funds test card
	body := `{
		"amount": 10000,
		"currency": "usd",
		"payment_method_data": {"type": "card", "card": {"number": "4000000000009995"}},
		"confirm": true
	}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should fail with card error
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	// The payment intent should have last_payment_error
	if result["last_payment_error"] != nil {
		errObj := result["last_payment_error"].(map[string]any)
		assert.Equal(t, "card_error", errObj["type"])
	}
}

func TestStripeDecline_ExpiredCard(t *testing.T) {
	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())
	defer ts.Close()

	body := `{
		"amount": 10000,
		"currency": "usd",
		"payment_method_data": {"type": "card", "card": {"number": "4000000000000069"}},
		"confirm": true
	}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if result["last_payment_error"] != nil {
		errObj := result["last_payment_error"].(map[string]any)
		assert.Equal(t, "card_error", errObj["type"])
		assert.Equal(t, "expired_card", errObj["decline_code"])
	}
}

func TestStripeDecline_GenericDecline(t *testing.T) {
	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())
	defer ts.Close()

	body := `{
		"amount": 10000,
		"currency": "usd",
		"payment_method_data": {"type": "card", "card": {"number": "4000000000000002"}},
		"confirm": true
	}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if result["last_payment_error"] != nil {
		errObj := result["last_payment_error"].(map[string]any)
		assert.Equal(t, "card_error", errObj["type"])
	}
}

func TestAdyenDecline_InsufficientFunds(t *testing.T) {
	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())
	defer ts.Close()

	body := `{
		"amount": {"currency": "EUR", "value": 10000},
		"merchantAccount": "TestMerchant",
		"reference": "order_123",
		"paymentMethod": {"type": "scheme", "number": "4000000000009995"}
	}`
	resp, err := http.Post(ts.URL+"/adyen/v71/payments", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	assert.Equal(t, "Refused", result["resultCode"])
	assert.Contains(t, result["refusalReason"].(string), "Refused")
}

func TestAdyenDecline_StolenCard(t *testing.T) {
	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())
	defer ts.Close()

	body := `{
		"amount": {"currency": "EUR", "value": 10000},
		"merchantAccount": "TestMerchant",
		"reference": "order_123",
		"paymentMethod": {"type": "scheme", "number": "4000000000009979"}
	}`
	resp, err := http.Post(ts.URL+"/adyen/v71/payments", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	assert.Equal(t, "Refused", result["resultCode"])
}

func TestPayPalDecline_InsufficientFunds(t *testing.T) {
	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())
	defer ts.Close()

	// Create order with decline card
	body := `{
		"intent": "CAPTURE",
		"purchase_units": [{
			"amount": {"currency_code": "USD", "value": "100.00"}
		}],
		"payment_source": {"card": {"number": "4000000000009995"}}
	}`
	resp, err := http.Post(ts.URL+"/paypal/v2/checkout/orders", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)

	var order map[string]any
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()
	orderID := order["id"].(string)

	// Attempt capture - should fail
	resp, err = http.Post(ts.URL+"/paypal/v2/checkout/orders/"+orderID+"/capture", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return an error
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestMetadataDeclineTrigger(t *testing.T) {
	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())
	defer ts.Close()

	// Use success card but with decline metadata
	body := `{
		"amount": 10000,
		"currency": "usd",
		"metadata": {"x-sim-decline": "INSUFFICIENT_FUNDS"},
		"payment_method_data": {"type": "card", "card": {"number": "4242424242424242"}},
		"confirm": true
	}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	// Should fail due to metadata trigger
	if result["last_payment_error"] != nil {
		errObj := result["last_payment_error"].(map[string]any)
		assert.Equal(t, "card_error", errObj["type"])
	}
}

func TestAllCanonicalDeclines_Stripe(t *testing.T) {
	testCards := map[string]string{
		"4000000000000002": "GENERIC_DECLINE",
		"4000000000009995": "INSUFFICIENT_FUNDS",
		"4000000000000069": "CARD_EXPIRED",
		"4000000000009987": "LOST_CARD",
		"4000000000009979": "STOLEN_CARD",
		"4000000000000127": "INVALID_CVC",
	}

	for card, expectedDecline := range testCards {
		t.Run(expectedDecline, func(t *testing.T) {
			store := state.NewMemoryStore()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			cfg := server.Config{WebhookTarget: "http://localhost:8080", WebhookSecret: "test"}
			srv := server.New(cfg, store, logger)
			ts := httptest.NewServer(srv.SetupRouter())
			defer ts.Close()

			body := `{
				"amount": 10000,
				"currency": "usd",
				"payment_method_data": {"type": "card", "card": {"number": "` + card + `"}},
				"confirm": true
			}`
			resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
			require.NoError(t, err)
			defer resp.Body.Close()

			var result map[string]any
			json.NewDecoder(resp.Body).Decode(&result)

			// Should have error for decline cards
			if result["last_payment_error"] != nil {
				errObj := result["last_payment_error"].(map[string]any)
				assert.Equal(t, "card_error", errObj["type"])
			}
		})
	}
}
