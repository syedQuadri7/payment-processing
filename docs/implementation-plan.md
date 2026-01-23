# Implementation Plan

A phased implementation plan for the Payment Processing Service, referencing requirements, architecture, and design decisions.

---

## Progress Tracking

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1 | **Complete** | Database Foundation |
| Phase 2 | **Complete** | Domain Layer Completion |
| Phase 3 | **Complete** | Repository Layer Completion |
| Phase 4 | **Complete** | Multi-Provider Webhook Adapters |
| Phase 5 | **Complete** | Core Payment Workflow |
| Phase 6 | **Complete** | Decline Handling and Recovery |
| Phase 7 | **Complete** | Double-Entry Ledger |
| Phase 8 | **Next** | API Layer |
| Phase 9 | Planned | Event Publishing (Transactional Outbox) |
| Phase 10 | Planned | Audit and Observability |

---

## Current State

The codebase now includes:
- Complete database schema with 14 migrations (all core tables, ledger, outbox, audit)
- Full domain model with state machines and canonical types
- Repository implementations with PostgreSQL
- Multi-provider webhook adapters (Stripe, Adyen, PayPal)
- Complete PaymentIntentWorkflow with authorization, capture, void, retry logic
- Decline classification with database-backed code mapping
- Payment attempt tracking with immutable records
- Double-entry ledger activities for bookkeeping (authorization holds, captures, refunds)
- Clearing account monitoring with threshold-based alerts

**Next**: Phase 8 (API Layer) implements the REST API endpoints for payment operations.

---

## Implementation Phases

### Phase 1: Database Foundation **[COMPLETE]**

Establishes the persistence layer following the schema documentation.

**References:**
- `docs/schema/readme.md` - Design principles
- `docs/schema/core-tables.md` - Payment tables
- `docs/schema/ledger-tables.md` - Double-entry ledger
- `docs/schema/outbox-audit.md` - Event publishing and audit

**Tasks:**

| Task | Schema Reference | Priority |
|------|------------------|----------|
| 1.1 Create accounts table migration | ledger-tables.md | Must Have |
| 1.2 Create payment_intents table migration | core-tables.md | Must Have |
| 1.3 Create payment_methods table migration | core-tables.md | Must Have |
| 1.4 Create authorization_holds table migration | core-tables.md | Must Have |
| 1.5 Create payment_attempts table migration | core-tables.md | Must Have |
| 1.6 Create journal_entries and ledger_entries migrations | ledger-tables.md | Must Have |
| 1.7 Create outbox table migration | outbox-audit.md | Must Have |
| 1.8 Create audit_log table migration | outbox-audit.md | Must Have |
| 1.9 Create decline_code_mappings table migration | core-tables.md | Must Have |
| 1.10 Create processed_events table migration | outbox-audit.md | Must Have |
| 1.11 Seed decline code mappings for Stripe, Adyen, PayPal | core-tables.md | Must Have |
| 1.12 Create clearing accounts seed data | ledger-tables.md | Must Have |

**Deliverables:**
- All migrations in `internal/database/migrations/`
- Migration tests verifying up/down operations
- Seed data for decline codes and clearing accounts

**Validation:**
- `go test ./internal/repository/...` passes
- Migrations can be applied and rolled back cleanly

---

### Phase 2: Domain Layer Completion **[COMPLETE]**

Completes the domain model with all entities and business rules.

**References:**
- `docs/architecture/domain-model.md` - Entity relationships, state machines
- `docs/requirements/functional.md` - FR-INT, FR-AUTH, FR-LED requirements

**Tasks:**

| Task | Requirement | Priority |
|------|-------------|----------|
| 2.1 Complete PaymentIntent state machine validation | FR-INT-01 to FR-INT-08 | Must Have |
| 2.2 Implement AuthorizationHold domain logic | FR-AUTH-01 to FR-AUTH-08 | Must Have |
| 2.3 Implement PaymentAttempt linear state machine | domain-model.md | Must Have |
| 2.4 Implement Account balance calculations | FR-LED-05 | Must Have |
| 2.5 Add canonical event types | domain-model.md | Must Have |
| 2.6 Add canonical decline codes | domain-model.md | Must Have |
| 2.7 Implement journal entry balance validation | FR-LED-03 | Must Have |

