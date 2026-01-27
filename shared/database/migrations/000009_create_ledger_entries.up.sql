-- Ledger entries table - individual debit and credit entries
-- This table is APPEND-ONLY: no updates or deletes allowed
-- Corrections are made by creating new offsetting entries
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    journal_entry_id UUID NOT NULL REFERENCES journal_entries(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    amount DECIMAL(19,4) NOT NULL,
    direction VARCHAR(10) NOT NULL,         -- DEBIT or CREDIT
    balance_after DECIMAL(19,4) NOT NULL,   -- Running balance after this entry
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_ledger_amount_positive CHECK (amount > 0),
    CONSTRAINT chk_ledger_direction CHECK (direction IN ('DEBIT', 'CREDIT'))
);

-- Indexes for common query patterns
CREATE INDEX idx_ledger_entries_journal_entry_id ON ledger_entries(journal_entry_id);
CREATE INDEX idx_ledger_entries_account_id ON ledger_entries(account_id);
CREATE INDEX idx_ledger_entries_created_at ON ledger_entries(created_at);
CREATE INDEX idx_ledger_entries_account_created ON ledger_entries(account_id, created_at);

COMMENT ON TABLE ledger_entries IS 'Append-only table of individual debit and credit entries';
COMMENT ON COLUMN ledger_entries.amount IS 'Entry amount (always positive)';
COMMENT ON COLUMN ledger_entries.direction IS 'DEBIT or CREDIT';
COMMENT ON COLUMN ledger_entries.balance_after IS 'Running balance after this entry for reconciliation';

-- Create a rule to prevent updates (append-only enforcement)
CREATE RULE ledger_entries_no_update AS ON UPDATE TO ledger_entries
    DO INSTEAD NOTHING;

-- Create a rule to prevent deletes (append-only enforcement)
CREATE RULE ledger_entries_no_delete AS ON DELETE TO ledger_entries
    DO INSTEAD NOTHING;
