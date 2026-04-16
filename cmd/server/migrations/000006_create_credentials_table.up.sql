CREATE TABLE IF NOT EXISTS credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(name) > 0 AND name ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    description TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL CHECK (type IN ('SSH_KEY', 'BASIC_AUTH', 'BEARER_TOKEN')),
    auth_config JSONB NOT NULL DEFAULT '{}',
    labels JSONB NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_credentials_name ON credentials(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_credentials_deleted_at ON credentials(deleted_at);
CREATE INDEX IF NOT EXISTS idx_credentials_type ON credentials(type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_credentials_labels ON credentials USING GIN (labels);
