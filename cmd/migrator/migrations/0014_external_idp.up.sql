-- External identity-provider configuration (per realm) and the account-to-
-- upstream-identity binding. Wave 4 ships brokering for Firebase, Google,
-- GitHub, Apple, and generic custom OIDC; ldap is reserved for Wave 14.

CREATE TYPE aegis.external_idp_kind AS ENUM
    ('FIREBASE', 'OAUTH_GOOGLE', 'OAUTH_GITHUB', 'OAUTH_APPLE', 'OIDC_CUSTOM', 'LDAP');

CREATE TABLE aegis.external_idp_config (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ,
    realm_id           UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    kind               aegis.external_idp_kind NOT NULL,
    name               TEXT NOT NULL,
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    client_id          TEXT,
    client_secret_encrypted BYTEA,
    discovery_url      TEXT,
    issuer             TEXT,
    scopes             JSONB NOT NULL DEFAULT '[]',
    config             JSONB NOT NULL DEFAULT '{}',
    UNIQUE (realm_id, kind, name)
);
CREATE INDEX idx_external_idp_config_realm ON aegis.external_idp_config (realm_id);

CREATE TABLE aegis.account_external_id (
    account_id  UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    kind        aegis.external_idp_kind NOT NULL,
    external_id TEXT NOT NULL,
    PRIMARY KEY (account_id, kind, external_id),
    UNIQUE (kind, external_id)
);
