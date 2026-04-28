CREATE TABLE IF NOT EXISTS components (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
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

CREATE UNIQUE INDEX IF NOT EXISTS idx_components_app_name ON components(application_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_components_app_slug ON components(application_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_application_id ON components(application_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_module_id ON components(module_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_kind ON components(kind) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_components_deleted_at ON components(deleted_at);

CREATE TABLE IF NOT EXISTS component_overrides (
    component_id UUID NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    disabled BOOLEAN NOT NULL DEFAULT FALSE,
    module_id UUID REFERENCES modules(id) ON DELETE RESTRICT,
    version TEXT,
    values_template TEXT,
    depends_on TEXT[],
    outputs JSONB,
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (component_id, environment_id)
);

CREATE INDEX IF NOT EXISTS idx_component_overrides_environment_id ON component_overrides(environment_id);
CREATE INDEX IF NOT EXISTS idx_component_overrides_module_id ON component_overrides(module_id) WHERE module_id IS NOT NULL;
