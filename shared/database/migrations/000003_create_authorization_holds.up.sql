-- Authorization holds table - tracks card authorization holds
CREATE TABLE authorization_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_intent_id UUID NOT NULL REFERENCES payment_intents(id),
    amount DECIMAL(19,4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL,         -- ACTIVE, CAPTURED, VOIDED, EXPIRED
    provider VARCHAR(20) NOT NULL,
    auth_code VARCHAR(100),
    network_txn_id VARCHAR(100),
    expires_at TIMESTAMPTZ NOT NULL,
    captured_amount DECIMAL(19,4) DEFAULT 0,
    captured_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_authorization_holds_payment_intent_id ON authorization_holds(payment_intent_id);
CREATE INDEX idx_authorization_holds_status ON authorization_holds(status);
CREATE INDEX idx_authorization_holds_expires_at ON authorization_holds(expires_at) WHERE status = 'ACTIVE';
