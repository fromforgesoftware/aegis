-- RBAC core: the permission catalog seeded by the consuming service, roles
-- (per realm, system or admin-created custom), and the role↔permission
-- junction. ACL bindings, resources, and the materialised closure all land
-- in subsequent Wave 5 slices.

CREATE TABLE aegis.permission (
    id            TEXT PRIMARY KEY,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resource_type TEXT NOT NULL,
    verb          TEXT NOT NULL,
    description   TEXT
);

CREATE TYPE aegis.role_kind AS ENUM ('SYSTEM', 'CUSTOM');

CREATE TABLE aegis.role (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    realm_id      UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    description   TEXT,
    kind          aegis.role_kind NOT NULL,
    UNIQUE (realm_id, name, resource_type)
);

CREATE TABLE aegis.role_permission (
    role_id       UUID NOT NULL REFERENCES aegis.role(id) ON DELETE CASCADE,
    permission_id TEXT NOT NULL REFERENCES aegis.permission(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);
CREATE INDEX idx_role_permission_permission ON aegis.role_permission (permission_id);
