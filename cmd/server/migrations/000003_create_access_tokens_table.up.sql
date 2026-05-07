CREATE TABLE IF NOT EXISTS access_tokens (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('PAT', 'SAT', 'SESSION')),
    binding_type TEXT NOT NULL CHECK (binding_type IN ('USER', 'CLUSTER', 'RUNNER')),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'REVOKED')),
    token_hash BYTEA NOT NULL, -- one-way hash; not reversible
    token_prefix TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    issuer TEXT,
    idp_access_token BYTEA, -- plaintext OIDC access token; sensitive, not encrypted at rest
    idp_refresh_token BYTEA, -- plaintext OIDC refresh token; long-lived re-auth, not encrypted at rest
    idp_id_token BYTEA, -- plaintext OIDC ID token; not encrypted at rest
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_access_tokens_token_hash ON access_tokens(token_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_access_tokens_subject_name ON access_tokens(subject, name) WHERE deleted_at IS NULL AND kind <> 'SESSION';
CREATE INDEX IF NOT EXISTS idx_access_tokens_subject_kind ON access_tokens(subject, kind);
CREATE INDEX IF NOT EXISTS idx_access_tokens_status ON access_tokens(status);