CREATE TABLE IF NOT EXISTS revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    component_id UUID NOT NULL REFERENCES components(id) ON DELETE RESTRICT,
    component_name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('INFRASTRUCTURE', 'WORKLOAD')),
    status TEXT NOT NULL CHECK (status IN (
        'PENDING', 'QUEUED',
        'PLANNING', 'AWAITING_APPROVAL', 'APPLYING',
        'SUCCEEDED', 'FAILED', 'BLOCKED', 'CANCELED', 'SUPERSEDED'
    )),
    -- Computed effect (engine plan outcome). Distinct from change_set_entries.change_type
    -- which is user intent (CREATE/UPDATE/DESTROY/ORPHAN). One UPDATE entry may produce a
    -- revision with UPDATE, RECREATE, or NO_CHANGE here; ORPHAN entries produce no revision.
    change_type TEXT NOT NULL DEFAULT 'CREATE' CHECK (change_type IN (
        'CREATE', 'UPDATE', 'DESTROY', 'RECREATE', 'IMPORT', 'NO_CHANGE'
    )),
    previous_revision_id UUID REFERENCES revisions(id),
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE RESTRICT,
    source_id UUID REFERENCES sources(id) ON DELETE RESTRICT,
    version TEXT NOT NULL DEFAULT '',
    values_template TEXT NOT NULL DEFAULT '',
    resolved_values TEXT NOT NULL DEFAULT '',
    depends_on TEXT[] NOT NULL DEFAULT '{}',
    blocked_by TEXT[] NOT NULL DEFAULT '{}',
    working_directory TEXT NOT NULL DEFAULT '',
    artifact_checksum TEXT NOT NULL DEFAULT '',
    artifact_url TEXT NOT NULL DEFAULT '',
    available_phases TEXT[] NOT NULL DEFAULT '{}' CHECK (available_phases <@ ARRAY['PLAN', 'APPLY']),
    plan_file_key TEXT NOT NULL DEFAULT '',
    plan_summary JSONB,
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_revisions_run_id ON revisions(run_id);
CREATE INDEX IF NOT EXISTS idx_revisions_component_id ON revisions(component_id);
CREATE INDEX IF NOT EXISTS idx_revisions_status ON revisions(status);
CREATE INDEX IF NOT EXISTS idx_revisions_previous_revision_id ON revisions(previous_revision_id) WHERE previous_revision_id IS NOT NULL;
