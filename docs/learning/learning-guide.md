# Learning Guide: Payment Processing System

This document provides a structured approach to understanding the codebase and preparing for technical discussions about payment systems.

## Code Review Order

Start from the core domain and work outward to the edges.

### 1. Domain Layer (`shared/domain/`)

Start here. These are the core concepts everything else builds on.

| File | What to Understand |
|------|-------------------|
| `payment_intent.go` | The central entity. What states can it be in? What transitions are valid? |
| `payment_method.go` | How payment instruments (cards) are represented |
| `authorization_hold.go` | What happens between auth and capture? Why track this separately? |
| `payment_attempt.go` | Why is each attempt a separate record? What makes this "linear"? |
| `decline_code.go` | How are provider-specific declines normalized? What's soft vs hard? |
| `types.go` | Enums and constants that define the vocabulary |

**Key questions to answer:**
- Draw the state machine for PaymentIntent on paper
- Why separate PaymentIntent from PaymentMethod from PaymentAttempt?

### 2. Database Schema (`shared/database/migrations/`)

Read the `.up.sql` files in order. They tell the story of the data model.

| Migration | Focus On |
|-----------|----------|
| `000001` - `000004` | Core payment tables and relationships |
| `000007` - `000009` | Ledger tables (accounts, journal entries) |
| `000010` - `000011` | Outbox and audit log |
| `000012` | Processed events (idempotency) |

**Key questions to answer:**
- What foreign keys exist? What do they enforce?
- Why is there an outbox table? What problem does it solve?

### 3. Repository Layer (`shared/repository/`)

How the domain interacts with the database.

| File | Focus On |
|------|----------|
| `payment_intents.go` | CRUD operations, status updates |
| `ledger.go` | Double-entry bookkeeping operations |
| `outbox.go` | How events are written atomically with state changes |

**Key questions to answer:**
- How does a single transaction update payment_intents AND write to outbox?
- What makes the ledger "double-entry"?

### 4. Provider Adapters (`shared/adapter/`)

How external provider webhooks become internal canonical events.

| File | Focus On |
|------|----------|
| `adapter.go` | The interface all adapters implement |
| `stripe.go` | Signature verification, event parsing, decline mapping |
| `adyen.go` | Different provider, same canonical output |

**Key questions to answer:**
- Why normalize at the edge rather than throughout the system?
- How is webhook signature verification done? Why does it matter?

### 5. Temporal Workflows (`services/payment-worker/internal/workflow/`)

The orchestration layer that coordinates payment processing.

| File | Focus On |
|------|----------|
| `payment_intent_workflow.go` | The main workflow - states, signals, activities |
| `activities.go` | What can activities do that workflows cannot? |
| `state.go` | How workflow state is managed |
| `errors.go` | How errors control retry behavior |

**Key questions to answer:**
- What makes a workflow "deterministic"? What can't you do in a workflow?
- How do webhooks (async events) get into the workflow? (signals)
- What happens if the worker crashes mid-workflow?

### 6. API Layer (`services/payment-api/`)

The HTTP interface to the system.

| Path | Focus On |
|------|----------|
| `internal/handlers/intents.go` | Payment intent CRUD |
| `internal/handlers/webhooks.go` | Webhook ingestion |
| `internal/middleware/` | Auth, rate limiting, idempotency |

**Key questions to answer:**
- How does the idempotency middleware work?
- What happens when a webhook arrives?

---

## Interview-Style Questions

### Payment Domain Concepts

**Q: What's the difference between authorization and capture?**

Think about: holds on funds, two-phase commit analogy, when merchants might delay capture.

**Q: Why would you separate PaymentIntent from PaymentMethod?**

Think about: reusability, lifecycle differences, a customer might have multiple payment methods.

**Q: What's a "soft decline" vs a "hard decline"? How should the system handle each?**

Think about: retry eligibility, insufficient funds (temporary) vs invalid card (permanent).

**Q: Why track each payment attempt as a separate immutable record?**

Think about: audit trail, analytics, understanding failure patterns, no state loops.

### System Design

**Q: Why use Temporal instead of a simple database state machine?**

Think about: crash recovery, automatic retries, long-running processes, visibility into in-flight work.

**Q: Explain the transactional outbox pattern. What problem does it solve?**

Think about: dual-write problem, atomicity, what happens if you update DB then message queue fails?

**Q: Why normalize provider webhooks at the system boundary?**

Think about: provider-agnostic core logic, adding new providers, testing, single source of truth for event handling.

**Q: How does double-entry bookkeeping help with financial accuracy?**

Think about: every debit has a credit, balance verification, catching bugs mathematically.

### Failure Scenarios

**Q: A payment is authorized but the worker crashes before capture. What happens?**

Think about: Temporal's durability, workflow history, replay.

**Q: A webhook arrives twice with the same event. How do you prevent duplicate processing?**

Think about: processed_events table, idempotency keys, checking before processing.

**Q: The payment provider is down. How does the system behave?**

Think about: activity retries, backoff, timeout configuration, eventual failure handling.

**Q: An authorization expires before capture. What should happen?**

Think about: authorization hold lifecycle, void vs expired states, re-authorization.

### Code-Level Questions

**Q: Walk me through what happens when a POST to /intents/:id/authorize is received.**

Trace: handler -> validation -> workflow signal/start -> activities -> provider call -> state update.

**Q: How does the Stripe adapter verify a webhook is authentic?**

Look at: `shared/adapter/stripe.go`, HMAC signature verification.

**Q: Where is the retry policy for failed authorizations configured?**

Look at: workflow activity options, Temporal retry policies.

**Q: How does the system ensure a ledger entry is never created without updating the payment intent?**

Look at: database transactions, repository layer.

### Design Tradeoffs

**Q: You chose X. What are the downsides? When might you choose differently?**

Be ready to discuss tradeoffs for:
- Temporal vs simpler state machine
- PostgreSQL vs event sourcing
- Canonical events vs provider-specific handling
- Synchronous capture vs async capture

**Q: What would need to change to support 10x the transaction volume?**

Think about: database scaling, connection pooling, Temporal worker scaling, read replicas.

**Q: What's missing from this system that a production payment processor would need?**

Think about: PCI compliance, real provider integration, dispute handling, payouts, multi-currency, reporting.

---

## Suggested Learning Exercises

1. **Draw the state machines** - PaymentIntent, AuthorizationHold, PaymentAttempt. Do it from memory after reading the code.

2. **Trace a payment end-to-end** - From API call to ledger entry. Write down every function/table touched.

3. **Break it intentionally** - What happens if you kill the worker mid-payment? What if the database is down?

4. **Explain it to someone** - Teaching forces understanding. Explain the outbox pattern or Temporal workflows out loud.

5. **Read the decisions** - `docs/decisions/finalized-decisions.md` documents why certain choices were made.

---

## Key Files Quick Reference

| Concept | Primary File |
|---------|--------------|
| Payment states | `shared/domain/payment_intent.go` |
| Decline handling | `shared/domain/decline_code.go` |
| Webhook normalization | `shared/adapter/stripe.go` |
| Main workflow | `services/payment-worker/internal/workflow/payment_intent_workflow.go` |
| Outbox pattern | `shared/repository/outbox.go` |
| Ledger operations | `shared/repository/ledger.go` |
| API handlers | `services/payment-api/internal/handlers/` |
| Idempotency middleware | `services/payment-api/internal/middleware/idempotency.go` |
