CREATE TYPE auth_session_status AS ENUM ('active', 'revoked');

CREATE TABLE IF NOT EXISTS auth_sessions (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    subject         TEXT NOT NULL,
    issuer          TEXT NOT NULL CHECK (issuer <> ''),
    status          auth_session_status NOT NULL DEFAULT 'active',
    access_token    BYTEA,
    refresh_token   BYTEA,
    id_token        BYTEA,
    scopes          TEXT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMP WITH TIME ZONE,
    deleted_at      TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_issuer ON auth_sessions(issuer);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_status ON auth_sessions(status);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_subject ON auth_sessions(subject);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);
