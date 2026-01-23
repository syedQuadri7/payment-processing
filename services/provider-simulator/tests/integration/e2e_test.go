// Package integration provides end-to-end tests for the provider simulator.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// testServer creates a test server with a fresh store.
func testServer(t *testing.T) (*httptest.Server, state.Store) {
	t.Helper()

	store := state.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := server.Config{
		Port:           0, // Not used for test server
		WebhookTarget:  "http://localhost:8080",
		WebhookSecret:  "whsec_test_secret",
		WebhookDelayMs: 0,
	}

	srv := server.New(cfg, store, logger)
	ts := httptest.NewServer(srv.SetupRouter())

	return ts, store
}

func TestHealthEndpoint(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "healthy", result["status"])
	assert.NotNil(t, result["stats"])
}

func TestStripePaymentIntent_Success(t *testing.T) {
	ts, store := testServer(t)
	defer ts.Close()

	// Create payment intent
	body := `{"amount": 10000, "currency": "usd"}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var pi map[string]any
	err = json.NewDecoder(resp.Body).Decode(&pi)
	require.NoError(t, err)

	assert.Equal(t, "payment_intent", pi["object"])
	assert.Equal(t, float64(10000), pi["amount"])
	assert.Equal(t, "usd", pi["currency"])
	assert.Equal(t, "requires_payment_method", pi["status"])

	// Verify stored in state
	piID := pi["id"].(string)
	payment, err := store.GetPayment(context.Background(), piID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), payment.Amount)
}

func TestStripePaymentIntent_ConfirmSuccess(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Create with success card
	body := `{
		"amount": 5000,
		"currency": "usd",
		"payment_method_data": {"type": "card", "card": {"number": "4242424242424242"}}
	}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)

	var pi map[string]any
	err = json.NewDecoder(resp.Body).Decode(&pi)
	resp.Body.Close()
	require.NoError(t, err)
	piID := pi["id"].(string)

	// Confirm
	resp, err = http.Post(ts.URL+"/stripe/v1/payment_intents/"+piID+"/confirm", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&pi)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pi["status"])
}