**Deliverables:**
- Complete domain types in `internal/domain/`
- Unit tests for state machine transitions
- Unit tests for balance calculations

**Validation:**
- `go test ./internal/domain/...` with >80% coverage

---

### Phase 3: Repository Layer Completion **[COMPLETE]**

Implements all database operations with proper concurrency handling.

**References:**
- `docs/schema/readme.md` - Concurrency control
- `docs/requirements/functional.md` - FR-CON requirements
- `docs/decisions/finalized-decisions.md` - FD-003 Idempotency

**Tasks:**

| Task | Requirement | Priority |
|------|-------------|----------|
| 3.1 Implement PaymentIntentRepository CRUD | FR-INT-01 | Must Have |
| 3.2 Implement PaymentMethodRepository CRUD | core-tables.md | Must Have |
| 3.3 Implement AuthorizationHoldRepository with expiration queries | FR-AUTH-05 | Must Have |
| 3.4 Implement PaymentAttemptRepository | FR-DEC-06 | Must Have |
| 3.5 Implement AccountRepository with optimistic locking | FR-CON-03 | Must Have |
| 3.6 Implement JournalEntryRepository with balance validation | FR-LED-03 | Must Have |
| 3.7 Implement LedgerEntryRepository (append-only) | FR-LED-04 | Must Have |
| 3.8 Implement OutboxRepository | FR-EVT-01 | Must Have |
| 3.9 Implement AuditLogRepository | FR-AUD-01 to FR-AUD-08 | Must Have |
| 3.10 Implement ProcessedEventsRepository | FR-CON-05 | Must Have |
| 3.11 Implement DeclineCodeMappingRepository | FR-DEC-02 | Must Have |
| 3.12 Add SELECT FOR UPDATE for payment updates | FR-CON-02 | Must Have |

**Deliverables:**
- Complete repository implementations in `internal/repository/`
- Integration tests against test database
- Transaction helper utilities

**Validation:**
- Integration tests pass with PostgreSQL
- Concurrent update tests verify locking behavior

---

### Phase 4: Multi-Provider Webhook Adapters **[COMPLETE]**

Implements the adapter layer for provider webhook normalization.

**References:**
- `docs/architecture/system-design.md` - Adapter Layer section
- `docs/decisions/finalized-decisions.md` - FD-012 Multi-Provider Adapters
- `docs/api/webhooks/stripe.md`, `adyen.md`, `paypal.md`
- `docs/requirements/functional.md` - FR-ADP requirements

**Tasks:**

| Task | Requirement | Priority |
|------|-------------|----------|
| 4.1 Define canonical event interface | FD-012 | Must Have |
| 4.2 Implement Stripe webhook signature verification | FR-ADP-02 | Must Have |
| 4.3 Implement Stripe event type mapping | FR-ADP-06 | Must Have |
| 4.4 Implement Stripe decline code mapping | FR-ADP-05 | Must Have |
| 4.5 Implement Adyen HMAC verification | FR-ADP-02 | Must Have |
| 4.6 Implement Adyen notification mapping | FR-ADP-06 | Must Have |
| 4.7 Implement Adyen refusal code mapping | FR-ADP-05 | Must Have |
| 4.8 Implement PayPal webhook verification (API call) | FR-ADP-02 | Must Have |
| 4.9 Implement PayPal event mapping | FR-ADP-06 | Must Have |
| 4.10 Implement raw payload preservation | FR-ADP-04 | Must Have |
| 4.11 Implement provider-specific HTTP response formats | FR-ADP-07 | Must Have |

