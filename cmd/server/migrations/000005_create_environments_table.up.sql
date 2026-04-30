CREATE TABLE IF NOT EXISTS environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id),
    name TEXT NOT NULL CHECK (length(name) > 0 AND name ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    description TEXT NOT NULL DEFAULT '',
    workload_targets JSONB NOT NULL DEFAULT '[]',
    infrastructure_targets JSONB NOT NULL DEFAULT '[]',
    labels JSONB NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_environments_app_name ON environments(application_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_environments_application_id ON environments(application_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_environments_deleted_at ON environments(deleted_at);
CREATE INDEX IF NOT EXISTS idx_environments_labels ON environments USING GIN (labels);
