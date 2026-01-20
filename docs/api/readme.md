# API Documentation

## Overview

The service exposes two categories of endpoints:

1. **Internal APIs** (`/api/v1/...`) - REST endpoints for creating and managing payments
2. **Webhook Receivers** (`/webhooks/...`) - Endpoints that receive notifications from payment providers

## Authentication

### Internal APIs

Internal API endpoints require bearer token authentication. Authentication implementation is to be determined based on the deployment environment.

### Webhook Endpoints

Each provider uses a different signature verification method:

| Provider | Verification Method |
|----------|---------------------|
| Stripe | HMAC-SHA256 signature with timestamp in `Stripe-Signature` header |
| Adyen | HMAC-SHA256 using shared secret |
| PayPal | Webhook ID verification via PayPal API callback |

## Request Standards

### Required Headers

| Header | When Required | Purpose |
|--------|---------------|---------|
| `Content-Type` | All requests with body | Must be `application/json` |
| `Idempotency-Key` | POST and PUT requests | UUID for request deduplication |
| `Authorization` | Internal API calls | Bearer token for authentication |

### Response Headers

| Header | Purpose |
|--------|---------|
| `X-Request-Id` | Unique request identifier for tracing and support |
| `X-Idempotency-Key` | Echo of provided idempotency key |

## Error Handling

### Error Response Structure

All errors return a consistent structure containing:
- Error type (category of error)
- Error code (specific error identifier)
- Human-readable message
- Parameter that caused the error (if applicable)
- Request ID for support reference

### Error Types

| Type | HTTP Status | When Used |
|------|-------------|-----------|
| `validation_error` | 400 | Invalid input format or values |
| `authentication_error` | 401 | Invalid credentials or webhook signature |
| `not_found` | 404 | Requested resource does not exist |
| `conflict` | 409 | Idempotency conflict or invalid state transition |
| `business_rule_violation` | 422 | Business rule prevented the operation |
| `internal_error` | 500 | Unexpected server error |

## Idempotency

All state-changing operations must support idempotency via the `Idempotency-Key` header.

### Behavior Requirements

| Scenario | Expected Behavior |
|----------|-------------------|
| First request with key | Process normally, cache response |
| Duplicate request (same key) | Return cached response immediately |
| Concurrent request (same key) | Return `409 Conflict` |
| Key expiration | Keys expire after 24 hours |

## Pagination

List endpoints use cursor-based pagination.

### Pagination Parameters

| Parameter | Purpose |
|-----------|---------|
| `limit` | Maximum items to return (default: 50, max: 100) |
| `cursor` | Cursor from previous response for next page |

### Pagination Response Fields

| Field | Purpose |
|-------|---------|
| `data` | Array of items |
| `has_more` | Boolean indicating more pages exist |
| `next_cursor` | Cursor to use for next page (if `has_more` is true) |

## Rate Limiting

### Limits by Endpoint Type

| Endpoint Type | Limit |
|---------------|-------|
| Read operations | 1000 requests/minute |
| Write operations | 100 requests/minute |
| Webhook receivers | Unlimited (provider-controlled) |

### Rate Limit Response Headers

| Header | Purpose |
|--------|---------|
| `X-RateLimit-Limit` | Maximum requests allowed |
| `X-RateLimit-Remaining` | Requests remaining in window |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |

## Endpoint Categories

### Internal APIs

| Document | Purpose |
|----------|---------|
| [Payment Intents](internal/intents.md) | Create, authorize, capture, and manage payments |
| [Accounts](internal/accounts.md) | Query account balances and ledger entries |
| [Health](internal/health.md) | Health checks and observability metrics |

### Webhook Receivers

| Document | Purpose |
|----------|---------|
| [Stripe Webhooks](webhooks/stripe.md) | Handle Stripe payment events |
| [Adyen Webhooks](webhooks/adyen.md) | Handle Adyen notifications |
| [PayPal Webhooks](webhooks/paypal.md) | Handle PayPal events |
