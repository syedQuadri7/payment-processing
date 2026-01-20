# Ledger Tables

Database tables implementing double-entry bookkeeping for accurate financial tracking.

## accounts

Holds account balances with multiple balance types. Supports both customer accounts and internal system accounts.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `type` | String(20) | Yes | Account type |
| `owner_id` | String(50) | No | Customer ID (null for system accounts) |
| `name` | String(100) | Yes | Human-readable account name |
| `currency` | String(3) | Yes | ISO 4217 currency code |
| `ledger_balance` | Decimal(19,4) | Yes | Sum of all settled transactions |
| `pending_balance` | Decimal(19,4) | Yes | Authorized but unsettled |
| `available_balance` | Decimal(19,4) | Yes | Ledger minus pending debits |
| `reserved_balance` | Decimal(19,4) | Yes | Reserved for scheduled payments |
| `daily_limit` | Decimal(19,4) | No | Maximum daily outflow |
| `status` | String(20) | Yes | ACTIVE, FROZEN, CLOSED |
| `version` | Integer | Yes | Optimistic locking version |
| `created_at` | Timestamp | Yes | Creation timestamp |
| `updated_at` | Timestamp | Yes | Last update timestamp |

### Account Types

| Type | Normal Balance | Purpose |
|------|----------------|---------|
| `ASSET` | Debit | Customer accounts, clearing accounts |
| `LIABILITY` | Credit | Merchant payable, reserves |
| `EQUITY` | Credit | Capital, retained earnings |
| `REVENUE` | Credit | Transaction fees |
| `EXPENSE` | Debit | Processing costs |
| `CLEARING` | Varies | In-flight transaction tracking |

### Balance Calculation

```
available_balance = ledger_balance - pending_balance - reserved_balance
```

### Concurrency Control

The `version` field must be used for optimistic locking:
1. Read current version with balance
2. Compute new balance
3. Update with WHERE version = expected_version
4. If no rows affected, retry with fresh data

---

## journal_entries

Groups related ledger entries into logical transactions. Every journal entry must balance (total debits = total credits).

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `description` | Text | Yes | Human-readable description |
| `reference_type` | String(50) | No | Type of originating entity |
| `reference_id` | UUID | No | ID of originating entity |
| `posted_at` | Timestamp | Yes | When the entry was posted |
| `created_at` | Timestamp | Yes | Creation timestamp |

### Reference Types

| Type | Description |
|------|-------------|
| `PAYMENT_INTENT` | Payment authorization or capture |
| `REFUND` | Payment refund |
| `ADJUSTMENT` | Manual adjustment |
| `FEE` | Fee collection |
| `SETTLEMENT` | Bank settlement |

### Balance Constraint

Every journal entry must have total debits equal to total credits across its ledger entries. This can be enforced via:
- Application-level validation before insert
- Database trigger after commit
- Batch reconciliation job

---

## ledger_entries

Individual debit and credit entries. Append-only table that is never updated or deleted.

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `journal_entry_id` | UUID | Yes | Reference to parent journal entry |
| `account_id` | UUID | Yes | Reference to account being debited/credited |
| `amount` | Decimal(19,4) | Yes | Entry amount (always positive, non-zero) |
| `direction` | String(10) | Yes | DEBIT or CREDIT |
| `balance_after` | Decimal(19,4) | Yes | Running balance after this entry |
| `created_at` | Timestamp | Yes | Creation timestamp |

### Constraints

- Amount must be non-zero
- Direction must be DEBIT or CREDIT
- Journal entry must exist
- Account must exist

### Immutability

This table is append-only:
- No UPDATE operations allowed
- No DELETE operations allowed
- Corrections are made by creating new offsetting entries
- Balance_after provides running total for reconciliation

---

## Clearing Accounts

Special internal accounts that track in-flight transactions. Non-zero balances exceeding defined thresholds indicate issues requiring investigation.

### Required Clearing Accounts

| Account Name | Purpose | Expected Steady State |
|--------------|---------|----------------------|
| Authorization Clearing | Funds between auth and capture | Near-zero |
| Settlement Clearing | Awaiting bank settlement | Varies by settlement cycle |
| Fee Clearing | Fees awaiting disbursement | Near-zero |
| Refund Clearing | Refunds in progress | Near-zero |

### Monitoring Requirements

| Condition | Alert Level | Action Required |
|-----------|-------------|-----------------|
| Non-zero balance > 12 hours | Warning | Review pending transactions |
| Non-zero balance > 24 hours | Critical | Investigate stuck transactions |
| Balance growing continuously | Critical | Check for processing failures |

---

## Ledger Transaction Patterns

### Authorization (Place Hold)

When authorization succeeds, create journal entry with:
- DEBIT to customer account (reduces available)
- CREDIT to authorization clearing

Also update customer account:
- Increase pending_balance
- Decrease available_balance

### Capture (Claim Held Funds)

When capture succeeds, create journal entry with:
- DEBIT from authorization clearing
- CREDIT to settlement clearing

Also update customer account:
- Decrease pending_balance
- Decrease ledger_balance

### Settlement (Bank Transfer)

When settlement completes, create journal entry with:
- DEBIT from settlement clearing
- CREDIT to merchant payable

### Void (Release Hold)

When authorization is voided, create journal entry with:
- DEBIT from authorization clearing
- CREDIT to customer account (restores available)

Also update customer account:
- Decrease pending_balance
- Increase available_balance
