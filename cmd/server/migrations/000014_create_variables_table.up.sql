CREATE TABLE IF NOT EXISTS variables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT NOT NULL CHECK (
        length(key) > 0 AND (
            key ~ '^[A-Za-z_][A-Za-z0-9_]{0,62}$'
            OR
            key ~ '^[a-z]([a-z0-9-]{0,61}[a-z0-9])?\.[A-Za-z][A-Za-z0-9_-]*$'
        )
    ),
    value TEXT NOT NULL DEFAULT '',
    sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    type TEXT NOT NULL DEFAULT 'STRING' CHECK (type IN ('STRING', 'NUMBER', 'BOOLEAN', 'COMPLEX')),
    source TEXT NOT NULL DEFAULT 'USER' CHECK (source IN ('USER', 'INFRASTRUCTURE')),
    description VARCHAR(1024) NOT NULL DEFAULT '',
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    created_by TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    UNIQUE (environment_id, key)
);

CREATE INDEX IF NOT EXISTS idx_variables_application_id ON variables (application_id);
CREATE INDEX IF NOT EXISTS idx_variables_environment_id ON variables (environment_id);