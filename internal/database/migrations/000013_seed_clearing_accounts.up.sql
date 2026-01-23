-- Seed clearing accounts for tracking in-flight transactions
-- These are internal system accounts with no owner_id

-- Authorization Clearing - funds between auth and capture
INSERT INTO accounts (type, owner_id, name, currency, status)
VALUES ('CLEARING', NULL, 'Authorization Clearing', 'USD', 'ACTIVE');

-- Settlement Clearing - funds awaiting bank settlement
INSERT INTO accounts (type, owner_id, name, currency, status)
VALUES ('CLEARING', NULL, 'Settlement Clearing', 'USD', 'ACTIVE');

-- Fee Clearing - collected fees awaiting disbursement
INSERT INTO accounts (type, owner_id, name, currency, status)
VALUES ('CLEARING', NULL, 'Fee Clearing', 'USD', 'ACTIVE');

-- Refund Clearing - refunds in progress
INSERT INTO accounts (type, owner_id, name, currency, status)
VALUES ('CLEARING', NULL, 'Refund Clearing', 'USD', 'ACTIVE');

-- Merchant Payable - liability account for merchant funds
INSERT INTO accounts (type, owner_id, name, currency, status)
VALUES ('LIABILITY', NULL, 'Merchant Payable', 'USD', 'ACTIVE');

-- Transaction Fee Revenue - revenue account for transaction fees
INSERT INTO accounts (type, owner_id, name, currency, status)
VALUES ('REVENUE', NULL, 'Transaction Fee Revenue', 'USD', 'ACTIVE');
