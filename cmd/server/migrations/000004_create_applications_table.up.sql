CREATE TABLE IF NOT EXISTS applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(name) > 0 AND name ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    description TEXT NOT NULL DEFAULT '',
    labels JSONB NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_applications_name ON applications(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_applications_deleted_at ON applications(deleted_at);
CREATE INDEX IF NOT EXISTS idx_applications_labels ON applications USING GIN (labels);
