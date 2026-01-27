-- Remove added indexes
DROP INDEX IF EXISTS idx_authorization_holds_intent_active;
DROP INDEX IF EXISTS idx_payment_intents_provider_payment_id;

-- Remove added columns
ALTER TABLE payment_methods DROP COLUMN IF EXISTS card_brand;
ALTER TABLE payment_attempts DROP COLUMN IF EXISTS error_message;
ALTER TABLE authorization_holds DROP COLUMN IF EXISTS voided_at;
