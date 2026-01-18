-- Seed decline code mappings for common provider codes

-- Stripe decline codes
INSERT INTO decline_code_mappings (provider, provider_code, canonical_code, decline_type, description, retry_eligible, suggested_action) VALUES
('STRIPE', 'insufficient_funds', 'INSUFFICIENT_FUNDS', 'SOFT', 'The card has insufficient funds to complete the purchase.', true, 'Retry after a delay or request alternate payment method'),
('STRIPE', 'card_declined', 'GENERIC_DECLINE', 'SOFT', 'The card was declined for an unknown reason.', true, 'Retry or request alternate payment method'),
('STRIPE', 'expired_card', 'EXPIRED_CARD', 'HARD', 'The card has expired.', false, 'Request updated card details'),
('STRIPE', 'incorrect_cvc', 'INVALID_CVC', 'HARD', 'The CVC number is incorrect.', false, 'Request customer to re-enter CVC'),
('STRIPE', 'processing_error', 'PROCESSOR_ERROR', 'TEMPORARY', 'An error occurred while processing the card.', true, 'Retry after a short delay'),
('STRIPE', 'fraudulent', 'FRAUD_SUSPECTED', 'FRAUD', 'The payment has been flagged as potentially fraudulent.', false, 'Do not retry, investigate'),
('STRIPE', 'stolen_card', 'STOLEN_CARD', 'FRAUD', 'The card has been reported stolen.', false, 'Do not retry, report to fraud team'),
('STRIPE', 'lost_card', 'LOST_CARD', 'FRAUD', 'The card has been reported lost.', false, 'Do not retry, report to fraud team'),
('STRIPE', 'do_not_honor', 'DO_NOT_HONOR', 'HARD', 'The card issuer declined the transaction.', false, 'Request alternate payment method'),
('STRIPE', 'invalid_account', 'INVALID_ACCOUNT', 'HARD', 'The card or account is invalid.', false, 'Request alternate payment method'),

-- Adyen decline codes
('ADYEN', '2', 'INSUFFICIENT_FUNDS', 'SOFT', 'Not enough balance', true, 'Retry after a delay or request alternate payment method'),
('ADYEN', '5', 'GENERIC_DECLINE', 'SOFT', 'Refused', true, 'Retry or request alternate payment method'),
('ADYEN', '6', 'EXPIRED_CARD', 'HARD', 'Expired Card', false, 'Request updated card details'),
('ADYEN', '8', 'FRAUD_SUSPECTED', 'FRAUD', 'Fraud', false, 'Do not retry, investigate'),
('ADYEN', '14', 'INVALID_CARD_NUMBER', 'HARD', 'Invalid Card Number', false, 'Request correct card details'),
('ADYEN', '20', 'PROCESSOR_ERROR', 'TEMPORARY', 'Acquirer Error', true, 'Retry after a short delay'),
('ADYEN', '24', 'INVALID_CVC', 'HARD', 'CVC Declined', false, 'Request customer to re-enter CVC'),
('ADYEN', '31', 'SUSPECTED_FRAUD', 'FRAUD', 'Issuer Suspected Fraud', false, 'Do not retry, investigate'),
('ADYEN', '43', 'STOLEN_CARD', 'FRAUD', 'Stolen Card', false, 'Do not retry, report to fraud team'),

-- PayPal decline codes
('PAYPAL', 'INSUFFICIENT_FUNDS', 'INSUFFICIENT_FUNDS', 'SOFT', 'Insufficient funds in PayPal account.', true, 'Retry after a delay or request alternate payment method'),
('PAYPAL', 'TRANSACTION_REFUSED', 'GENERIC_DECLINE', 'SOFT', 'PayPal refused the transaction.', true, 'Retry or request alternate payment method'),
('PAYPAL', 'CREDIT_CARD_CVV_CHECK_FAILED', 'INVALID_CVC', 'HARD', 'CVV check failed.', false, 'Request customer to re-enter CVV'),
('PAYPAL', 'EXPIRED_CREDIT_CARD', 'EXPIRED_CARD', 'HARD', 'The credit card has expired.', false, 'Request updated card details'),
('PAYPAL', 'INSTRUMENT_DECLINED', 'GENERIC_DECLINE', 'SOFT', 'The payment instrument was declined.', true, 'Retry or request alternate payment method'),
('PAYPAL', 'INTERNAL_SERVICE_ERROR', 'PROCESSOR_ERROR', 'TEMPORARY', 'Internal PayPal service error.', true, 'Retry after a short delay'),
('PAYPAL', 'PAYER_ACCOUNT_RESTRICTED', 'ACCOUNT_RESTRICTED', 'HARD', 'Payer PayPal account is restricted.', false, 'Request alternate payment method'),
('PAYPAL', 'PAYER_ACCOUNT_LOCKED_OR_CLOSED', 'ACCOUNT_CLOSED', 'HARD', 'Payer PayPal account is locked or closed.', false, 'Request alternate payment method');
