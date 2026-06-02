-- Organizations: the tenant inside a realm. Each org is anchored 1:1 to an
-- authz resource (resource_id) so membership, invites, and inheritance reuse
-- the ReBAC engine. owner_account_id is the creating account.

CREATE TABLE aegis.organization (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,
    realm_id         UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    resource_id      UUID NOT NULL REFERENCES aegis.resource(id) ON DELETE CASCADE,
    owner_account_id UUID REFERENCES aegis.account(id) ON DELETE SET NULL,
    name             TEXT NOT NULL,
    slug             TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'ACTIVE',
    settings         JSONB NOT NULL DEFAULT '{}',
    UNIQUE (realm_id, slug),
    UNIQUE (resource_id)
);
CREATE INDEX idx_organization_realm ON aegis.organization (realm_id);
