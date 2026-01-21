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

## When Ledger Entries Are Created

Understanding when ledger entries are created is critical for correct accounting.

### Key Principle

**Authorization is a promise, not money movement.** Ledger entries track actual money movement, not promises.

| Event | Ledger Entry? | Balance Update? | Rationale |
|-------|---------------|-----------------|-----------|
| Authorization | No | Yes (pending) | Promise to pay, not actual payment |
| Capture | Yes | Yes | Actual money movement |
| Refund | Yes | Yes | Reversal of money movement |
| Void | No | Yes (pending) | Cancels a promise, no money moved |

### Learning Project Simplification

For this learning project, we track authorizations via `pending_balance` on accounts, but do not create ledger entries until capture. This keeps the ledger focused on actual money movement.

Production systems may choose to track authorization holds in the ledger for more detailed audit trails, but it adds complexity without changing the accounting fundamentals.

---

## Ledger Transaction Patterns

### Authorization (Place Hold)

When authorization succeeds:

**Balance updates only (no ledger entry):**
- Increase `pending_balance` on customer account
- Decrease `available_balance` on customer account

**Why no ledger entry?** Authorization is a promise from the card issuer that funds are available. No money has moved yet. The ledger tracks money movement, not promises.

```
Customer Account:
  ledger_balance:    $1000.00  (unchanged)
  pending_balance:   $100.00   (increased by auth amount)
  available_balance: $900.00   (decreased by auth amount)
```

### Capture (Claim Held Funds)

When capture succeeds, **create ledger entries** (actual money movement):

```
Journal Entry: "Capture payment PI-123"
  DEBIT   Customer Account     $100.00
  CREDIT  Settlement Clearing  $100.00
```

**Balance updates:**
- Decrease `pending_balance` on customer account
- Decrease `ledger_balance` on customer account
- Increase balance on settlement clearing account

### Settlement (Bank Transfer)

When settlement completes, **create ledger entries:**

```
Journal Entry: "Settlement batch SB-456"
  DEBIT   Settlement Clearing  $100.00
  CREDIT  Merchant Payable     $100.00
```

This moves funds from our clearing account to the merchant's payable account.

### Refund (Reverse Capture)

When refund succeeds, **create ledger entries** (reversal):

```
Journal Entry: "Refund for PI-123"
  DEBIT   Merchant Payable     $100.00
  CREDIT  Customer Account     $100.00
```

### Void (Release Hold)

When authorization is voided:

**Balance updates only (no ledger entry):**
- Decrease `pending_balance` on customer account
- Increase `available_balance` on customer account

**Why no ledger entry?** Voiding releases the hold but no money ever moved, so there's nothing to record in the ledger.

---

## Summary: Entry Timing

| Operation | Creates Ledger Entry | Updates Balances |
|-----------|---------------------|------------------|
| Authorization | No | `pending_balance` increased |
| Capture | Yes | `ledger_balance` decreased, `pending_balance` decreased |
| Void | No | `pending_balance` decreased |
| Refund | Yes | `ledger_balance` increased |
| Settlement | Yes | Clearing account decreased, merchant increased |

---

## Production Considerations

Production systems may differ in these ways:

| Aspect | Learning Approach | Production Option |
|--------|-------------------|-------------------|
| Auth ledger entries | None | Optional entries for audit trail |
| Pending balance tracking | Account field | Separate holds table |
| Multi-currency | Single currency | Separate entries per currency |
| Settlement timing | Simplified | Complex batch reconciliation |