**Deliverables:**
- Adapter implementations in `internal/adapter/`
- Webhook handlers in `server/webhooks/`
- Unit tests with sample webhook payloads

**Validation:**
- Webhook simulator events are correctly normalized
- Signature verification rejects tampered payloads

---

### Phase 5: Core Payment Workflow **[COMPLETE]**

Implements the Temporal workflow following FD-011 patterns.

**References:**
- `docs/decisions/finalized-decisions.md` - FD-011 Temporal Workflows
- `docs/architecture/domain-model.md` - PaymentIntent state machine
- `docs/requirements/functional.md` - FR-INT, FR-AUTH requirements

**Tasks:**

| Task | Requirement | Priority |
|------|-------------|----------|
| 5.1 Define workflow input/output types | FD-011 | Must Have |
| 5.2 Implement PaymentWorkflow with state management | FR-INT-01 | Must Have |
| 5.3 Add signal handler for canonical webhook events | FD-012 | Must Have |
| 5.4 Add query handler for payment status | - | Must Have |
| 5.5 Implement authorization activity (provider-specific) | FR-AUTH-01 | Must Have |
| 5.6 Implement capture activity | FR-AUTH-03 | Must Have |
| 5.7 Implement void activity | FR-AUTH-07 | Must Have |
| 5.8 Implement database persistence activities | FD-011 | Must Have |
| 5.9 Implement outbox write activity | FR-EVT-01 | Must Have |
| 5.10 Add activity retry policies | FD-018 | Must Have |

**Deliverables:**
- Workflow definition in `workflow/payment_workflow.go`
- Activities in `workflow/activities/`
- Workflow tests using Temporal test framework

**Validation:**
- Unit tests cover happy path and failure scenarios
- Workflow survives simulated crashes and restarts

---

### Phase 6: Decline Handling and Recovery **[COMPLETE]**

Implements intelligent retry logic for soft declines.

**References:**
- `docs/decisions/finalized-decisions.md` - FD-001 Retry Timing
- `docs/requirements/functional.md` - FR-DEC requirements
- `docs/architecture/domain-model.md` - Decline Categories

**Tasks:**

| Task | Requirement | Status |
|------|-------------|--------|
| 6.1 Implement decline classification activity | FR-DEC-01 | Done |
| 6.2 Implement retry scheduling with durable timers | FD-001 | Done |
| 6.3 Implement recovery workflow state (RECOVERING) | domain-model.md | Done |
| 6.4 Track retry attempts and enforce maximum | FR-DEC-07 | Done |
| 6.5 Implement PaymentAttempt creation for each retry | FR-DEC-06 | Done |
| 6.6 Skip retry for hard declines and fraud | FR-DEC-05 | Done |
| 6.7 Support immediate retry on payment method update | FR-DEC-09 | Done |

**Implementation Summary:**

Activities added to `workflow/persistence_activities.go`:
- `ClassifyDecline` - Looks up provider-specific decline codes from `decline_code_mappings` table, returns canonical classification with retry eligibility
- `CreatePaymentAttempt` - Creates immutable attempt record in PENDING status
- `CompletePaymentAttempt` - Updates attempt with final status and decline details

Workflow changes in `workflow/payment_intent_workflow.go`:
- Authorization loop generates unique attempt IDs via `workflow.SideEffect`
- Creates attempt record before each authorization call
- Calls `ClassifyDecline` to map provider codes to canonical codes
- Completes attempt with classification results
- `waitWithSignals` now listens for payment method update signal during RECOVERING state
- Immediate retry triggered when payment method updated (skips remaining wait)

Repository interfaces added to `workflow/activities.go`:
- `DeclineCodeRepository` - For database decline code lookup
- `PaymentAttemptRepository` - For attempt persistence
- `NewActivitiesWithDependencies` constructor for production use

**Tests:**
- `TestPaymentIntentWorkflow_ImmediateRetryOnPaymentMethodUpdate` - Verifies FR-DEC-09
- `TestPaymentIntentWorkflow_ClassifyDecline` - Verifies FR-DEC-01
- Updated existing tests to mock new activities

