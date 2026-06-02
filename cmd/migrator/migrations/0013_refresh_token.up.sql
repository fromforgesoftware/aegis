-- Refresh tokens for the OAuth code grant: opaque, hashed at rest, rotated on
-- every use with a rotated_from chain so a replayed (already-used) token can be
-- detected and the whole session revoked. session_id ties a token to the login
-- session it was minted under (carried through the authorization code).

ALTER TABLE aegis.authorization_code
    ADD COLUMN session_id UUID REFERENCES aegis.session(id) ON DELETE CASCADE;

CREATE TABLE aegis.refresh_token (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    session_id   UUID NOT NULL REFERENCES aegis.session(id) ON DELETE CASCADE,
    client_id    TEXT NOT NULL,                                   -- OAuth client_id
    token_hash   TEXT NOT NULL UNIQUE,                            -- SHA-256 of the opaque token
    scopes       JSONB NOT NULL DEFAULT '[]',                     -- granted scopes, carried across rotation
    rotated_from UUID REFERENCES aegis.refresh_token(id),         -- previous token in the chain
    used_at      TIMESTAMPTZ,                                     -- set on rotation; reuse ⇒ revoke session
    expires_at   TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_refresh_token_session ON aegis.refresh_token (session_id);
