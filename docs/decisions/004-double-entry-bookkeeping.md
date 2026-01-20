# ADR-004: Double-Entry Bookkeeping

## Status

Accepted

## Problem

Payment systems handle money movement and must maintain accurate, auditable records. We need a system that can prove correctness, detect errors, and provide complete audit trails.

**The core challenge**: Traditional single-entry systems (one record per transaction) have no built-in consistency checks. Errors can go undetected, and tracing money flow is difficult.

| Single-Entry Limitation | Consequence |
|------------------------|-------------|
| No balance verification | Cannot prove books are correct |
| No self-auditing | Errors may go undetected |
| Unclear money flow | Difficult to trace transactions |
| Limited audit trail | Compliance challenges |

## Solutions Considered

### Solution A: Single-Entry Ledger

One record per transaction with running balance.

| Pros | Cons |
|------|------|
| Simple to implement | No built-in consistency check |
| Easy to understand | Cannot prove balance correctness |
| Fewer records | Difficult to trace complex flows |
| | Updates to balance are error-prone |

### Solution B: Double-Entry Bookkeeping

Every transaction creates balanced debit and credit entries.

| Pros | Cons |
|------|------|
| Self-auditing (debits = credits) | More records per transaction |
| Mathematical proof of correctness | Team needs accounting knowledge |
| Complete audit trail | Corrections require new entries |
| Industry standard for finance | More complex queries |
| Clearing accounts reveal issues | |

### Solution C: Event Sourcing

Store all state changes as immutable events. Derive current state by replaying events.

| Pros | Cons |
|------|------|
| Complete history | Complex to query current state |
| Can replay to any point | Requires event store infrastructure |
| Natural audit trail | Schema evolution challenges |
| | Not standard accounting practice |

## Chosen Solution

**Solution B: Double-Entry Bookkeeping**

Stripe describes this approach as providing "mathematical proof of correctness" for their 5 billion daily events.

### Core Principles

| Principle | Description |
|-----------|-------------|
| Balanced entries | Every transaction creates at least two entries: a debit and a credit |
| Conservation | Sum of all debits must equal sum of all credits (always) |
| Immutability | Ledger entries are never updated, only append corrections |
| Clearing accounts | Track in-flight transactions with accounts that should trend to zero |

### Account Types

| Type | Normal Balance | Examples |
|------|----------------|----------|
| ASSET | Debit | Customer accounts, clearing accounts |
| LIABILITY | Credit | Merchant payable, reserves |
| EQUITY | Credit | Capital, retained earnings |
| REVENUE | Credit | Transaction fees |
| EXPENSE | Debit | Processing costs |

### Balance Types

Each account tracks multiple balance types:

| Balance | Description | Calculation |
|---------|-------------|-------------|
| Ledger | Sum of settled entries | Immutable entry sum |
| Pending | Authorized but unsettled | Active holds |
| Available | Spendable funds | Ledger - pending - reserved |
| Reserved | Held for scheduled payments | Scheduled payment amounts |

### Transaction Patterns

**Authorization (hold placed)**

| Entry | Account | Effect |
|-------|---------|--------|
| Debit | Customer Available Balance | Decreases by auth amount |
| Credit | Authorization Clearing | Increases by auth amount |

**Capture (funds claimed)**

| Entry | Account | Effect |
|-------|---------|--------|
| Debit | Authorization Clearing | Decreases by capture amount |
| Credit | Settlement Clearing | Increases by capture amount |

**Settlement (funds transferred)**

| Entry | Account | Effect |
|-------|---------|--------|
| Debit | Settlement Clearing | Decreases by settlement amount |
| Credit | Merchant Payable | Increases by settlement amount |

**Void (hold released)**

| Entry | Account | Effect |
|-------|---------|--------|
| Debit | Authorization Clearing | Decreases by void amount |
| Credit | Customer Available Balance | Increases by void amount |

### Clearing Account Monitoring

Clearing accounts should trend toward zero:

| Account | Purpose | Expected State |
|---------|---------|----------------|
| Authorization Clearing | Holds between auth and capture | Near-zero |
| Settlement Clearing | Awaiting bank settlement | Varies by cycle |
| Fee Clearing | Collected fees awaiting disbursement | Near-zero |
| Refund Clearing | Refunds in progress | Near-zero |

**Monitoring Rules**

| Condition | Alert Level | Required Action |
|-----------|-------------|-----------------|
| Non-zero balance > 12 hours | Warning | Review pending transactions |
| Non-zero balance > 24 hours | Critical | Investigate stuck transactions |
| Balance growing continuously | Critical | Check for processing failures |

### Data Model Requirements

**Accounts Table**

| Requirement | Description |
|-------------|-------------|
| Balance precision | DECIMAL(19,4) for all monetary values |
| Multiple balances | Track ledger, pending, available, reserved separately |
| Optimistic locking | Version column for concurrent update safety |
| Status tracking | ACTIVE, FROZEN, CLOSED states |

**Journal Entries Table**

| Requirement | Description |
|-------------|-------------|
| Grouping | Groups related ledger entries into logical transactions |
| Reference | Links to originating entity (payment intent, refund, etc.) |
| Balance constraint | Total debits must equal total credits |

**Ledger Entries Table**

| Requirement | Description |
|-------------|-------------|
| Immutability | Append-only, no updates or deletes |
| Direction | Each entry is either DEBIT or CREDIT |
| Running balance | Track balance_after for reconciliation |
| Amount constraint | Must be positive and non-zero |

## Why This Solution

| Reason | Explanation |
|--------|-------------|
| **Self-auditing** | If total debits don't equal total credits, there's a bug. Imbalances are immediately visible. |
| **Mathematical proof** | The constraint that books must balance provides provable correctness of all money movement. |
| **Complete audit trail** | Every money movement is recorded. Can trace any transaction from start to finish. |
| **Regulatory compliance** | Double-entry is the standard for financial record-keeping. Auditors understand it. |
| **Error detection** | Clearing accounts that don't trend to zero reveal stuck transactions or processing failures. |
| **Reconciliation** | Easy to verify internal records against external bank statements. |

### Trade-off Acceptance

| Trade-off | Mitigation |
|-----------|------------|
| More records | Batch balance updates; use materialized views for common queries |
| Accounting knowledge required | Train team on basic double-entry concepts |
| Correction complexity | Build helper functions for common correction patterns |
| More writes per transaction | Optimistic locking prevents lost updates; acceptable overhead |

### Concurrency Control

Balance updates require optimistic locking:

| Step | Action |
|------|--------|
| 1. Read | Fetch current balance and version |
| 2. Compute | Calculate new balance |
| 3. Update | Apply update with version check |
| 4. Retry | If version mismatch, re-read and retry |

## References

- [Stripe's approach to data quality](https://stripe.com/blog/ledger)
- [Double-entry bookkeeping](https://en.wikipedia.org/wiki/Double-entry_bookkeeping)
- [Square's Books ledger service](https://developer.squareup.com/blog/)
