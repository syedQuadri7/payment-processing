-- Revert payment_method_id to UUID type
-- Note: This may fail if data contains non-UUID values

ALTER TABLE payment_intents ALTER COLUMN payment_method_id TYPE uuid USING payment_method_id::uuid;

-- Re-add foreign key constraint
ALTER TABLE payment_intents
  ADD CONSTRAINT payment_intents_payment_method_id_fkey
  FOREIGN KEY (payment_method_id) REFERENCES payment_methods(id);
