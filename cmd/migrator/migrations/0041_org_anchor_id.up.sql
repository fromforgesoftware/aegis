-- An organization (a consumer's "workspace") and its authz anchor resource
-- now share ONE id, so consumers Check container permissions against the
-- org id their tokens carry (org_id claim) — no anchor lookup, the same
-- convention as every other registered resource. Backfill: repoint bindings +
-- invitations, move the anchor onto the org id, then re-anchor the org row.
-- FKs are dropped for the dance because the referenced ids change mid-flight.
--
-- The projection also gains a member-grants branch encoding the CREATE
-- CONVENTION: creation is always checked against the parent workspace (the
-- child doesn't exist yet), and every workspace member holds every
-- "<type>.create" permission on it — aegis's built-in equivalent of
-- Zanzibar's `permission create_x = member`.

ALTER TABLE aegis.organization DROP CONSTRAINT organization_resource_id_fkey;
ALTER TABLE aegis.acl DROP CONSTRAINT acl_resource_id_fkey;
ALTER TABLE aegis.invitation DROP CONSTRAINT invitation_resource_id_fkey;

UPDATE aegis.acl a SET resource_id = o.id
FROM aegis.organization o
WHERE a.resource_id = o.resource_id AND o.resource_id <> o.id;

UPDATE aegis.invitation i SET resource_id = o.id
FROM aegis.organization o
WHERE i.resource_id = o.resource_id AND o.resource_id <> o.id;

UPDATE aegis.resource r SET id = o.id
FROM aegis.organization o
WHERE r.id = o.resource_id AND o.resource_id <> o.id;

UPDATE aegis.organization o SET resource_id = o.id
WHERE o.resource_id <> o.id;

ALTER TABLE aegis.organization ADD CONSTRAINT organization_resource_id_fkey
    FOREIGN KEY (resource_id) REFERENCES aegis.resource(id) ON DELETE CASCADE;
ALTER TABLE aegis.acl ADD CONSTRAINT acl_resource_id_fkey
    FOREIGN KEY (resource_id) REFERENCES aegis.resource(id) ON DELETE CASCADE;
ALTER TABLE aegis.invitation ADD CONSTRAINT invitation_resource_id_fkey
    FOREIGN KEY (resource_id) REFERENCES aegis.resource(id) ON DELETE CASCADE;

-- Redefine the projection with the member-grants branch: any account bound on
-- an organization anchor (membership IS bindings) holds every create-verb
-- permission on that anchor.
DROP MATERIALIZED VIEW aegis.effective_authorizations;

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
SELECT DISTINCT account_id, permission_id, resource_id FROM (
    SELECT ba.account_id, rep.permission_id, ra.resource_id
    FROM binding_accounts ba
    JOIN aegis.role ro ON ro.id = ba.role_id AND ro.deleted_at IS NULL
    JOIN aegis.role_effective_permission rep ON rep.role_id = ba.role_id
    JOIN resource_ancestry ra ON ra.ancestor_id = ba.resource_id
    UNION ALL
    SELECT ba.account_id, p.id, ba.resource_id
    FROM binding_accounts ba
    JOIN aegis.resource r ON r.id = ba.resource_id AND r.type = 'organization' AND r.deleted_at IS NULL
    JOIN aegis.permission p ON p.verb = 'create'
) grants;

CREATE UNIQUE INDEX idx_effective_auth_grain
    ON aegis.effective_authorizations (account_id, resource_id, permission_id);
CREATE INDEX idx_effective_auth_resource
    ON aegis.effective_authorizations (resource_id, permission_id);
