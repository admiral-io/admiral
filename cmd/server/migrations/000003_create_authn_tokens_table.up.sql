CREATE TYPE authn_token_kind AS ENUM ('external', 'user', 'cluster');

CREATE TABLE IF NOT EXISTS authn_tokens (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    parent_id TEXT REFERENCES authn_tokens(id) ON DELETE SET NULL,
    subject TEXT NOT NULL,
    issuer TEXT NOT NULL CHECK (issuer <> ''),
    kind authn_token_kind NOT NULL,
    access_token BYTEA NOT NULL,
    refresh_token BYTEA,
    id_token BYTEA,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_authn_tokens_issuer ON authn_tokens(issuer);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_kind ON authn_tokens(kind);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_parent_id ON authn_tokens(parent_id);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_expires_at ON authn_tokens(expires_at);
