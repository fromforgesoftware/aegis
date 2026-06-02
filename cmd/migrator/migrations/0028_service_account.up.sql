-- service_account gives a SERVICE-type account machine credentials, so a
-- non-human identity is both a first-class authz subject (bindable like a user)
-- and able to obtain tokens via client_credentials. Only the secret hash is
-- stored; the raw secret is surfaced once at creation.
CREATE TABLE aegis.service_account (
    account_id   UUID PRIMARY KEY REFERENCES aegis.account(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    realm_id     UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    client_id    TEXT NOT NULL UNIQUE,
    secret_hash  TEXT NOT NULL,
    scopes       JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_service_account_realm ON aegis.service_account (realm_id);
