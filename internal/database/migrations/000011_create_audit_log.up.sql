-- Audit log table - append-only audit trail of all significant changes
-- Required for regulatory compliance and debugging
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,       -- PAYMENT_INTENT, PAYMENT_METHOD, ACCOUNT, AUTHORIZATION_HOLD
    entity_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL,            -- CREATE, UPDATE, DELETE, STATUS_CHANGE
    actor_type VARCHAR(20) NOT NULL,        -- SYSTEM, USER, WEBHOOK, API, WORKFLOW
    actor_id VARCHAR(100),                  -- ID of the actor (user ID, API key ID, etc.)
    old_values JSONB,                       -- Previous field values
    new_values JSONB,                       -- New field values
    metadata JSONB,                         -- Additional context (correlation ID, etc.)
    ip_address INET,                        -- Client IP address
    user_agent TEXT,                        -- Client user agent
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_audit_entity_type CHECK (
        entity_type IN ('PAYMENT_INTENT', 'PAYMENT_METHOD', 'ACCOUNT', 'AUTHORIZATION_HOLD')
    ),
    CONSTRAINT chk_audit_action CHECK (
        action IN ('CREATE', 'UPDATE', 'DELETE', 'STATUS_CHANGE')
    ),
    CONSTRAINT chk_audit_actor_type CHECK (
        actor_type IN ('SYSTEM', 'USER', 'WEBHOOK', 'API', 'WORKFLOW')
    )
);

-- Indexes for common query patterns
CREATE INDEX idx_audit_log_entity ON audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_actor ON audit_log(actor_type, actor_id) WHERE actor_id IS NOT NULL;
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX idx_audit_log_action ON audit_log(action);

-- Partitioning index for time-based cleanup (PostgreSQL 11+)
-- Useful for partitioning by month/quarter for long-term retention
CREATE INDEX idx_audit_log_created_at_brin ON audit_log USING BRIN(created_at);

-- Prevent updates and deletes (append-only)
CREATE RULE audit_log_no_update AS ON UPDATE TO audit_log
    DO INSTEAD NOTHING;

CREATE RULE audit_log_no_delete AS ON DELETE TO audit_log
    DO INSTEAD NOTHING;

COMMENT ON TABLE audit_log IS 'Append-only audit trail of all significant changes';
COMMENT ON COLUMN audit_log.old_values IS 'Previous field values (for UPDATE and STATUS_CHANGE)';
COMMENT ON COLUMN audit_log.new_values IS 'New field values (for CREATE, UPDATE, STATUS_CHANGE)';
COMMENT ON COLUMN audit_log.metadata IS 'Additional context like correlation ID, provider, etc.';
