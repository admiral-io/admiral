CREATE TABLE IF NOT EXISTS sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(name) > 0 AND name ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    description TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL CHECK (type IN ('GIT', 'TERRAFORM', 'HELM', 'OCI', 'HTTP')),
    url TEXT NOT NULL CHECK (length(url) > 0),
    credential_id UUID REFERENCES credentials(id) ON DELETE RESTRICT,
    catalog BOOLEAN NOT NULL DEFAULT FALSE,
    source_config JSONB NOT NULL DEFAULT '{}',
    labels JSONB NOT NULL DEFAULT '{}',
    last_test_status TEXT CHECK (last_test_status IN ('SUCCESS', 'FAILURE')),
    last_test_error TEXT NOT NULL DEFAULT '',
    last_tested_at TIMESTAMP WITH TIME ZONE,
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_name ON sources(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sources_deleted_at ON sources(deleted_at);
CREATE INDEX IF NOT EXISTS idx_sources_type ON sources(type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sources_credential_id ON sources(credential_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sources_catalog ON sources(catalog) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sources_labels ON sources USING GIN (labels);
