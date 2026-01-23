# Provider Simulator API Reference

The provider simulator provides HTTP APIs that mimic Stripe, Adyen, and PayPal payment APIs, allowing end-to-end testing of payment flows without connecting to real payment providers.

## Base URL

Default: `http://localhost:9000`

## Health Check

### GET /health

Returns simulator health and statistics.

**Response:**
```json
{
  "status": "healthy",
  "time": "2024-01-15T10:30:00Z",
  "stats": {
    "total_payments": 5,
    "total_webhooks": 12,
    "payments_by_provider": {"stripe": 3, "adyen": 2},
    "webhooks_by_status": {"delivered": 10, "pending": 2}
  },
  "config": {
    "webhook_target": "http://localhost:8080",
    "webhook_delay_ms": 0
  }
}
```

---

## Stripe API

### POST /stripe/v1/payment_intents

Create a new payment intent.

**Request:**
```json
{
  "amount": 10000,
  "currency": "usd",
  "capture_method": "automatic",
  "description": "Order #123",
  "metadata": {"order_id": "123"},
  "payment_method_data": {
    "type": "card",
    "card": {
      "number": "4242424242424242",
      "exp_month": 12,
      "exp_year": 2025,
      "cvc": "123"
    }
  },
  "confirm": false
}
```

**Response:**
```json
{
  "id": "pi_abc123",
  "object": "payment_intent",
  "amount": 10000,
  "amount_capturable": 0,
  "amount_received": 0,
  "capture_method": "automatic",
  "client_secret": "pi_abc123_secret_xyz",
  "confirmation_method": "automatic",
  "created": 1705312200,
  "currency": "usd",
  "livemode": false,
  "payment_method_types": ["card"],
  "status": "requires_payment_method"
}
```

### GET /stripe/v1/payment_intents/{id}

Retrieve a payment intent.

### POST /stripe/v1/payment_intents/{id}/confirm

Confirm a payment intent.

**Request:**
```json
{
  "payment_method_data": {
    "type": "card",
    "card": {"number": "4242424242424242"}
  }
}
```

### POST /stripe/v1/payment_intents/{id}/capture

Capture an authorized payment intent.

**Request:**
```json
{
  "amount_to_capture": 5000
}
```

### POST /stripe/v1/payment_intents/{id}/cancel

Cancel a payment intent.

**Request:**
```json
{
  "cancellation_reason": "requested_by_customer"
}
```

### POST /stripe/v1/refunds

Create a refund.

**Request:**
```json
{
  "payment_intent": "pi_abc123",
  "amount": 5000,
  "reason": "requested_by_customer"
}
```

**Response:**
```json
{
  "id": "re_xyz789",
  "object": "refund",
  "amount": 5000,
  "charge": "ch_abc123",
  "created": 1705312300,
  "currency": "usd",
  "status": "succeeded"
}
```

---

## Adyen API

### POST /adyen/v71/payments

Create a new payment.

**Request:**
```json
{
  "amount": {
    "currency": "EUR",
    "value": 15000
  },
  "merchantAccount": "TestMerchant",
  "reference": "order_123",
  "paymentMethod": {
    "type": "scheme",
    "number": "4242424242424242",
    "expiryMonth": "12",
    "expiryYear": "2025",
    "cvc": "123"
  }
}
```

**Response:**
```json
{
  "pspReference": "ADYEN_abc123",
  "resultCode": "Authorised",
  "merchantReference": "order_123",
  "amount": {
    "currency": "EUR",
    "value": 15000
  }
}
```

### POST /adyen/v71/payments/{id}/captures

Capture a payment.

**Request:**
```json
{
  "merchantAccount": "TestMerchant",
  "amount": {
    "currency": "EUR",
    "value": 15000
  }
}
```

**Response:**
```json
{
  "pspReference": "ADYEN_cap456",
  "paymentPspReference": "ADYEN_abc123",
  "status": "received"
}
```

### POST /adyen/v71/payments/{id}/cancels

Cancel a payment.

**Request:**
```json
{
  "merchantAccount": "TestMerchant"
}
```

### POST /adyen/v71/payments/{id}/refunds

Refund a payment.

**Request:**
```json
{
  "merchantAccount": "TestMerchant",
  "amount": {
    "currency": "EUR",
    "value": 5000
  }
}
```

---

## PayPal API

### POST /paypal/v1/oauth2/token

Get OAuth access token (mock).

**Response:**
```json
{
  "scope": "https://uri.paypal.com/services/payments/payment",
  "access_token": "A21AAF...",
  "token_type": "Bearer",
  "app_id": "APP-80W284485P519543T",
  "expires_in": 32400,
  "nonce": "2024-01-15T10:30:00Z..."
}
```

### POST /paypal/v2/checkout/orders

Create a new order.

**Request:**
```json
{
  "intent": "CAPTURE",
  "purchase_units": [{
    "reference_id": "ref_123",
    "amount": {
      "currency_code": "USD",
      "value": "100.00"
    },
    "description": "Order #123"
  }],
  "payment_source": {
    "card": {
      "number": "4242424242424242",
      "expiry": "2025-12",
      "security_code": "123"
    }
  }
}
```

