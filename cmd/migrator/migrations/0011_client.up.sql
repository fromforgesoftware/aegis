-- OIDC/OAuth2 client applications registered per realm. client_secret_hash
-- is set only for CONFIDENTIAL clients (the raw secret is surfaced once on
-- creation, never stored). redirect_uris / grant_types / scopes are JSONB
-- string arrays.
CREATE TABLE aegis.client (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    realm_id           UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    client_id          TEXT NOT NULL,
    client_secret_hash TEXT,
    type               TEXT NOT NULL,
    name               TEXT NOT NULL,
    grant_types        JSONB NOT NULL DEFAULT '[]',
    scopes             JSONB NOT NULL DEFAULT '[]',
    redirect_uris      JSONB NOT NULL DEFAULT '[]',
    pkce_required      BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (realm_id, client_id)
);
