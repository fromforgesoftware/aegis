-- Permission inheritance + role composition. permission_inheritance is a DAG
-- of implications (doc.write implies doc.read); role_composition builds a role
-- from component roles folded with set operators (UNION/INTERSECT/EXCLUDE) in
-- ordinal order. Neither is flattened in SQL — the resolver computes each
-- role's effective permission set in Go (ordered folds and set difference are
-- awkward in set-based SQL) and writes it to role_effective_permission, the
-- cache the projection reads. So the MV now joins role_effective_permission
-- instead of role_permission directly.

CREATE TABLE aegis.permission_inheritance (
    permission_id         TEXT NOT NULL REFERENCES aegis.permission(id) ON DELETE CASCADE,
    implied_permission_id TEXT NOT NULL REFERENCES aegis.permission(id) ON DELETE CASCADE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (permission_id, implied_permission_id),
    CHECK (permission_id <> implied_permission_id)
);
CREATE INDEX idx_permission_inheritance_implied ON aegis.permission_inheritance (implied_permission_id);

CREATE TYPE aegis.composition_operator AS ENUM ('UNION', 'INTERSECT', 'EXCLUDE');

CREATE TABLE aegis.role_composition (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    role_id           UUID NOT NULL REFERENCES aegis.role(id) ON DELETE CASCADE,
    component_role_id UUID NOT NULL REFERENCES aegis.role(id) ON DELETE CASCADE,
    operator          aegis.composition_operator NOT NULL,
    ordinal           INTEGER NOT NULL DEFAULT 0,
    CHECK (role_id <> component_role_id),
    UNIQUE (role_id, component_role_id)
);
CREATE INDEX idx_role_composition_role ON aegis.role_composition (role_id, ordinal);

-- role_effective_permission is the resolver's output cache: the fully folded,
-- inheritance-expanded permission set per role. Rebuilt wholesale on resolve.
CREATE TABLE aegis.role_effective_permission (
    role_id       UUID NOT NULL REFERENCES aegis.role(id) ON DELETE CASCADE,
    permission_id TEXT NOT NULL REFERENCES aegis.permission(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
CREATE INDEX idx_role_effective_permission_permission ON aegis.role_effective_permission (permission_id);

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
