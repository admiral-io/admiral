CREATE TABLE IF NOT EXISTS change_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_id TEXT NOT NULL UNIQUE CHECK (display_id ~ '^cs-[0-9a-z]{12}$'),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'DEPLOYED', 'DISCARDED')),
    copied_from_id UUID REFERENCES change_sets(id) ON DELETE SET NULL,
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    run_id UUID REFERENCES runs(id) ON DELETE SET NULL,
    base_head_revisions JSONB NOT NULL DEFAULT '{}',
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_change_sets_app_env ON change_sets(application_id, environment_id);
CREATE INDEX IF NOT EXISTS idx_change_sets_status ON change_sets(status);
CREATE INDEX IF NOT EXISTS idx_change_sets_created_at ON change_sets(created_at DESC);

CREATE TABLE IF NOT EXISTS change_set_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    change_set_id UUID NOT NULL REFERENCES change_sets(id) ON DELETE CASCADE,
    component_id UUID REFERENCES components(id) ON DELETE CASCADE,
    component_name TEXT NOT NULL CHECK (length(component_name) > 0 AND component_name ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$'),
    -- User intent. Distinct from revisions.change_type (computed effect) which adds
    -- RECREATE/IMPORT/NO_CHANGE. ORPHAN here produces no revision (handled by
    -- post-run reconciliation flipping components.desired_state).
    change_type TEXT NOT NULL CHECK (change_type IN ('CREATE', 'UPDATE', 'DESTROY', 'ORPHAN')),
    module_id UUID,
    version TEXT,
    values_template TEXT,
    depends_on TEXT[],
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (change_set_id, component_name)
);

CREATE INDEX IF NOT EXISTS idx_change_set_entries_change_set_id ON change_set_entries(change_set_id);
CREATE INDEX IF NOT EXISTS idx_change_set_entries_component_id ON change_set_entries(component_id) WHERE component_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS change_set_variable_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    change_set_id UUID NOT NULL REFERENCES change_sets(id) ON DELETE CASCADE,
    key TEXT NOT NULL CHECK (length(key) > 0 AND key ~ '^[A-Za-z_][A-Za-z0-9_]{0,62}$'),
    value TEXT,
    type TEXT NOT NULL DEFAULT 'STRING' CHECK (type IN ('STRING', 'NUMBER', 'BOOLEAN', 'COMPLEX')),
    sensitive BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (change_set_id, key)
);

CREATE INDEX IF NOT EXISTS idx_change_set_variable_entries_change_set_id ON change_set_variable_entries(change_set_id);

ALTER TABLE runs
    ADD CONSTRAINT runs_change_set_id_fkey
    FOREIGN KEY (change_set_id) REFERENCES change_sets(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_runs_change_set_id ON runs(change_set_id) WHERE change_set_id IS NOT NULL;
