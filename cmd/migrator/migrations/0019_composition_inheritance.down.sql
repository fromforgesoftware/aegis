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
    UNION
    SELECT a.resource_id, a.role_id, m.account_id
    FROM aegis.acl a
    JOIN aegis.actor_set s ON s.id = a.subject_id AND s.deleted_at IS NULL
    JOIN aegis.actor_set_member m ON m.actor_set_id = a.subject_id
    WHERE a.deleted_at IS NULL AND a.subject_type = 'ACTOR_SET'
)
SELECT DISTINCT
    ba.account_id,
    rp.permission_id,
    ra.resource_id
FROM binding_accounts ba
JOIN aegis.role ro ON ro.id = ba.role_id AND ro.deleted_at IS NULL
JOIN aegis.role_permission rp ON rp.role_id = ba.role_id
JOIN resource_ancestry ra ON ra.ancestor_id = ba.resource_id;

CREATE UNIQUE INDEX idx_effective_auth_grain
    ON aegis.effective_authorizations (account_id, resource_id, permission_id);
CREATE INDEX idx_effective_auth_resource
    ON aegis.effective_authorizations (resource_id, permission_id);

DROP TABLE IF EXISTS aegis.role_effective_permission;
DROP TABLE IF EXISTS aegis.role_composition;
DROP TYPE IF EXISTS aegis.composition_operator;
DROP TABLE IF EXISTS aegis.permission_inheritance;
