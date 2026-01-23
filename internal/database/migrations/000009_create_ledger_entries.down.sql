-- Remove rules first
DROP RULE IF EXISTS ledger_entries_no_delete ON ledger_entries;
DROP RULE IF EXISTS ledger_entries_no_update ON ledger_entries;

DROP TABLE IF EXISTS ledger_entries;
