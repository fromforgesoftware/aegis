-- scim_user maps a SCIM-provisioned identity to an Aegis account, carrying the
-- IdP's externalId for round-tripping. The account is the source of truth for
-- userName (email), displayName, and active (status); this table holds only the
-- SCIM bookkeeping that has no home on the account aggregate.
CREATE TABLE aegis.scim_user (
    account_id  UUID PRIMARY KEY REFERENCES aegis.account(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    realm_id    UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    external_id TEXT
);

-- externalId is unique per realm when present (Okta dedups on it).
CREATE UNIQUE INDEX idx_scim_user_external ON aegis.scim_user (realm_id, external_id)
    WHERE external_id IS NOT NULL;