**Validation:**
- Soft declines trigger automatic retry with correct intervals (4h, 12h, 24h, 48h)
- Hard declines and fraud immediately fail without retry
- Payment method update during recovery triggers immediate retry

---

### Phase 7: Double-Entry Ledger **[COMPLETE]**

Implements the bookkeeping system.

**References:**
- `docs/decisions/finalized-decisions.md` - FD-014 Double-Entry Bookkeeping
- `docs/schema/ledger-tables.md` - Ledger Transaction Patterns
- `docs/requirements/functional.md` - FR-LED requirements

**Tasks:**

| Task | Requirement | Status |
|------|-------------|--------|
| 7.1 Implement journal entry creation with balance validation | FR-LED-03 | Done |
| 7.2 Implement ledger entry creation (append-only) | FR-LED-04 | Done |
| 7.3 Implement capture ledger activity | ledger-tables.md | Done |
| 7.4 Implement refund ledger activity | ledger-tables.md | Done |
| 7.5 Implement pending balance updates for authorization | ledger-tables.md | Done |
| 7.6 Implement pending balance updates for void | ledger-tables.md | Done |
| 7.7 Implement clearing account balance monitoring | FR-LED-06 | Done |
| 7.8 Add reconciliation query support | FR-LED-07 | Done |

**Implementation Summary:**

Activities added to `workflow/ledger_activities.go`:
- `PlaceAuthorizationHold` - Increases pending_balance on customer account (no ledger entry - authorization is a promise, not money movement)
- `ReleaseAuthorizationHold` - Decreases pending_balance on customer account (for void or expiration)
- `RecordCapture` - Creates balanced journal entry with debit to customer, credit to settlement clearing; updates account balances with optimistic locking
- `RecordRefund` - Creates balanced journal entry with debit to merchant, credit to customer
- `GetClearingAccountBalances` - Retrieves all clearing account balances for a currency
- `MonitorClearingAccounts` - Identifies clearing accounts with non-zero balances exceeding thresholds (12h warning, 24h critical)
- `GetAccountBalance` - Retrieves current balance state for an account

Repository interfaces defined:
- `AccountRepository` - For account balance operations with optimistic locking
- `JournalEntryRepository` - For journal entry persistence and idempotency checks
- `LedgerEntryRepository` - For append-only ledger entry creation

Key design decisions:
- Authorization places hold on pending_balance (no ledger entry until capture)
- Capture creates journal entry: DEBIT customer, CREDIT settlement clearing
- Refund creates journal entry: DEBIT merchant, CREDIT customer
- Optimistic locking via version field prevents concurrent balance corruption
- Idempotency via journal entry reference lookup prevents duplicate entries

**Tests:**
- `TestPlaceAuthorizationHold_Success` - Verifies hold placement and balance updates
- `TestPlaceAuthorizationHold_InsufficientFunds` - Verifies insufficient funds handling
- `TestReleaseAuthorizationHold_Success` - Verifies hold release
- `TestRecordCapture_Success` - Verifies capture with journal/ledger entries
- `TestRecordCapture_Idempotent` - Verifies duplicate capture handling
- `TestRecordRefund_Success` - Verifies refund with balanced entries
- `TestMonitorClearingAccounts_WithWarning` - Verifies 12h warning threshold
- `TestMonitorClearingAccounts_WithCritical` - Verifies 24h critical threshold
- `TestPlaceAuthorizationHold_OptimisticLockFailure` - Verifies concurrent update handling

**Validation:**
- All journal entries balance (debits = credits) - enforced by domain.JournalEntry.Validate()
- Concurrent balance updates handled via optimistic locking with version field
- Clearing account monitoring alerts on non-zero balances exceeding thresholds

---

### Phase 8: API Layer

Implements the REST API following documentation.

