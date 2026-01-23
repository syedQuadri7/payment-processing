-- Journal entries table - groups related ledger entries into logical transactions
-- Every journal entry must balance (total debits = total credits)
CREATE TABLE journal_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    description TEXT NOT NULL,
    reference_type VARCHAR(50),             -- PAYMENT_INTENT, REFUND, ADJUSTMENT, FEE, SETTLEMENT
    reference_id UUID,                      -- ID of originating entity
    posted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_journal_reference_type CHECK (
        reference_type IS NULL OR
        reference_type IN ('PAYMENT_INTENT', 'REFUND', 'ADJUSTMENT', 'FEE', 'SETTLEMENT')
    )
);

-- Indexes for common query patterns
CREATE INDEX idx_journal_entries_reference ON journal_entries(reference_type, reference_id) WHERE reference_id IS NOT NULL;
CREATE INDEX idx_journal_entries_posted_at ON journal_entries(posted_at);
CREATE INDEX idx_journal_entries_created_at ON journal_entries(created_at);

COMMENT ON TABLE journal_entries IS 'Groups related ledger entries into logical transactions that must balance';
COMMENT ON COLUMN journal_entries.reference_type IS 'Type of entity that originated this journal entry';
COMMENT ON COLUMN journal_entries.reference_id IS 'ID of the originating entity';
