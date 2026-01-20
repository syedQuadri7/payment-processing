# Accounts API

Query account information, balances, and ledger entries. This API provides read-only access to financial data.

## Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/v1/accounts/:id` | Get account with current balances |
| GET | `/api/v1/accounts/:id/ledger` | Get ledger entries for account |

---

## Get Account

Retrieves an account with its current balance states.

### Response Fields

| Field | Description |
|-------|-------------|
| `id` | Unique account identifier |
| `type` | Account type (checking, savings, loan, clearing) |
| `owner_id` | Customer who owns the account |
| `currency` | Account currency (ISO 4217) |
| `status` | Account status (active, frozen, closed) |
| `balances` | Object containing all balance types |
| `daily_limit` | Maximum daily outflow (if configured) |

### Balance Types

| Balance | Description | Business Meaning |
|---------|-------------|------------------|
| `ledger` | Sum of all settled transactions | Immutable historical record |
| `pending` | Authorized but unsettled amounts | Money on hold |
| `available` | Funds available for new transactions | What customer can spend |
| `reserved` | Reserved for scheduled payments | Earmarked for future payments |

### Balance Calculation

```
available = ledger - pending - reserved
```

---

## Get Ledger Entries

Retrieves the ledger entry history for an account with optional filtering.

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `start_date` | ISO 8601 date | Filter entries from this date |
| `end_date` | ISO 8601 date | Filter entries until this date |
| `type` | string | Filter by entry type: `debit` or `credit` |
| `limit` | integer | Maximum entries to return (default: 50, max: 100) |
| `cursor` | string | Pagination cursor for next page |

### Response Fields per Entry

| Field | Description |
|-------|-------------|
| `id` | Unique entry identifier |
| `journal_entry_id` | Groups related debits/credits together |
| `amount` | Entry amount (positive value) |
| `direction` | `debit` (decrease) or `credit` (increase) |
| `balance_after` | Running balance after this entry |
| `description` | Human-readable description |
| `reference_type` | Type of entity that caused this entry |
| `reference_id` | ID of the causing entity |
| `created_at` | Entry timestamp |

### Reference Types

| Type | Description |
|------|-------------|
| `payment_intent` | Payment authorization or capture |
| `refund` | Payment refund |
| `adjustment` | Manual adjustment |
| `deposit` | Deposit into account |
| `fee` | Fee collection |

---

## Account Types

| Type | Purpose |
|------|---------|
| `checking` | Standard customer checking account |
| `savings` | Customer savings account |
| `loan` | Loan account |
| `clearing` | Internal system clearing account |

---

## Clearing Accounts

Clearing accounts are internal system accounts that track in-flight transactions. They are critical for operational monitoring.

### Standard Clearing Accounts

| Account | Purpose | Expected Steady State |
|---------|---------|----------------------|
| `payment_clearing` | Funds between authorization and capture | Near-zero |
| `settlement_clearing` | Funds awaiting bank settlement | Varies by settlement cycle |
| `fee_clearing` | Collected fees awaiting disbursement | Near-zero |

### Monitoring Rules

- Non-zero clearing balances exceeding 24 hours indicate unresolved issues
- Each clearing account should have an alert threshold configured
- The `last_zero_at` field tracks when the account was last at zero balance

### Investigation Triggers

| Condition | Indicates |
|-----------|-----------|
| `payment_clearing` non-zero > 24h | Stuck authorizations not captured or voided |
| `settlement_clearing` growing | Settlement batch delays |
| `fee_clearing` non-zero > 24h | Fee disbursement issues |
