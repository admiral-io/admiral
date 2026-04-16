CREATE TABLE IF NOT EXISTS access_tokens (
    id                TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name              TEXT NOT NULL DEFAULT '',
    subject           TEXT NOT NULL,
    kind              TEXT NOT NULL CHECK (kind IN ('pat', 'sat', 'session')),
    binding_type      TEXT NOT NULL CHECK (binding_type IN ('user', 'cluster', 'runner')),
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    token_hash        BYTEA NOT NULL,
    token_prefix      TEXT NOT NULL,
    scopes            TEXT[] NOT NULL DEFAULT '{}',
    issuer            TEXT,
    idp_access_token  BYTEA,
    idp_refresh_token BYTEA,
    idp_id_token      BYTEA,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMP WITH TIME ZONE,
    deleted_at        TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX idx_access_tokens_token_hash ON access_tokens(token_hash);
CREATE INDEX idx_access_tokens_subject_kind ON access_tokens(subject, kind);
CREATE INDEX idx_access_tokens_status ON access_tokens(status);
