-- Payment methods table - stores customer payment instruments
CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,           -- CARD, ACH, WALLET
    provider VARCHAR(20) NOT NULL,       -- STRIPE, ADYEN, PAYPAL
    token VARCHAR(255) NOT NULL,
    last_four VARCHAR(4),
    expiry_month INT,
    expiry_year INT,
    is_default BOOLEAN DEFAULT false,
    status VARCHAR(20) DEFAULT 'ACTIVE', -- ACTIVE, EXPIRED, DELETED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common query patterns
CREATE INDEX idx_payment_methods_customer_id ON payment_methods(customer_id);
CREATE INDEX idx_payment_methods_customer_default ON payment_methods(customer_id, is_default) WHERE is_default = true;
CREATE INDEX idx_payment_methods_status ON payment_methods(status);
