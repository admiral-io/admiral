CREATE TABLE IF NOT EXISTS components (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(name) > 0 AND name ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    slug TEXT NOT NULL CHECK (length(slug) > 0 AND slug ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    description TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('INFRASTRUCTURE', 'WORKLOAD')),
    desired_state TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (desired_state IN ('ACTIVE', 'DESTROY', 'ORPHAN', 'DESTROYED')),
    deletion_protection BOOLEAN NOT NULL DEFAULT false,
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE RESTRICT,
    version TEXT NOT NULL DEFAULT '',
    values_template TEXT NOT NULL DEFAULT '',
    depends_on TEXT[] NOT NULL DEFAULT '{}',
    outputs JSONB NOT NULL DEFAULT '[]',
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_components_app_env_name ON components(application_id, environment_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_components_app_env_slug ON components(application_id, environment_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_app_env ON components(application_id, environment_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_module_id ON components(module_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_kind ON components(kind) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_deleted_at ON components(deleted_at);
