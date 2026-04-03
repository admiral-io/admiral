CREATE TYPE access_token_kind AS ENUM ('pat', 'sat');
CREATE TYPE access_token_status AS ENUM ('active', 'revoked');

CREATE TABLE IF NOT EXISTS access_tokens (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name            TEXT NOT NULL DEFAULT '',
    subject         TEXT NOT NULL,
    kind            access_token_kind NOT NULL,
    status          access_token_status NOT NULL DEFAULT 'active',
    token_hash      BYTEA NOT NULL,
    token_prefix    TEXT NOT NULL,
    scopes          TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMP WITH TIME ZONE,
    deleted_at      TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX idx_access_tokens_token_hash ON access_tokens(token_hash);
CREATE INDEX idx_access_tokens_subject_kind ON access_tokens(subject, kind);
CREATE INDEX idx_access_tokens_status ON access_tokens(status);
