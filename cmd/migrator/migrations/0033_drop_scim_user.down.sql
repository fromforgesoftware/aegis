CREATE TABLE aegis.scim_user (
    account_id  UUID PRIMARY KEY REFERENCES aegis.account(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    realm_id    UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    external_id TEXT
);
CREATE UNIQUE INDEX idx_scim_user_external ON aegis.scim_user (realm_id, external_id)
    WHERE external_id IS NOT NULL;
