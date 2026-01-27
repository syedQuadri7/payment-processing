-- Remove rules first
DROP RULE IF EXISTS audit_log_no_delete ON audit_log;
DROP RULE IF EXISTS audit_log_no_update ON audit_log;

DROP TABLE IF EXISTS audit_log;
