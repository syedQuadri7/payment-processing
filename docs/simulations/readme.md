# Simulations

Testing tools and simulation requirements for the payment processing service.

## Overview

Testing payment systems requires simulating various webhook scenarios from payment providers. The project includes a webhook simulator tool that generates provider-specific webhook payloads with proper signatures.

## Webhook Simulator

The webhook simulator is located at `tools/webhook-simulator/`. It provides capabilities to test the full webhook receiving and processing pipeline.

### Supported Providers

| Provider | Endpoint | Signature Method |
|----------|----------|------------------|
| Stripe | `/webhooks/stripe` | HMAC-SHA256 with timestamp |
| Adyen | `/webhooks/adyen` | HMAC-SHA256 |
| PayPal | `/webhooks/paypal` | Mock headers (API verification mocked in test mode) |

### Simulator Capabilities

| Capability | Description |
|------------|-------------|
| Send single webhook | Simulate individual webhook events from any provider |
| Run scenario files | Execute multi-step test flows from YAML configuration |
| Invalid signatures | Test signature verification rejection |
| Custom payloads | Override default values for specific test cases |
| Timing control | Add delays between webhook steps |
| Status verification | Validate expected HTTP response codes |

## Test Scenarios

Pre-built scenario files cover common payment flows and edge cases.

### Scenario Categories

| Scenario | Purpose | Expected Outcome |
|----------|---------|------------------|
| Happy Path (auth_capture) | Authorization followed by successful capture | Payment reaches CAPTURED status |
| Soft Declines | Retry-eligible decline codes | Payment enters recovery workflow |
| Hard Declines | Non-retryable decline codes | Payment marked FAILED immediately |
| Fraud Declines | Fraud flags and disputes | Payment marked for review, no retry |
| Invalid Webhooks | Missing/invalid signatures, malformed payloads | 401 or 400 responses |

### Scenario File Requirements

Each scenario file must define:

| Field | Required | Description |
|-------|----------|-------------|
| name | Yes | Human-readable scenario name |
| target | Yes | Service URL to send webhooks to |
| provider | Yes | Which provider format to use |
| steps | Yes | Ordered list of webhook events |

Each step in a scenario specifies:

| Field | Required | Description |
|-------|----------|-------------|
| event | Yes | Provider event type to simulate |
| delay | No | Wait time before sending (for timing-sensitive tests) |
| data | Yes | Event payload data |
| expect_status | Yes | Expected HTTP response code |

## Testing Strategy

### 1. Unit Testing (Adapter Layer)

Test each adapter component in isolation:

| Component | What to Test |
|-----------|--------------|
| Signature Verification | Valid signatures pass, invalid signatures rejected |
| Event Mapping | Provider events map to correct canonical events |
| Decline Code Mapping | Provider decline codes map to correct canonical codes |
| Payload Parsing | Provider JSON correctly parsed to internal structures |

### 2. Integration Testing

Test the full webhook flow with a running service:

| Test Area | What to Verify |
|-----------|----------------|
| Endpoint routing | Webhooks reach correct provider adapter |
| Signature validation | Invalid signatures return 401 |
| Event processing | Valid webhooks update payment state |
| Idempotency | Duplicate webhooks processed only once |
| Signal delivery | Webhooks trigger Temporal workflow signals |

### 3. End-to-End Testing

Test complete payment flows:

| Step | Verification |
|------|--------------|
| 1. Create payment intent | Intent created with CREATED status |
| 2. Simulate authorization webhook | Status transitions to AUTHORIZED |
| 3. Request capture via API | Capture initiated |
| 4. Simulate capture webhook | Status transitions to CAPTURED |
| 5. Check ledger | Double-entry records created correctly |
| 6. Check outbox | CDC events written for each state change |

### 4. Negative Testing

Test error handling and security:

| Test Case | Expected Behavior |
|-----------|-------------------|
| Missing signature header | 401 Unauthorized |
| Invalid signature value | 401 Unauthorized |
| Expired timestamp (replay attack) | 401 Unauthorized |
| Malformed JSON payload | 400 Bad Request |
| Unknown event type | 200 OK (logged but not processed) |
| Duplicate webhook ID | 200 OK (not reprocessed) |

## Decline Code Test Matrix

### Soft Declines (Retry Eligible)

| Canonical Code | Stripe Code | Adyen Code | PayPal Code |
|----------------|-------------|------------|-------------|
| INSUFFICIENT_FUNDS | insufficient_funds | Refused | INSUFFICIENT_FUNDS |
| GENERIC_DECLINE | card_declined | Refused | INSTRUMENT_DECLINED |
| PROCESSING_ERROR | processing_error | Error | INTERNAL_SERVICE_ERROR |
| DO_NOT_HONOR | do_not_honor | Refused | DO_NOT_HONOR |

### Hard Declines (No Retry)

| Canonical Code | Stripe Code | Adyen Code | PayPal Code |
|----------------|-------------|------------|-------------|
| CARD_EXPIRED | expired_card | Expired Card | CREDIT_CARD_EXPIRED |
| INVALID_NUMBER | incorrect_number | Invalid Card Number | INVALID_ACCOUNT |
| INVALID_CVV | incorrect_cvc | CVC Declined | CREDIT_CARD_CVV_CHECK_FAILED |
| ACCOUNT_CLOSED | card_declined | Closed Account | INVALID_ACCOUNT |

### Fraud Declines

| Canonical Code | Stripe Code | Adyen Code | PayPal Code |
|----------------|-------------|------------|-------------|
| FRAUD_SUSPICION | fraudulent | Fraud | TRANSACTION_REFUSED |
| STOLEN_CARD | stolen_card | Stolen Card | TRANSACTION_REFUSED |
| LOST_CARD | lost_card | Lost Card | TRANSACTION_REFUSED |

## Monitoring During Tests

When running simulations, monitor these components:

| Component | What to Monitor | Location |
|-----------|-----------------|----------|
| Temporal UI | Workflow executions, signals received, activity results | http://localhost:8233 |
| Service logs | Signature verification, event processing, state transitions | stdout/stderr |
| Database | Payment intent status, ledger entries, processed_events | PostgreSQL |
| Kafka | CDC events from outbox table | Kafka consumer |

### Key Metrics to Track

| Metric | Expected During Tests |
|--------|----------------------|
| Webhook processing latency | < 100ms per webhook |
| Signature verification failures | Only for invalid signature tests |
| State transition errors | None for valid webhooks |
| Duplicate event count | Only for idempotency tests |

## CI/CD Integration Requirements

The webhook simulator should be integrated into the CI/CD pipeline:

| Stage | Tests to Run |
|-------|--------------|
| Unit tests | Adapter tests with mocked inputs |
| Integration tests | Webhook simulator against test service instance |
| End-to-end tests | Full payment flow scenarios |
| Performance tests | High-volume webhook throughput |

### CI Environment Requirements

| Requirement | Description |
|-------------|-------------|
| Running service | Payment processing service must be started |
| Temporal server | Required for workflow execution |
| PostgreSQL | Required for state persistence |
| Test isolation | Each test run should use isolated test data |
| Cleanup | Test data should be cleaned between runs |

## Related Documentation

- [Stripe Webhooks](../api/webhooks/stripe.md) - Stripe webhook format and decline code mapping
- [Adyen Webhooks](../api/webhooks/adyen.md) - Adyen webhook format and decline code mapping
- [PayPal Webhooks](../api/webhooks/paypal.md) - PayPal webhook format and decline code mapping
- [Decline Code Mappings](../schema/core-tables.md) - Database table for provider code mappings
