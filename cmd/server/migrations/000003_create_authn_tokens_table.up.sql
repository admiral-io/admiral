CREATE TYPE authn_token_kind AS ENUM ('external', 'user', 'agent');
CREATE TYPE authn_token_status AS ENUM ('active', 'revoked', 'rotating');

CREATE TABLE IF NOT EXISTS authn_tokens (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    parent_id TEXT REFERENCES authn_tokens(id) ON DELETE SET NULL,
    name TEXT,
    subject TEXT NOT NULL,
    issuer TEXT NOT NULL CHECK (issuer <> ''),
    kind authn_token_kind NOT NULL,
    status authn_token_status NOT NULL DEFAULT 'active',
    access_token BYTEA NOT NULL,
    refresh_token BYTEA,
    id_token BYTEA,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE CHECK (expires_at IS NULL OR expires_at > created_at),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_authn_tokens_issuer ON authn_tokens(issuer);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_kind ON authn_tokens(kind);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_status ON authn_tokens(status);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_parent_id ON authn_tokens(parent_id);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_expires_at ON authn_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_authn_tokens_subject_kind ON authn_tokens(subject, kind);
