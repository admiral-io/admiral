CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    runner_id UUID NOT NULL REFERENCES runners(id) ON DELETE RESTRICT,
    revision_id UUID NOT NULL REFERENCES revisions(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    job_type TEXT NOT NULL CHECK (job_type IN ('PLAN', 'APPLY', 'DESTROY_PLAN', 'DESTROY_APPLY')),
    status TEXT NOT NULL CHECK (status IN (
        'PENDING', 'ASSIGNED', 'RUNNING',
        'SUCCEEDED', 'FAILED', 'CANCELED'
    )),
    claimed_at TIMESTAMP WITH TIME ZONE,
    claimed_by_instance_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_jobs_runner_status ON jobs(runner_id, status);
CREATE INDEX IF NOT EXISTS idx_jobs_revision_id ON jobs(revision_id);
CREATE INDEX IF NOT EXISTS idx_jobs_run_id ON jobs(run_id);
