-- The resource registry: consumers register a row here when they create a
-- domain object that participates in authz (workspace, doc, game-realm…).
-- parent_id forms the hierarchy the MV walks to inherit grants; inherit_via
-- labels the edge type so later slices can express custom inheritance graphs
-- without requiring a strict tree.

CREATE TABLE aegis.resource (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,
    realm_id         UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    type             TEXT NOT NULL,
    owner_account_id UUID REFERENCES aegis.account(id) ON DELETE SET NULL,
    parent_id        UUID REFERENCES aegis.resource(id) ON DELETE CASCADE,
    inherit_via      TEXT,
    visibility       TEXT NOT NULL DEFAULT 'PRIVATE'
);
CREATE INDEX idx_resource_realm_type ON aegis.resource (realm_id, type);
CREATE INDEX idx_resource_parent ON aegis.resource (parent_id);
CREATE INDEX idx_resource_owner ON aegis.resource (owner_account_id);
