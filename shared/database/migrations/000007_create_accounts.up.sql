-- Accounts table - holds account balances for double-entry bookkeeping
-- Supports both customer accounts and internal system accounts (clearing, revenue, etc.)
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(20) NOT NULL,              -- ASSET, LIABILITY, EQUITY, REVENUE, EXPENSE, CLEARING
    owner_id VARCHAR(50),                   -- Customer ID (null for system accounts)
    name VARCHAR(100) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    ledger_balance DECIMAL(19,4) NOT NULL DEFAULT 0,
    pending_balance DECIMAL(19,4) NOT NULL DEFAULT 0,
    available_balance DECIMAL(19,4) NOT NULL DEFAULT 0,
    reserved_balance DECIMAL(19,4) NOT NULL DEFAULT 0,
    daily_limit DECIMAL(19,4),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE, FROZEN, CLOSED
    version INT NOT NULL DEFAULT 1,         -- Optimistic locking version
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_account_type CHECK (type IN ('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE', 'CLEARING')),
    CONSTRAINT chk_account_status CHECK (status IN ('ACTIVE', 'FROZEN', 'CLOSED'))
);

-- Indexes for common query patterns
CREATE INDEX idx_accounts_owner_id ON accounts(owner_id) WHERE owner_id IS NOT NULL;
CREATE INDEX idx_accounts_type ON accounts(type);
CREATE INDEX idx_accounts_status ON accounts(status);
CREATE INDEX idx_accounts_currency ON accounts(currency);

-- Unique constraint: one account per owner/type/currency combination
CREATE UNIQUE INDEX idx_accounts_owner_type_currency ON accounts(owner_id, type, currency) WHERE owner_id IS NOT NULL;

-- Unique constraint: system accounts by name and currency
CREATE UNIQUE INDEX idx_accounts_system_name_currency ON accounts(name, currency) WHERE owner_id IS NULL;

COMMENT ON TABLE accounts IS 'Holds account balances for double-entry bookkeeping';
COMMENT ON COLUMN accounts.ledger_balance IS 'Sum of all settled transactions';
COMMENT ON COLUMN accounts.pending_balance IS 'Authorized but unsettled amount';
COMMENT ON COLUMN accounts.available_balance IS 'Ledger minus pending debits';
COMMENT ON COLUMN accounts.reserved_balance IS 'Reserved for scheduled payments';
COMMENT ON COLUMN accounts.version IS 'Optimistic locking version for concurrent updates';
