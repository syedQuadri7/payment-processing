-- Payment attempts table - tracks individual payment processing attempts
CREATE TABLE payment_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_intent_id UUID NOT NULL REFERENCES payment_intents(id),
    attempt_number INT NOT NULL,
    status VARCHAR(20) NOT NULL,         -- PENDING, PROCESSING, SUCCEEDED, FAILED
    provider VARCHAR(20) NOT NULL,
    provider_response_code VARCHAR(100),
    canonical_decline_code VARCHAR(50),
    decline_type VARCHAR(20),            -- SOFT, HARD, FRAUD, TEMPORARY
    processor_txn_id VARCHAR(100),
    idempotency_key VARCHAR(150) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE(payment_intent_id, attempt_number)
);

-- Indexes for common query patterns
CREATE INDEX idx_payment_attempts_payment_intent_id ON payment_attempts(payment_intent_id);
CREATE INDEX idx_payment_attempts_status ON payment_attempts(status);
CREATE INDEX idx_payment_attempts_created_at ON payment_attempts(created_at);
