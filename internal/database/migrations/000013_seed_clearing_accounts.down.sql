-- Remove seeded clearing accounts
DELETE FROM accounts WHERE owner_id IS NULL AND name IN (
    'Authorization Clearing',
    'Settlement Clearing',
    'Fee Clearing',
    'Refund Clearing',
    'Merchant Payable',
    'Transaction Fee Revenue'
);
