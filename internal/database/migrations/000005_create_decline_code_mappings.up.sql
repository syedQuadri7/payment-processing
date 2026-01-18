-- Decline code mappings table - maps provider-specific codes to canonical codes
CREATE TABLE decline_code_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(20) NOT NULL,
    provider_code VARCHAR(100) NOT NULL,
    canonical_code VARCHAR(50) NOT NULL,
    decline_type VARCHAR(20) NOT NULL,   -- SOFT, HARD, FRAUD, TEMPORARY
    description TEXT,
    retry_eligible BOOLEAN NOT NULL DEFAULT false,
    suggested_action TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_code)
);

-- Indexes for common query patterns
CREATE INDEX idx_decline_code_mappings_provider ON decline_code_mappings(provider);
CREATE INDEX idx_decline_code_mappings_canonical_code ON decline_code_mappings(canonical_code);
