-- Payment intents table - main payment transaction record
CREATE TABLE payment_intents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(100) UNIQUE NOT NULL,
    customer_id VARCHAR(50) NOT NULL,
    amount DECIMAL(19,4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL,         -- CREATED, REQUIRES_METHOD, REQUIRES_AUTH, AUTHORIZED, CAPTURED, FAILED, CANCELLED
    capture_method VARCHAR(20) NOT NULL DEFAULT 'AUTOMATIC',
    provider VARCHAR(20) NOT NULL,
    provider_payment_id VARCHAR(100),
    payment_method_id UUID REFERENCES payment_methods(id),
    workflow_id VARCHAR(100) UNIQUE,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_payment_intents_customer_id ON payment_intents(customer_id);
CREATE INDEX idx_payment_intents_status ON payment_intents(status);
CREATE INDEX idx_payment_intents_provider ON payment_intents(provider);
CREATE INDEX idx_payment_intents_created_at ON payment_intents(created_at);
CREATE INDEX idx_payment_intents_workflow_id ON payment_intents(workflow_id) WHERE workflow_id IS NOT NULL;
