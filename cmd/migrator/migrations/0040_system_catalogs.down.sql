-- Lossy down: SYSTEM roles (slug ids, NULL realm) cannot survive the cast
-- back to UUID ids + NOT NULL realm, so they are deleted first (cascading
-- their bindings and junctions).

DROP MATERIALIZED VIEW aegis.effective_authorizations;

DELETE FROM aegis.role WHERE realm_id IS NULL;

DROP TABLE aegis.catalog;
ALTER TABLE aegis.role DROP COLUMN managed_by;
ALTER TABLE aegis.permission DROP COLUMN managed_by;

DROP INDEX aegis.idx_role_system_unique;
ALTER TABLE aegis.role ALTER COLUMN realm_id SET NOT NULL;

ALTER TABLE aegis.role_permission DROP CONSTRAINT role_permission_role_id_fkey;
ALTER TABLE aegis.acl DROP CONSTRAINT acl_role_id_fkey;
ALTER TABLE aegis.role_composition DROP CONSTRAINT role_composition_role_id_fkey;
ALTER TABLE aegis.role_composition DROP CONSTRAINT role_composition_component_role_id_fkey;
ALTER TABLE aegis.role_effective_permission DROP CONSTRAINT role_effective_permission_role_id_fkey;
ALTER TABLE aegis.invitation DROP CONSTRAINT invitation_role_id_fkey;

-- Dropped before the type change so it does not re-validate as uuid <> text
-- mid-conversion (see the up migration).
ALTER TABLE aegis.role_composition DROP CONSTRAINT role_composition_check;

ALTER TABLE aegis.role ALTER COLUMN id DROP DEFAULT;
ALTER TABLE aegis.role ALTER COLUMN id TYPE UUID USING id::uuid;
ALTER TABLE aegis.role ALTER COLUMN id SET DEFAULT uuid_generate_v4();
ALTER TABLE aegis.role_permission ALTER COLUMN role_id TYPE UUID USING role_id::uuid;
ALTER TABLE aegis.acl ALTER COLUMN role_id TYPE UUID USING role_id::uuid;
ALTER TABLE aegis.role_composition ALTER COLUMN role_id TYPE UUID USING role_id::uuid;
ALTER TABLE aegis.role_composition ALTER COLUMN component_role_id TYPE UUID USING component_role_id::uuid;
ALTER TABLE aegis.role_effective_permission ALTER COLUMN role_id TYPE UUID USING role_id::uuid;
ALTER TABLE aegis.invitation ALTER COLUMN role_id TYPE UUID USING role_id::uuid;

ALTER TABLE aegis.role_permission ADD CONSTRAINT role_permission_role_id_fkey
    FOREIGN KEY (role_id) REFERENCES aegis.role(id) ON DELETE CASCADE;
ALTER TABLE aegis.acl ADD CONSTRAINT acl_role_id_fkey
    FOREIGN KEY (role_id) REFERENCES aegis.role(id) ON DELETE CASCADE;
ALTER TABLE aegis.role_composition ADD CONSTRAINT role_composition_role_id_fkey
    FOREIGN KEY (role_id) REFERENCES aegis.role(id) ON DELETE CASCADE;
ALTER TABLE aegis.role_composition ADD CONSTRAINT role_composition_component_role_id_fkey
    FOREIGN KEY (component_role_id) REFERENCES aegis.role(id) ON DELETE CASCADE;
ALTER TABLE aegis.role_effective_permission ADD CONSTRAINT role_effective_permission_role_id_fkey
    FOREIGN KEY (role_id) REFERENCES aegis.role(id) ON DELETE CASCADE;
ALTER TABLE aegis.invitation ADD CONSTRAINT invitation_role_id_fkey
    FOREIGN KEY (role_id) REFERENCES aegis.role(id) ON DELETE CASCADE;

ALTER TABLE aegis.role_composition ADD CONSTRAINT role_composition_check
    CHECK (role_id <> component_role_id);

CREATE MATERIALIZED VIEW aegis.effective_authorizations AS
WITH RECURSIVE resource_ancestry AS (
    SELECT id AS resource_id, id AS ancestor_id, ARRAY[id] AS visited
    FROM aegis.resource
    WHERE deleted_at IS NULL
    UNION ALL
    SELECT ra.resource_id, r.parent_id, ra.visited || r.parent_id
    FROM resource_ancestry ra
    JOIN aegis.resource r ON r.id = ra.ancestor_id
    WHERE r.parent_id IS NOT NULL
      AND r.deleted_at IS NULL
      AND NOT r.parent_id = ANY(ra.visited)
),
binding_accounts AS (
    SELECT a.resource_id, a.role_id, a.subject_id AS account_id
    FROM aegis.acl a
    WHERE a.deleted_at IS NULL AND a.subject_type = 'ACCOUNT'
      AND (a.expires_at IS NULL OR a.expires_at > NOW())
    UNION
    SELECT a.resource_id, a.role_id, m.account_id
    FROM aegis.acl a
    JOIN aegis.actor_set s ON s.id = a.subject_id AND s.deleted_at IS NULL
    JOIN aegis.actor_set_member m ON m.actor_set_id = a.subject_id
    WHERE a.deleted_at IS NULL AND a.subject_type = 'ACTOR_SET'
      AND (a.expires_at IS NULL OR a.expires_at > NOW())
)
SELECT DISTINCT
    ba.account_id,
    rep.permission_id,
    ra.resource_id
FROM binding_accounts ba
JOIN aegis.role ro ON ro.id = ba.role_id AND ro.deleted_at IS NULL
JOIN aegis.role_effective_permission rep ON rep.role_id = ba.role_id
JOIN resource_ancestry ra ON ra.ancestor_id = ba.resource_id;

CREATE UNIQUE INDEX idx_effective_auth_grain
    ON aegis.effective_authorizations (account_id, resource_id, permission_id);
CREATE INDEX idx_effective_auth_resource
    ON aegis.effective_authorizations (resource_id, permission_id);
