CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE RESTRICT,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN (
        'PENDING', 'QUEUED', 'RUNNING',
        'SUCCEEDED', 'PARTIALLY_FAILED', 'FAILED',
        'CANCELED'
    )),
    trigger_type TEXT NOT NULL CHECK (trigger_type IN ('MANUAL', 'CI', 'DESTROY')),
    triggered_by TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    destroy BOOLEAN NOT NULL DEFAULT FALSE,
    source_deployment_id UUID REFERENCES deployments(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_deployments_app_env ON deployments(application_id, environment_id);
CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);
CREATE INDEX IF NOT EXISTS idx_deployments_created_at ON deployments(created_at DESC);