**Response:**
```json
{
  "id": "ORDER123456789",
  "status": "CREATED",
  "intent": "CAPTURE",
  "purchase_units": [{
    "reference_id": "ref_123",
    "amount": {
      "currency_code": "USD",
      "value": "100.00"
    }
  }],
  "create_time": "2024-01-15T10:30:00Z",
  "links": [
    {"href": "...", "rel": "self", "method": "GET"},
    {"href": "...", "rel": "capture", "method": "POST"}
  ]
}
```

### GET /paypal/v2/checkout/orders/{id}

Retrieve an order.

### POST /paypal/v2/checkout/orders/{id}/authorize

Authorize an order.

### POST /paypal/v2/checkout/orders/{id}/capture

Capture an order directly.

### POST /paypal/v2/payments/authorizations/{id}/capture

Capture an authorization.

**Request:**
```json
{
  "amount": {
    "currency_code": "USD",
    "value": "100.00"
  },
  "final_capture": true
}
```

### POST /paypal/v2/payments/authorizations/{id}/void

Void an authorization.

### POST /paypal/v2/payments/captures/{id}/refund

Refund a capture.

**Request:**
```json
{
  "amount": {
    "currency_code": "USD",
    "value": "50.00"
  }
}
```

### POST /paypal/v1/notifications/verify-webhook-signature

Verify webhook signature (mock - always returns SUCCESS).

**Request:**
```json
{
  "auth_algo": "SHA256withRSA",
  "cert_url": "https://...",
  "transmission_id": "...",
  "transmission_sig": "...",
  "transmission_time": "...",
  "webhook_id": "...",
  "webhook_event": {...}
}
```

**Response:**
```json
{
  "verification_status": "SUCCESS"
}
```

---

## Admin API

### POST /admin/reset

Clear all state (payments, webhooks, refunds).

**Response:**
```json
{
  "status": "reset"
}
```

### GET /admin/payments

List all payments.

**Query Parameters:**
- `provider` - Filter by provider (stripe, adyen, paypal)
- `status` - Filter by status

**Response:**
```json
{
  "payments": [...],
  "count": 5
}
```

### GET /admin/payments/{id}

Get payment details including refunds.

**Response:**
```json
{
  "payment": {
    "id": "pi_abc123",
    "provider": "stripe",
    "status": "captured",
    "amount": 10000,
    "currency": "usd",
    ...
  },
  "refunds": [...]
}
```

### POST /admin/payments/{id}/behavior

Configure behavior for the next action on a payment.

**Request:**
```json
{
  "decline_on_next": "INSUFFICIENT_FUNDS",
  "delay_ms": 2000,
  "timeout_on_next": false
}
```

**Response:**
```json
{
  "status": "behavior_set",
  "payment": {...}
}
```

### GET /admin/webhooks

List all webhooks.

**Query Parameters:**
- `status` - Filter by status (pending, delivered, failed, retrying)

### POST /admin/webhooks/{id}/redeliver

Queue a webhook for redelivery.

### GET /admin/stats

Get store statistics.

**Response:**
```json
{
  "total_payments": 10,
  "total_webhooks": 25,
  "total_refunds": 3,
  "payments_by_provider": {"stripe": 5, "adyen": 3, "paypal": 2},
  "payments_by_status": {"captured": 7, "authorized": 2, "failed": 1},
  "webhooks_by_status": {"delivered": 20, "pending": 3, "failed": 2}
}
```

---

## Error Responses

### Stripe Errors

```json
{
  "error": {
    "type": "card_error",
    "code": "insufficient_funds",
    "decline_code": "insufficient_funds",
    "message": "Your card has insufficient funds."
  }
}
```

### Adyen Errors

```json
{
  "status": 400,
  "errorCode": "167",
  "message": "Original payment not in correct state",
  "errorType": "validation"
}
```

### PayPal Errors

```json
{
  "name": "INSTRUMENT_DECLINED",
  "message": "Payment declined",
  "links": [{
    "href": "https://developer.paypal.com/docs/api/...",
    "rel": "information_link"
  }]
}
```

---

## Webhook Delivery

When payment state changes, the simulator automatically delivers webhooks to the configured target URL.

### Webhook Format

Each provider uses its native webhook format:

**Stripe:**
```json
{
  "id": "evt_abc123",
  "object": "event",
  "type": "payment_intent.succeeded",
  "data": {
    "object": {...}
  }
}
```

**Adyen:**
```json
{
  "live": "false",
  "notificationItems": [{
    "NotificationRequestItem": {
      "eventCode": "AUTHORISATION",
      "pspReference": "...",
      ...
    }
  }]
}
```

**PayPal:**
```json
{
  "id": "WH-abc123",
  "event_type": "PAYMENT.CAPTURE.COMPLETED",
  "resource": {...}
}
```

### Webhook Signatures

All webhooks are signed using the provider's signature format:

- **Stripe**: `Stripe-Signature: t={timestamp},v1={hmac}`
- **Adyen**: `X-Adyen-Hmac-Signature: {base64-hmac}`
- **PayPal**: Mock headers (PAYPAL-TRANSMISSION-ID, etc.)
