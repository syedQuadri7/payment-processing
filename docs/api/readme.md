# API Documentation

This directory contains documentation for all API endpoints in the Payment Processing Service.

## Overview

The service exposes two categories of endpoints:

1. **Internal APIs** (`/api/v1/...`) - REST endpoints for creating and managing payments
2. **Webhook Receivers** (`/webhooks/...`) - Endpoints that receive notifications from payment providers

## Base URL

```
Development: http://localhost:8080
```

## Authentication

Internal API endpoints require authentication (implementation TBD).

Webhook endpoints use provider-specific signature verification:
- **Stripe**: HMAC-SHA256 signature in `Stripe-Signature` header
- **Adyen**: HMAC-SHA256 using shared secret
- **PayPal**: Webhook ID verification via PayPal API

## Common Headers

### Request Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Content-Type` | Yes | `application/json` |
| `Idempotency-Key` | Yes (POST/PUT) | UUID for request deduplication |
| `Authorization` | Yes | Bearer token (internal APIs) |

### Response Headers

| Header | Description |
|--------|-------------|
| `X-Request-Id` | Unique request identifier for tracing |
| `X-Idempotency-Key` | Echo of provided idempotency key |

## Error Response Format

All errors follow a consistent format:

```json
{
  "error": {
    "type": "validation_error",
    "code": "invalid_amount",
    "message": "Amount must be greater than zero",
    "param": "amount",
    "request_id": "req_abc123"
  }
}
```

### Error Types

| Type | HTTP Status | Description |
|------|-------------|-------------|
| `validation_error` | 400 | Invalid input format or values |
| `authentication_error` | 401 | Invalid credentials or signature |
| `not_found` | 404 | Resource does not exist |
| `conflict` | 409 | Idempotency conflict or invalid state |
| `business_rule_violation` | 422 | Business rule prevented operation |
| `internal_error` | 500 | Server error |

## Endpoint Categories

### Internal APIs

- [Payment Intents](internal/intents.md) - Create, authorize, capture payments
- [Accounts](internal/accounts.md) - Query account balances and ledger
- [Health](internal/health.md) - Health checks and metrics

### Webhook Receivers

- [Stripe Webhooks](webhooks/stripe.md) - Handle Stripe payment events
- [Adyen Webhooks](webhooks/adyen.md) - Handle Adyen notifications
- [PayPal Webhooks](webhooks/paypal.md) - Handle PayPal events

## Idempotency

All state-changing operations support idempotency via the `Idempotency-Key` header.

**Behavior:**
- First request with a key: Processed normally, response cached
- Duplicate request with same key: Returns cached response immediately
- Concurrent request with same key: Returns `409 Conflict`
- Keys expire after 24 hours

**Best Practice:** Use UUIDs generated client-side.

```bash
curl -X POST http://localhost:8080/api/v1/intents \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{"amount": "100.00", "currency": "USD", ...}'
```

## Pagination

List endpoints support cursor-based pagination:

```json
{
  "data": [...],
  "has_more": true,
  "next_cursor": "cursor_abc123"
}
```

Use the `cursor` query parameter to fetch subsequent pages.

## Rate Limiting

Rate limits apply per API key:

| Endpoint Type | Limit |
|---------------|-------|
| Read operations | 1000/minute |
| Write operations | 100/minute |
| Webhook receivers | Unlimited |

Rate limit headers are included in responses:
- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`
