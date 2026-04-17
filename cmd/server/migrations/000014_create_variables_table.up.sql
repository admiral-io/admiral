CREATE TABLE IF NOT EXISTS variables (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key             VARCHAR(63) NOT NULL,
    value           TEXT NOT NULL DEFAULT '',
    sensitive       BOOLEAN NOT NULL DEFAULT FALSE,
    type            TEXT NOT NULL DEFAULT 'STRING' CHECK (type IN ('STRING', 'NUMBER', 'BOOLEAN', 'COMPLEX')),
    source          TEXT NOT NULL DEFAULT 'USER' CHECK (source IN ('USER', 'INFRASTRUCTURE')),
    description     VARCHAR(1024) NOT NULL DEFAULT '',
    application_id  UUID REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID REFERENCES environments(id) ON DELETE CASCADE,
    created_by      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_variable_scope CHECK (
        (environment_id IS NULL) OR (application_id IS NOT NULL)
    )
);

-- Scope-level unique indexes (PostgreSQL NULLs are not equal in UNIQUE).
CREATE UNIQUE INDEX uq_variable_global_key
    ON variables (key)
    WHERE application_id IS NULL AND environment_id IS NULL;

CREATE UNIQUE INDEX uq_variable_app_key
    ON variables (application_id, key)
    WHERE application_id IS NOT NULL AND environment_id IS NULL;

CREATE UNIQUE INDEX uq_variable_env_key
    ON variables (environment_id, key)
    WHERE environment_id IS NOT NULL;

CREATE INDEX idx_variables_application_id ON variables (application_id);
CREATE INDEX idx_variables_environment_id ON variables (environment_id);
