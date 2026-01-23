-- Outbox table - implements transactional outbox pattern for reliable event publishing
-- Events are written here in the same transaction as business data changes
-- CDC (Debezium) reads this table and publishes to Kafka
CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(50) NOT NULL,    -- PaymentIntent, Account, etc.
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,        -- payment.created, payment.authorized, etc.
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for CDC and polling
CREATE INDEX idx_outbox_created_at ON outbox(created_at);
CREATE INDEX idx_outbox_aggregate ON outbox(aggregate_type, aggregate_id);

-- For CDC with Debezium, set REPLICA IDENTITY to capture full row data
ALTER TABLE outbox REPLICA IDENTITY FULL;

COMMENT ON TABLE outbox IS 'Transactional outbox for reliable event publishing via CDC';
COMMENT ON COLUMN outbox.aggregate_type IS 'Type of entity (PaymentIntent, Account)';
COMMENT ON COLUMN outbox.aggregate_id IS 'ID of the entity';
COMMENT ON COLUMN outbox.event_type IS 'Canonical event type (payment.authorized, etc.)';
COMMENT ON COLUMN outbox.payload IS 'Canonical event data as JSON';