**References:**
- `docs/api/readme.md` - Authentication, error formats
- `docs/api/internal/intents.md` - Payment Intent endpoints
- `docs/api/internal/accounts.md` - Account endpoints
- `docs/decisions/finalized-decisions.md` - FD-016, FD-017

**Tasks:**

| Task | Requirement | Priority |
|------|-------------|----------|
| 8.1 Implement API key authentication middleware | FD-017 | Must Have |
| 8.2 Implement API version middleware | FD-016 | Must Have |
| 8.3 Implement POST /api/v1/intents | FR-INT-01 | Must Have |
| 8.4 Implement GET /api/v1/intents/:id | functional.md | Must Have |
| 8.5 Implement PUT /api/v1/intents/:id/method | FR-INT-04 | Must Have |
| 8.6 Implement POST /api/v1/intents/:id/authorize | FR-AUTH-01 | Must Have |
| 8.7 Implement POST /api/v1/intents/:id/capture | FR-AUTH-03 | Must Have |
| 8.8 Implement POST /api/v1/intents/:id/cancel | FR-INT-05 | Must Have |
| 8.9 Implement GET /api/v1/intents/:id/attempts | functional.md | Must Have |
| 8.10 Implement idempotency key handling | FD-003 | Must Have |
| 8.11 Implement rate limiting middleware | FD-005 | Must Have |
| 8.12 Implement partial capture support | FD-019 | Should Have |
| 8.13 Implement webhook endpoints (Stripe, Adyen, PayPal) | FR-ADP-01 | Must Have |
| 8.14 Implement GET /health and GET /metrics | NFR-OBS-03, NFR-OBS-04 | Must Have |

**Deliverables:**
- HTTP handlers in `server/handlers/`
- Middleware in `server/middleware/`
- OpenAPI specification

**Validation:**
- API tests cover all endpoints
- Idempotency tests verify duplicate handling

---

### Phase 9: Event Publishing (Transactional Outbox)

Implements reliable event publishing via CDC.

**References:**
- `docs/decisions/finalized-decisions.md` - FD-013 Transactional Outbox
- `docs/architecture/system-design.md` - Transactional Outbox Pattern
- `docs/requirements/functional.md` - FR-EVT requirements

**Tasks:**

| Task | Requirement | Priority |
|------|-------------|----------|
| 9.1 Implement outbox write within transactions | FR-EVT-01, FR-EVT-02 | Must Have |
| 9.2 Define canonical event payload schemas | FR-EVT-04 | Must Have |
| 9.3 Implement simple outbox polling consumer | FR-EVT-05 | Must Have |
| 9.4 Configure Debezium CDC connector | FR-EVT-03 | Should Have |
| 9.5 Create Kafka topic configuration | FR-EVT-03 | Should Have |
| 9.6 Implement idempotent event consumer example | FR-EVT-07 | Should Have |

**Deliverables:**
- Outbox writing integrated into workflows
- Polling-based consumer for simple path
- Debezium configuration for production path

**Validation:**
- Events appear in outbox table within same transaction
- Polling consumer processes events correctly

---

### Phase 10: Audit and Observability

Implements audit logging and monitoring.

**References:**
- `docs/requirements/functional.md` - FR-AUD requirements
- `docs/requirements/non-functional.md` - NFR-OBS requirements
- `docs/decisions/finalized-decisions.md` - FD-009, FD-015

**Tasks:**

| Task | Requirement | Priority |
|------|-------------|----------|
| 10.1 Implement audit log writing for state changes | FR-AUD-01 | Must Have |
| 10.2 Track actor information in audit entries | FR-AUD-02 | Must Have |
| 10.3 Preserve old/new values for updates | FR-AUD-03 | Must Have |
| 10.4 Implement structured JSON logging | NFR-OBS-01 | Must Have |
| 10.5 Add correlation IDs to all operations | FR-AUD-07 | Must Have |
| 10.6 Implement Prometheus metrics | NFR-OBS-04 | Must Have |
| 10.7 Add per-provider metrics | NFR-OBS-07 | Should Have |
| 10.8 Implement clearing account balance alerts | NFR-OBS-05 | Should Have |
| 10.9 Add audit log query endpoints | FR-AUD-05 | Should Have |

