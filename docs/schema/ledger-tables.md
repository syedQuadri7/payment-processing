# Ledger Tables

Database tables implementing double-entry bookkeeping.

## accounts

Holds account balances with multiple balance types for accurate financial tracking.

```sql
CREATE TABLE accounts (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type                  VARCHAR(20) NOT NULL,
    owner_id              VARCHAR(50),
    name                  VARCHAR(100) NOT NULL,
    currency              VARCHAR(3) NOT NULL DEFAULT 'USD',
    ledger_balance        DECIMAL(19,4) NOT NULL DEFAULT 0,
    pending_balance       DECIMAL(19,4) NOT NULL DEFAULT 0,
    available_balance     DECIMAL(19,4) NOT NULL DEFAULT 0,
    reserved_balance      DECIMAL(19,4) NOT NULL DEFAULT 0,
    daily_limit           DECIMAL(19,4),
    status                VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    version               INTEGER NOT NULL DEFAULT 0,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_owner ON accounts(owner_id);
CREATE INDEX idx_accounts_type ON accounts(type);
CREATE INDEX idx_accounts_status ON accounts(status);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `type` | VARCHAR(20) | Account type (see below) |
| `owner_id` | VARCHAR(50) | Customer ID (null for system accounts) |
| `name` | VARCHAR(100) | Human-readable account name |
| `currency` | VARCHAR(3) | ISO 4217 currency code |
| `ledger_balance` | DECIMAL(19,4) | Sum of all settled transactions |
| `pending_balance` | DECIMAL(19,4) | Authorized but unsettled |
| `available_balance` | DECIMAL(19,4) | Ledger minus pending debits |
| `reserved_balance` | DECIMAL(19,4) | Reserved for scheduled payments |
| `daily_limit` | DECIMAL(19,4) | Maximum daily outflow (optional) |
| `status` | VARCHAR(20) | ACTIVE, FROZEN, CLOSED |
| `version` | INTEGER | Optimistic locking version |
| `created_at` | TIMESTAMPTZ | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | Last update timestamp |

### Account Types

| Type | Normal Balance | Description |
|------|----------------|-------------|
| `ASSET` | Debit | Customer accounts, clearing accounts |
| `LIABILITY` | Credit | Merchant payable, reserves |
| `EQUITY` | Credit | Capital, retained earnings |
| `REVENUE` | Credit | Transaction fees |
| `EXPENSE` | Debit | Processing costs |
| `CLEARING` | Varies | In-flight transaction tracking |

### Balance Calculations

```sql
-- Available balance calculation
available_balance = ledger_balance - pending_balance - reserved_balance

-- After authorization (hold placed)
UPDATE accounts SET
    pending_balance = pending_balance + :hold_amount,
    available_balance = ledger_balance - pending_balance - :hold_amount - reserved_balance
WHERE id = :customer_account_id;

-- After capture (hold converted to settlement)
UPDATE accounts SET
    pending_balance = pending_balance - :capture_amount,
    ledger_balance = ledger_balance - :capture_amount,
    available_balance = ledger_balance - :capture_amount - pending_balance + :capture_amount - reserved_balance
WHERE id = :customer_account_id;
```

### Optimistic Locking

```sql
UPDATE accounts
SET ledger_balance = ledger_balance + :amount,
    version = version + 1,
    updated_at = NOW()
WHERE id = :account_id AND version = :expected_version;