func TestStripePaymentIntent_ManualCapture(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Create with manual capture
	body := `{
		"amount": 7500,
		"currency": "usd",
		"capture_method": "manual",
		"payment_method_data": {"type": "card", "card": {"number": "4242424242424242"}},
		"confirm": true
	}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var pi map[string]any
	err = json.NewDecoder(resp.Body).Decode(&pi)
	require.NoError(t, err)

	assert.Equal(t, "requires_capture", pi["status"])
	piID := pi["id"].(string)

	// Capture
	resp, err = http.Post(ts.URL+"/stripe/v1/payment_intents/"+piID+"/capture", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&pi)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", pi["status"])
	assert.Equal(t, float64(7500), pi["amount_received"])
}

func TestStripeRefund(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Create and confirm payment
	body := `{
		"amount": 10000,
		"currency": "usd",
		"payment_method_data": {"type": "card", "card": {"number": "4242424242424242"}},
		"confirm": true
	}`
	resp, err := http.Post(ts.URL+"/stripe/v1/payment_intents", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)

	var pi map[string]any
	json.NewDecoder(resp.Body).Decode(&pi)
	resp.Body.Close()
	piID := pi["id"].(string)

	// Refund
	refundBody := fmt.Sprintf(`{"payment_intent": "%s", "amount": 5000}`, piID)
	resp, err = http.Post(ts.URL+"/stripe/v1/refunds", "application/json", bytes.NewBufferString(refundBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var refund map[string]any
	err = json.NewDecoder(resp.Body).Decode(&refund)
	require.NoError(t, err)
	assert.Equal(t, "refund", refund["object"])
	assert.Equal(t, float64(5000), refund["amount"])
}

func TestAdyenPayment_Success(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	body := `{
		"amount": {"currency": "EUR", "value": 15000},
		"merchantAccount": "TestMerchant",
		"reference": "order_123",
		"paymentMethod": {"type": "scheme", "number": "4242424242424242"}
	}`
	resp, err := http.Post(ts.URL+"/adyen/v71/payments", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "Authorised", result["resultCode"])
	assert.NotEmpty(t, result["pspReference"])
}

func TestAdyenCapture(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Create payment
	body := `{
		"amount": {"currency": "EUR", "value": 20000},
		"merchantAccount": "TestMerchant",
		"reference": "order_456",
		"paymentMethod": {"type": "scheme", "number": "4242424242424242"}
	}`
	resp, err := http.Post(ts.URL+"/adyen/v71/payments", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)

	var payment map[string]any
	json.NewDecoder(resp.Body).Decode(&payment)
	resp.Body.Close()
	pspRef := payment["pspReference"].(string)

	// Capture
	captureBody := `{"merchantAccount": "TestMerchant", "amount": {"currency": "EUR", "value": 20000}}`
	resp, err = http.Post(ts.URL+"/adyen/v71/payments/"+pspRef+"/captures", "application/json", bytes.NewBufferString(captureBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var capture map[string]any
	err = json.NewDecoder(resp.Body).Decode(&capture)
	require.NoError(t, err)
	assert.Equal(t, "received", capture["status"])
}

func TestAdyenRefund(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Create and capture payment
	body := `{
		"amount": {"currency": "EUR", "value": 25000},
		"merchantAccount": "TestMerchant",
		"reference": "order_789",
		"paymentMethod": {"type": "scheme", "number": "4242424242424242"}
	}`
	resp, err := http.Post(ts.URL+"/adyen/v71/payments", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)

	var payment map[string]any
	json.NewDecoder(resp.Body).Decode(&payment)
	resp.Body.Close()
	pspRef := payment["pspReference"].(string)

	// Capture
	captureBody := `{"merchantAccount": "TestMerchant"}`
	resp, err = http.Post(ts.URL+"/adyen/v71/payments/"+pspRef+"/captures", "application/json", bytes.NewBufferString(captureBody))
	require.NoError(t, err)
	resp.Body.Close()

	// Refund
	refundBody := `{"merchantAccount": "TestMerchant", "amount": {"currency": "EUR", "value": 10000}}`
	resp, err = http.Post(ts.URL+"/adyen/v71/payments/"+pspRef+"/refunds", "application/json", bytes.NewBufferString(refundBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var refund map[string]any
	err = json.NewDecoder(resp.Body).Decode(&refund)
	require.NoError(t, err)
	assert.Equal(t, "received", refund["status"])
}

func TestPayPalOrder_Success(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	body := `{
		"intent": "CAPTURE",
		"purchase_units": [{
			"amount": {"currency_code": "USD", "value": "100.00"}
		}]
	}`
	resp, err := http.Post(ts.URL+"/paypal/v2/checkout/orders", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var order map[string]any
	err = json.NewDecoder(resp.Body).Decode(&order)
	require.NoError(t, err)

	assert.Equal(t, "CREATED", order["status"])
	assert.NotEmpty(t, order["id"])
}

func TestPayPalOrder_AuthorizeAndCapture(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Create order
	body := `{
		"intent": "AUTHORIZE",
		"purchase_units": [{
			"amount": {"currency_code": "USD", "value": "150.00"}
		}],
		"payment_source": {"card": {"number": "4242424242424242"}}
	}`
	resp, err := http.Post(ts.URL+"/paypal/v2/checkout/orders", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)

	var order map[string]any
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()
	orderID := order["id"].(string)

	// Authorize
	resp, err = http.Post(ts.URL+"/paypal/v2/checkout/orders/"+orderID+"/authorize", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&order)
	require.NoError(t, err)
	assert.Equal(t, "APPROVED", order["status"])
}

func TestPayPalRefund(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Create and capture order
	body := `{
		"intent": "CAPTURE",
		"purchase_units": [{
			"amount": {"currency_code": "USD", "value": "200.00"}
		}],
		"payment_source": {"card": {"number": "4242424242424242"}}
	}`
	resp, err := http.Post(ts.URL+"/paypal/v2/checkout/orders", "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)

	var order map[string]any
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()
	orderID := order["id"].(string)

	// Capture
	resp, err = http.Post(ts.URL+"/paypal/v2/checkout/orders/"+orderID+"/capture", "application/json", nil)
	require.NoError(t, err)
	json.NewDecoder(resp.Body).Decode(&order)
	resp.Body.Close()

	// Get capture ID from response
	pu := order["purchase_units"].([]any)[0].(map[string]any)
	payments := pu["payments"].(map[string]any)
	captures := payments["captures"].([]any)
	captureID := captures[0].(map[string]any)["id"].(string)

	// Refund
	refundBody := `{"amount": {"currency_code": "USD", "value": "50.00"}}`
	resp, err = http.Post(ts.URL+"/paypal/v2/payments/captures/"+captureID+"/refund", "application/json", bytes.NewBufferString(refundBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestPayPalOAuth(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/paypal/v1/oauth2/token", "application/x-www-form-urlencoded", bytes.NewBufferString("grant_type=client_credentials"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var token map[string]any
	err = json.NewDecoder(resp.Body).Decode(&token)
	require.NoError(t, err)

	assert.NotEmpty(t, token["access_token"])
	assert.Equal(t, "Bearer", token["token_type"])
}

func TestAdminReset(t *testing.T) {
	ts, store := testServer(t)
	defer ts.Close()

	// Create some payments
	store.CreatePayment(context.Background(), &state.PaymentState{
		ID:       "test_1",
		Provider: "stripe",
		Amount:   1000,
	})
	store.CreatePayment(context.Background(), &state.PaymentState{
		ID:       "test_2",
		Provider: "adyen",
		Amount:   2000,
	})

	// Verify payments exist
	payments, _ := store.ListPayments(context.Background(), state.ListOptions{})
	assert.Len(t, payments, 2)

	// Reset
	resp, err := http.Post(ts.URL+"/admin/reset", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify payments cleared
	payments, _ = store.ListPayments(context.Background(), state.ListOptions{})
	assert.Len(t, payments, 0)
}

func TestAdminListPayments(t *testing.T) {
	ts, store := testServer(t)
	defer ts.Close()

	// Create payments
	store.CreatePayment(context.Background(), &state.PaymentState{
		ID:       "stripe_1",
		Provider: "stripe",
		Amount:   1000,
	})
	store.CreatePayment(context.Background(), &state.PaymentState{
		ID:       "adyen_1",
		Provider: "adyen",
		Amount:   2000,
	})

	// List all
	resp, err := http.Get(ts.URL + "/admin/payments")
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, float64(2), result["count"])

	// Filter by provider
	resp, err = http.Get(ts.URL + "/admin/payments?provider=stripe")
	require.NoError(t, err)
	defer resp.Body.Close()

	json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(t, float64(1), result["count"])
}
