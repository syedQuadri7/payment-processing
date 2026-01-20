# ADR-004: Double-Entry Bookkeeping

## Status

Accepted

## Context

Payment systems handle money movement and must maintain accurate, auditable records. Traditional single-entry systems (one record per transaction) have limitations:

- No built-in consistency checks
- Difficult to trace money flow
- Hard to detect errors or fraud
- Cannot prove balance correctness

Financial institutions and payment processors like Stripe use double-entry bookkeeping which provides:

- Mathematical proof that books balance (total debits = total credits)
- Self-auditing: imbalances immediately reveal problems
- Clear audit trail of all money movement
- Support for complex multi-party transactions

Stripe describes this as providing **"mathematical proof of correctness"** for their 5 billion daily events.

## Decision

We will implement **double-entry bookkeeping** with clearing account monitoring.

### Core Principles

1. **Every transaction creates at least two entries**: a debit and a credit
2. **Sum of all debits must equal sum of all credits** (always)
3. **Ledger entries are immutable**: never update, only append corrections
4. **Clearing accounts track in-flight transactions**

### Account Types

Following standard accounting:

| Type | Normal Balance | Examples |
|------|----------------|----------|
| ASSET | Debit | Customer accounts, clearing accounts |
| LIABILITY | Credit | Merchant payable, reserves |
| EQUITY | Credit | Capital, retained earnings |
| REVENUE | Credit | Transaction fees |
| EXPENSE | Debit | Processing costs |

### Transaction Examples

**Authorization (hold placed):**
```
Debit:  Customer Available Balance     -$100
Credit: Authorization Clearing         +$100
```

**Capture (funds claimed):**
```
Debit:  Authorization Clearing         -$100
Credit: Settlement Clearing            +$100
```

**Settlement (funds transferred):**
```
Debit:  Settlement Clearing            -$100
Credit: Merchant Payable               +$100
```

### Schema Design

```sql
-- Accounts with multiple balance types
CREATE TABLE accounts (
    id                UUID PRIMARY KEY,
    type              VARCHAR(20) NOT NULL,  -- ASSET, LIABILITY, etc.
    owner_id          VARCHAR(50),
    ledger_balance    DECIMAL(19,4) NOT NULL DEFAULT 0,
    pending_balance   DECIMAL(19,4) NOT NULL DEFAULT 0,
    available_balance DECIMAL(19,4) NOT NULL DEFAULT 0,
    version           INTEGER NOT NULL DEFAULT 0,  -- Optimistic locking
    created_at        TIMESTAMPTZ NOT NULL
);

-- Journal entries (transaction groups)
CREATE TABLE journal_entries (
    id           UUID PRIMARY KEY,
    description  TEXT NOT NULL,
    reference_type VARCHAR(50),
    reference_id   UUID,
    created_at   TIMESTAMPTZ NOT NULL
);

-- Individual ledger entries (append-only)
CREATE TABLE ledger_entries (
    id               UUID PRIMARY KEY,
    journal_entry_id UUID REFERENCES journal_entries(id),
    account_id       UUID REFERENCES accounts(id),
    amount           DECIMAL(19,4) NOT NULL,
    direction        VARCHAR(10) NOT NULL,  -- DEBIT or CREDIT
    balance_after    DECIMAL(19,4) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL
);
```

### Clearing Account Monitoring

Clearing accounts should tend toward zero at steady state:

| Account | Purpose | Expected State |
|---------|---------|----------------|
| `authorization_clearing` | Holds between auth and capture | Near-zero |
| `settlement_clearing` | Awaiting bank settlement | Varies by cycle |
| `fee_clearing` | Collected fees awaiting disbursement | Near-zero |

**Alert Rule**: Non-zero clearing balances exceeding 24 hours indicate unresolved issues requiring investigation.

### Balance Types

| Balance | Description | Calculation |
|---------|-------------|-------------|
| Ledger | Sum of settled entries | Immutable entry sum |
| Pending | Authorized but unsettled | Active holds |
| Available | Spendable funds | Ledger - pending debits |

## Consequences

### Positive

- **Self-auditing**: Books that don't balance indicate bugs immediately
- **Complete audit trail**: Every money movement recorded
- **Regulatory compliance**: Standard accounting practices
- **Debugging**: Can trace exact flow of any transaction
- **Reconciliation**: Easy to verify against external statements

### Negative

- **More records**: Two entries per transaction minimum
- **Complexity**: Need to understand accounting concepts
- **Performance**: More writes per transaction
- **Correction complexity**: Errors require correcting entries, not updates

### Mitigations

- Batch balance updates with optimistic locking
- Use materialized views for frequently-queried balances
- Build helper functions for common transaction patterns
- Train team on basic double-entry concepts

## Implementation Notes

### Ensuring Balance

```go
func (r *LedgerRepo) CreateJournalEntry(ctx context.Context, entries []LedgerEntry) error {
    // Validate debits = credits
    var debits, credits int64
    for _, e := range entries {
        if e.Direction == DirectionDebit {
            debits += e.Amount
        } else {
            credits += e.Amount
        }
    }
    if debits != credits {
        return ErrUnbalancedEntry
    }

    // Create entries in transaction
    return r.db.Transaction(ctx, func(tx *sql.Tx) error {
        // ... insert entries
    })
}
```

### Optimistic Locking for Balances

```sql
UPDATE accounts
SET ledger_balance = ledger_balance + $1,
    version = version + 1,
    updated_at = NOW()
WHERE id = $2 AND version = $3;
```

If no rows updated, retry with fresh version.

## References

- [Stripe's approach to data quality](https://stripe.com/blog/ledger)
- [Double-entry bookkeeping](https://en.wikipedia.org/wiki/Double-entry_bookkeeping)
- [Square's Books ledger service](https://developer.squareup.com/blog/)
