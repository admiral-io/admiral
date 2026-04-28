CREATE TABLE IF NOT EXISTS modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (length(name) > 0 AND name ~ '^[a-z][a-z0-9-]*(/[a-z][a-z0-9-]*)*$'),
    description TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL CHECK (type IN ('TERRAFORM', 'HELM', 'KUSTOMIZE', 'MANIFEST')),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,
    ref TEXT NOT NULL DEFAULT '',
    root TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    labels JSONB NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_modules_name ON modules(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_modules_deleted_at ON modules(deleted_at);
CREATE INDEX IF NOT EXISTS idx_modules_type ON modules(type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_modules_source_id ON modules(source_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_modules_labels ON modules USING GIN (labels);
