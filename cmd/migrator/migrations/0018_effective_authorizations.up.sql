-- The effective-authorizations projection: a materialised view that flattens
-- the binding closure into (account_id, permission_id, resource_id) tuples so
-- the hot-path Check (next slice) is a single indexed lookup instead of a
-- recursive walk per request. It expands actor_set membership, role →
-- permission, and the resource parent_id hierarchy — a grant on a parent
-- resource applies to every descendant. Soft-deleted bindings, roles, groups,
-- and resources drop out of the closure. It is refreshed out-of-band via the
-- SECURITY DEFINER function below, never inline on the write path.

CREATE MATERIALIZED VIEW aegis.effective_authorizations AS
WITH RECURSIVE resource_ancestry AS (
    -- Each resource is its own ancestor (a binding on R applies to R); the
    -- visited array breaks any accidental parent_id cycle.
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
    -- Account-subject bindings grant the account directly.
    SELECT a.resource_id, a.role_id, a.subject_id AS account_id
    FROM aegis.acl a
    WHERE a.deleted_at IS NULL AND a.subject_type = 'ACCOUNT'
    UNION
    -- Group-subject bindings fan out to every current member.
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

-- The grain index doubles as the Check lookup (account + resource + verb) and
-- enforces the (account, resource, permission) uniqueness the projection
-- already guarantees via DISTINCT.
CREATE UNIQUE INDEX idx_effective_auth_grain
    ON aegis.effective_authorizations (account_id, resource_id, permission_id);
-- Reverse lookup: who can do X on this resource.
CREATE INDEX idx_effective_auth_resource
    ON aegis.effective_authorizations (resource_id, permission_id);

-- refresh_effective_authorizations rebuilds the projection. SECURITY DEFINER
-- so the runtime role can trigger a refresh without owning the view; a plain
-- (non-concurrent) REFRESH because CONCURRENTLY cannot run inside a function's
-- transaction. search_path is pinned to defeat injection via a mutable path.
CREATE FUNCTION aegis.refresh_effective_authorizations()
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
    REFRESH MATERIALIZED VIEW aegis.effective_authorizations;
$$;
