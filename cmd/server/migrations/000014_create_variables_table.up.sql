CREATE TABLE IF NOT EXISTS variables (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key             VARCHAR(63) NOT NULL,
    value           TEXT NOT NULL DEFAULT '',
    sensitive       BOOLEAN NOT NULL DEFAULT FALSE,
    type            TEXT NOT NULL DEFAULT 'STRING' CHECK (type IN ('STRING', 'NUMBER', 'BOOLEAN', 'COMPLEX')),
    source          TEXT NOT NULL DEFAULT 'USER' CHECK (source IN ('USER', 'INFRASTRUCTURE')),
    description     VARCHAR(1024) NOT NULL DEFAULT '',
    application_id  UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    created_by      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (environment_id, key)
);

CREATE INDEX idx_variables_application_id ON variables (application_id);
CREATE INDEX idx_variables_environment_id ON variables (environment_id);
