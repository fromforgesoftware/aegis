-- Browser session (the hosted-login cookie points at a row here) and the
-- short-lived, single-use authorization code for the OAuth code grant.

CREATE TABLE aegis.session (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    realm_id   UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);
CREATE INDEX idx_session_account ON aegis.session (account_id);

CREATE TABLE aegis.authorization_code (
    code           TEXT PRIMARY KEY,            -- opaque, single-use
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    realm_id       UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    client_id      TEXT NOT NULL,               -- OAuth client_id
    account_id     UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
    redirect_uri   TEXT NOT NULL,
    scopes         JSONB NOT NULL DEFAULT '[]',
    pkce_challenge TEXT,                         -- S256 challenge (PKCE)
    nonce          TEXT,
    expires_at     TIMESTAMPTZ NOT NULL,
    consumed_at    TIMESTAMPTZ
);
