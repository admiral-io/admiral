CREATE TABLE IF NOT EXISTS runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE RESTRICT,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN (
        'PENDING', 'QUEUED', 'PLANNING', 'PLANNED', 'APPLYING',
        'SUCCEEDED', 'PARTIALLY_FAILED', 'FAILED',
        'CANCELED', 'SUPERSEDED'
    )),
    triggered_by TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    destroy BOOLEAN NOT NULL DEFAULT FALSE,
    source_run_id UUID REFERENCES runs(id),
    change_set_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_runs_app_env ON runs(application_id, environment_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at DESC);
