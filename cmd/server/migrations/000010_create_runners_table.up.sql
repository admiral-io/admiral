CREATE TABLE IF NOT EXISTS runners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(name) > 0 AND name ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    description TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('INFRASTRUCTURE', 'WORKFLOW')),
    labels JSONB NOT NULL DEFAULT '{}',
    last_heartbeat_at TIMESTAMP WITH TIME ZONE,
    last_status JSONB,
    last_instance_id UUID,
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_runners_name ON runners(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_runners_kind ON runners(kind) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_runners_deleted_at ON runners(deleted_at);
