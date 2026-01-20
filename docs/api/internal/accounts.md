# Accounts API

Query account information, balances, and ledger entries.

## Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/accounts/:id` | Get account with balances |
| `GET` | `/api/v1/accounts/:id/ledger` | Get ledger entries |

---

## Get Account

Retrieves an account with its current balance states.

### Request

```
GET /api/v1/accounts/:id
```

### Example Request

```bash
curl http://localhost:8080/api/v1/accounts/acc_123
```

### Example Response

```json
{
  "id": "acc_123",
  "type": "checking",
  "owner_id": "cust_456",
  "currency": "USD",
  "status": "active",
  "balances": {
    "ledger": "10000.00",
    "pending": "500.00",
    "available": "9500.00",
    "reserved": "0.00"
  },
  "daily_limit": "5000.00",
  "created_at": "2025-06-01T00:00:00Z",
  "updated_at": "2026-01-17T10:30:00Z"
}
```

### Balance Types

| Balance | Description | Calculation |
|---------|-------------|-------------|
| `ledger` | Sum of all settled transactions | Immutable entry sum |
| `pending` | Authorized but unsettled amounts | Active holds sum |
| `available` | Funds available for new transactions | Ledger - pending debits |
| `reserved` | Reserved for scheduled payments | Scheduled payment sum |

---

## Get Ledger Entries

Retrieves ledger entries for an account with optional filtering.

### Request

```
GET /api/v1/accounts/:id/ledger
```

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `start_date` | string | Filter entries from this date (ISO 8601) |
| `end_date` | string | Filter entries until this date |
| `type` | string | Filter by entry type: `debit` or `credit` |
| `limit` | integer | Maximum entries to return (default: 50, max: 100) |
| `cursor` | string | Pagination cursor |

### Example Request

```bash
curl "http://localhost:8080/api/v1/accounts/acc_123/ledger?start_date=2026-01-01&limit=10"
```

### Example Response

```json
{
  "data": [
    {
      "id": "le_001",
      "journal_entry_id": "je_abc",
      "account_id": "acc_123",
      "amount": "-100.00",
      "direction": "debit",
      "balance_after": "9900.00",
      "description": "Payment capture for pi_xyz",
      "reference_type": "payment_intent",
      "reference_id": "pi_xyz",
      "created_at": "2026-01-17T10:40:00Z"
    },
    {
      "id": "le_002",
      "journal_entry_id": "je_def",
      "account_id": "acc_123",
      "amount": "500.00",
      "direction": "credit",
      "balance_after": "10000.00",
      "description": "Deposit",
      "reference_type": "deposit",
      "reference_id": "dep_123",
      "created_at": "2026-01-15T14:00:00Z"
    }
  ],
  "has_more": true,
  "next_cursor": "cursor_abc123"
}
```

### Entry Fields

| Field | Description |
|-------|-------------|
| `journal_entry_id` | Links to the journal entry (groups related debits/credits) |
| `direction` | `debit` (decrease) or `credit` (increase) |
| `balance_after` | Running balance after this entry |
| `reference_type` | Type of entity that caused this entry |
| `reference_id` | ID of the causing entity |

---

## Account Types

| Type | Description |
|------|-------------|
| `checking` | Standard checking account |
| `savings` | Savings account |
| `loan` | Loan account |
| `clearing` | Internal clearing account |

---

## Clearing Accounts

Clearing accounts track in-flight transactions. Non-zero balances exceeding 24 hours indicate unresolved issues.

| Account | Purpose | Expected Balance |
|---------|---------|------------------|
| `payment_clearing` | Funds between authorization and capture | Near-zero |
| `settlement_clearing` | Funds awaiting bank settlement | Varies by cycle |
| `fee_clearing` | Collected fees awaiting disbursement | Near-zero |

### Example: Query Clearing Account

```bash
curl http://localhost:8080/api/v1/accounts/clearing_payment
```

```json
{
  "id": "clearing_payment",
  "type": "clearing",
  "currency": "USD",
  "status": "active",
  "balances": {
    "ledger": "1500.00",
    "pending": "0.00",
    "available": "1500.00",
    "reserved": "0.00"
  },
  "alert_threshold": "1000.00",
  "last_zero_at": "2026-01-17T00:00:00Z"
}
```

If `ledger` balance is non-zero for extended periods, investigation is required.
