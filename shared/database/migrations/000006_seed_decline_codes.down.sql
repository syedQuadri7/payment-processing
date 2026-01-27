-- Remove seeded decline code mappings
DELETE FROM decline_code_mappings WHERE provider IN ('STRIPE', 'ADYEN', 'PAYPAL');
