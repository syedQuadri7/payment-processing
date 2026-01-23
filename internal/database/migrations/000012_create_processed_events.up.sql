-- Processed events table - tracks processed webhook events for idempotency
-- Prevents duplicate processing when providers send the same webhook multiple times
CREATE TABLE processed_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(20) NOT NULL,          -- STRIPE, ADYEN, PAYPAL
    event_id VARCHAR(100) NOT NULL,         -- Provider's event ID
    event_type VARCHAR(50) NOT NULL,        -- Event type received
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_processed_events_provider CHECK (
        provider IN ('STRIPE', 'ADYEN', 'PAYPAL')
    ),
    CONSTRAINT uq_processed_events_provider_event UNIQUE (provider, event_id)
);

-- Indexes for common query patterns
CREATE INDEX idx_processed_events_processed_at ON processed_events(processed_at);

COMMENT ON TABLE processed_events IS 'Tracks processed webhook events for idempotency';
COMMENT ON COLUMN processed_events.event_id IS 'Provider unique event identifier';
