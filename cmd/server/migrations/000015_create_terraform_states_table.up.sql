CREATE TABLE IF NOT EXISTS terraform_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    component_id UUID NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    serial BIGINT NOT NULL,
    lineage TEXT NOT NULL DEFAULT '',
    storage_path TEXT NOT NULL,
    content_length BIGINT NOT NULL DEFAULT 0,
    content_md5 TEXT NOT NULL DEFAULT '',
    lock_id TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_terraform_states_comp_env_created
    ON terraform_states (component_id, environment_id, created_at DESC);

CREATE TABLE IF NOT EXISTS terraform_state_locks (
    component_id UUID NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    lock_id TEXT NOT NULL,
    operation TEXT NOT NULL DEFAULT '',
    who TEXT NOT NULL DEFAULT '',
    info TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (component_id, environment_id)
);