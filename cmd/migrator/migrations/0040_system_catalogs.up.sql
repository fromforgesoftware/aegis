-- Declarative system catalogs + owner-implicit access.
--
-- Role ids become TEXT so SYSTEM roles can use their slug as the id
-- ("strategies.admin"), the same convention permissions already follow —
-- consumers bind by slug and never look a role id up. SYSTEM roles are
-- realm-less (realm_id NULL): defined once, bindable in every realm.
-- managed_by stamps permission/role rows owned by a provisioned catalog so
-- reconciliation only ever touches its own rows. The catalog table records
-- each applied document for drift detection and revisioning.
--
-- Owner-implicit access needs no MV change: the authorization reader now
-- consults the LIVE resource row (owner_account_id) alongside the projection,
-- so owners see their resources immediately, with no refresh.

-- The MV references role.id / acl.role_id, so it must be dropped before the
-- type changes and recreated (definition unchanged from 0021) after.
DROP MATERIALIZED VIEW aegis.effective_authorizations;

ALTER TABLE aegis.role_permission DROP CONSTRAINT role_permission_role_id_fkey;
ALTER TABLE aegis.acl DROP CONSTRAINT acl_role_id_fkey;
ALTER TABLE aegis.role_composition DROP CONSTRAINT role_composition_role_id_fkey;
ALTER TABLE aegis.role_composition DROP CONSTRAINT role_composition_component_role_id_fkey;
ALTER TABLE aegis.role_effective_permission DROP CONSTRAINT role_effective_permission_role_id_fkey;
ALTER TABLE aegis.invitation DROP CONSTRAINT invitation_role_id_fkey;

ALTER TABLE aegis.role ALTER COLUMN id DROP DEFAULT;
ALTER TABLE aegis.role ALTER COLUMN id TYPE TEXT USING id::text;
ALTER TABLE aegis.role ALTER COLUMN id SET DEFAULT (uuid_generate_v4())::text;
ALTER TABLE aegis.role_permission ALTER COLUMN role_id TYPE TEXT USING role_id::text;
ALTER TABLE aegis.acl ALTER COLUMN role_id TYPE TEXT USING role_id::text;
ALTER TABLE aegis.role_composition ALTER COLUMN role_id TYPE TEXT USING role_id::text;
ALTER TABLE aegis.role_composition ALTER COLUMN component_role_id TYPE TEXT USING component_role_id::text;
ALTER TABLE aegis.role_effective_permission ALTER COLUMN role_id TYPE TEXT USING role_id::text;
ALTER TABLE aegis.invitation ALTER COLUMN role_id TYPE TEXT USING role_id::text;

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

-- SYSTEM roles are realm-less; the partial unique index constrains them (the
-- table UNIQUE treats NULL realm_ids as distinct rows).
ALTER TABLE aegis.role ALTER COLUMN realm_id DROP NOT NULL;
CREATE UNIQUE INDEX idx_role_system_unique
    ON aegis.role (name, resource_type) WHERE realm_id IS NULL;

ALTER TABLE aegis.permission ADD COLUMN managed_by TEXT;
ALTER TABLE aegis.role ADD COLUMN managed_by TEXT;

CREATE TABLE aegis.catalog (
    resource_type TEXT PRIMARY KEY,
    revision      BIGINT NOT NULL DEFAULT 1,
    document      JSONB NOT NULL,
    applied_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
