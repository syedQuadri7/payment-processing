-- Add missing columns and indexes based on schema documentation

-- Add voided_at column to authorization_holds (per schema/core-tables.md)
ALTER TABLE authorization_holds ADD COLUMN voided_at TIMESTAMPTZ;

-- Add error_message column to payment_attempts (per schema/core-tables.md)
ALTER TABLE payment_attempts ADD COLUMN error_message TEXT;

-- Add card_brand column to payment_methods (per schema/core-tables.md)
ALTER TABLE payment_methods ADD COLUMN card_brand VARCHAR(20);

-- Add composite index for webhook correlation (provider + provider_payment_id)
CREATE INDEX idx_payment_intents_provider_payment_id ON payment_intents(provider, provider_payment_id)
    WHERE provider_payment_id IS NOT NULL;

-- Add index for finding active authorization holds by intent
CREATE INDEX idx_authorization_holds_intent_active ON authorization_holds(payment_intent_id)
    WHERE status = 'ACTIVE';
