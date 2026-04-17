CREATE TABLE IF NOT EXISTS revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    component_id UUID NOT NULL REFERENCES components(id) ON DELETE RESTRICT,
    component_name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('INFRASTRUCTURE', 'WORKLOAD')),
    status TEXT NOT NULL CHECK (status IN (
        'PENDING', 'QUEUED',
        'PLANNING', 'AWAITING_APPROVAL', 'APPLYING',
        'SUCCEEDED', 'FAILED', 'BLOCKED', 'CANCELLED'
    )),
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE RESTRICT,
    source_id UUID,
    version TEXT NOT NULL DEFAULT '',
    resolved_values TEXT NOT NULL DEFAULT '',
    depends_on TEXT[] NOT NULL DEFAULT '{}',
    blocked_by TEXT[] NOT NULL DEFAULT '{}',
    working_directory TEXT NOT NULL DEFAULT '',
    artifact_checksum TEXT NOT NULL DEFAULT '',
    artifact_url TEXT NOT NULL DEFAULT '',
    plan_output_key TEXT NOT NULL DEFAULT '',
    plan_file_key TEXT NOT NULL DEFAULT '',
    plan_summary JSONB,
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_revisions_deployment_id ON revisions(deployment_id);
CREATE INDEX IF NOT EXISTS idx_revisions_component_id ON revisions(component_id);
CREATE INDEX IF NOT EXISTS idx_revisions_status ON revisions(status);