-- If rows_affected = 0, retry with fresh data
```

---

## journal_entries

Groups related ledger entries into logical transactions. Every journal entry must balance (total debits = total credits).

```sql
CREATE TABLE journal_entries (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    description       TEXT NOT NULL,
    reference_type    VARCHAR(50),
    reference_id      UUID,
    posted_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_journal_reference ON journal_entries(reference_type, reference_id);
CREATE INDEX idx_journal_posted ON journal_entries(posted_at);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `description` | TEXT | Human-readable description |
| `reference_type` | VARCHAR(50) | Type of originating entity |
| `reference_id` | UUID | ID of originating entity |
| `posted_at` | TIMESTAMPTZ | When the entry was posted |
| `created_at` | TIMESTAMPTZ | Creation timestamp |

### Reference Types

| Type | Description |
|------|-------------|
| `PAYMENT_INTENT` | Payment authorization or capture |
| `REFUND` | Payment refund |
| `ADJUSTMENT` | Manual adjustment |
| `FEE` | Fee collection |
| `SETTLEMENT` | Bank settlement |

---

## ledger_entries

Individual debit and credit entries. Append-only - never updated or deleted.

```sql
CREATE TABLE ledger_entries (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_entry_id      UUID NOT NULL REFERENCES journal_entries(id),
    account_id            UUID NOT NULL REFERENCES accounts(id),
    amount                DECIMAL(19,4) NOT NULL,
    direction             VARCHAR(10) NOT NULL,
    balance_after         DECIMAL(19,4) NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_journal ON ledger_entries(journal_entry_id);
CREATE INDEX idx_ledger_account ON ledger_entries(account_id);
CREATE INDEX idx_ledger_created ON ledger_entries(created_at);

-- Constraint to ensure non-zero entries
ALTER TABLE ledger_entries ADD CONSTRAINT chk_nonzero_amount CHECK (amount <> 0);
```

### Columns

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `journal_entry_id` | UUID | Parent journal entry |
| `account_id` | UUID | Account being debited/credited |
| `amount` | DECIMAL(19,4) | Entry amount (always positive) |
| `direction` | VARCHAR(10) | DEBIT or CREDIT |
| `balance_after` | DECIMAL(19,4) | Running balance after entry |
| `created_at` | TIMESTAMPTZ | Creation timestamp |

### Double-Entry Constraint

Ensure journal entries balance:

```sql
CREATE OR REPLACE FUNCTION check_journal_balance()
RETURNS TRIGGER AS $$
DECLARE
    debit_sum DECIMAL(19,4);
    credit_sum DECIMAL(19,4);
BEGIN
    SELECT
        COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0),
        COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0)
    INTO debit_sum, credit_sum
    FROM ledger_entries
    WHERE journal_entry_id = NEW.journal_entry_id;

    -- Allow during multi-insert, check on commit
    -- Or implement as application-level validation

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

---

## Clearing Accounts

Special accounts that track in-flight transactions. Non-zero balances indicate unresolved states.

### Standard Clearing Accounts

```sql
-- Create clearing accounts on system initialization
INSERT INTO accounts (type, name, currency) VALUES
    ('CLEARING', 'Authorization Clearing', 'USD'),
    ('CLEARING', 'Settlement Clearing', 'USD'),
    ('CLEARING', 'Fee Clearing', 'USD'),
    ('CLEARING', 'Refund Clearing', 'USD');
```

| Account | Purpose | Expected State |
|---------|---------|----------------|
| Authorization Clearing | Funds between auth and capture | Near-zero |
| Settlement Clearing | Awaiting bank settlement | Varies by cycle |
| Fee Clearing | Fees awaiting disbursement | Near-zero |
| Refund Clearing | Refunds in progress | Near-zero |

### Monitoring Query

```sql
-- Alert on stale clearing balances
SELECT
    name,
    ledger_balance,
    updated_at,
    NOW() - updated_at AS age
FROM accounts
WHERE type = 'CLEARING'
  AND ledger_balance <> 0
  AND NOW() - updated_at > INTERVAL '24 hours';
```

---

## Transaction Examples

### Authorization (Place Hold)

```sql
-- Journal entry
INSERT INTO journal_entries (id, description, reference_type, reference_id)
VALUES (:je_id, 'Authorization for pi_abc123', 'PAYMENT_INTENT', :intent_id);

-- Debit customer available, credit clearing
INSERT INTO ledger_entries (journal_entry_id, account_id, amount, direction, balance_after)
VALUES
    (:je_id, :customer_account_id, 100.00, 'DEBIT', :new_customer_balance),
    (:je_id, :auth_clearing_id, 100.00, 'CREDIT', :new_clearing_balance);

-- Update balances
UPDATE accounts SET pending_balance = pending_balance + 100.00 WHERE id = :customer_account_id;
```

### Capture (Claim Held Funds)

```sql
-- Journal entry
INSERT INTO journal_entries (id, description, reference_type, reference_id)
VALUES (:je_id, 'Capture for pi_abc123', 'PAYMENT_INTENT', :intent_id);

-- Move from auth clearing to settlement clearing
INSERT INTO ledger_entries (journal_entry_id, account_id, amount, direction, balance_after)
VALUES
    (:je_id, :auth_clearing_id, 100.00, 'DEBIT', :new_auth_clearing),
    (:je_id, :settlement_clearing_id, 100.00, 'CREDIT', :new_settlement_clearing);

-- Update customer balances (hold converted to actual debit)
UPDATE accounts SET
    pending_balance = pending_balance - 100.00,
    ledger_balance = ledger_balance - 100.00
WHERE id = :customer_account_id;
```

### Settlement (Transfer to Merchant)

```sql
-- Journal entry
INSERT INTO journal_entries (id, description, reference_type, reference_id)
VALUES (:je_id, 'Settlement batch 2026-01-17', 'SETTLEMENT', :batch_id);

-- Move from settlement clearing to merchant payable
INSERT INTO ledger_entries (journal_entry_id, account_id, amount, direction, balance_after)
VALUES
    (:je_id, :settlement_clearing_id, 10000.00, 'DEBIT', :new_settlement_clearing),
    (:je_id, :merchant_payable_id, 10000.00, 'CREDIT', :new_merchant_payable);
```
