-- Change payment_method_id from UUID to varchar to support external payment method IDs
-- (e.g., Stripe's pm_xxx format)

-- Drop foreign key constraint if exists
ALTER TABLE payment_intents DROP CONSTRAINT IF EXISTS payment_intents_payment_method_id_fkey;

-- Change column type from UUID to varchar(100)
ALTER TABLE payment_intents ALTER COLUMN payment_method_id TYPE varchar(100) USING payment_method_id::text;