**Deliverables:**
- Audit logging middleware and utilities
- Prometheus metrics endpoint
- Structured logging configuration

**Validation:**
- All payment state changes are logged
- Metrics endpoint returns valid Prometheus format

---

## Non-Functional Requirements Verification

Each phase must verify relevant NFRs from `docs/requirements/non-functional.md`:

| Phase | NFRs to Verify |
|-------|----------------|
| Phase 1 | NFR-SEC-02 (no hardcoded credentials) |
| Phase 3 | NFR-SEC-05 (row-level locking), NFR-PERF-06 (ledger latency) |
| Phase 4 | NFR-PERF-07 (webhook processing <50ms), NFR-SEC-03 (signature verification) |
| Phase 5 | NFR-REL-01 (survives restarts), NFR-PERF-03 (workflow startup) |
| Phase 6 | NFR-REL-02 (duplicate prevention) |
| Phase 7 | NFR-REL-03 (ledger consistency) |
| Phase 8 | NFR-PERF-01, NFR-PERF-02 (API response times), NFR-SEC-04 (key protection) |
| Phase 9 | NFR-PERF-05 (CDC latency), NFR-REL-05 (at-least-once delivery) |
| Phase 10 | NFR-MNT-05 (structured logging), NFR-OBS-06 (latency tracking) |

---

## Test Coverage Targets

From `docs/requirements/non-functional.md`:

| Component | Target Coverage |
|-----------|-----------------|
| Workflow code | >= 70% |
| Adapter code | >= 80% |
| Activity code | >= 70% |

---

## Success Criteria

From `docs/requirements/non-functional.md` - Success Criteria section:

**Functional Completeness:**
- [ ] Multi-provider webhooks work (all three adapters normalize correctly)
- [ ] Canonical model used internally (no provider-specific types in workflows)
- [ ] Intent/Method separation (payment method can be changed without new intent)
- [ ] Authorization holds work (pending balance reflects active holds)
- [ ] Capture records ledger entries (balanced double-entry entries created)
- [ ] Clearing accounts monitored (non-zero balances generate alerts)
- [ ] Retry uses canonical codes (decline classification provider-agnostic)
- [ ] Outbox publishes canonical events (within 100ms via CDC)

**Architectural Compliance:**
- [ ] Normalize at edge (workflows never see provider types)
- [ ] No dual-writes (all events through outbox only)
- [ ] Linear state machines (no circular PaymentAttempt states)
- [ ] Idempotent operations (duplicate requests return same result)
- [ ] Race conditions prevented (concurrent webhook/API handled safely)

---

## Recommended Implementation Order

1. **Phase 1** (Database) - Foundation for all other phases
2. **Phase 2** (Domain) - Business logic depends on types
3. **Phase 3** (Repository) - Persistence needed before workflows
4. **Phase 4** (Adapters) - Provider integration needed for workflows
5. **Phase 5** (Workflow) - Core business logic
6. **Phase 6** (Decline Handling) - Extends workflow
7. **Phase 7** (Ledger) - Extends workflow with bookkeeping
8. **Phase 8** (API) - Exposes functionality
9. **Phase 9** (Events) - Reliable event publishing
10. **Phase 10** (Audit) - Operational visibility

Phases 4-7 can be developed in parallel by different team members once Phase 3 is complete.

---

## See Also

- [Functional Requirements](requirements/functional.md)
- [Non-Functional Requirements](requirements/non-functional.md)
- [System Design](architecture/system-design.md)
- [Domain Model](architecture/domain-model.md)
- [Finalized Decisions](decisions/finalized-decisions.md)
- [Schema Documentation](schema/readme.md)
