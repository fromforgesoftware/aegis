-- The preference KEY SPACE becomes data, declared per realm, reconciled from config.
--
-- 0042 stored preference values against a key space hardcoded in Go. That forced an
-- aegis release, a chart release and a redeploy to add one setting, and it gave every
-- product the same keys — which is wrong twice over, because products do not share
-- their settings and identity should not need a release to learn a new one.
--
-- This is the same mechanism the authz catalog already uses: the values file renders a
-- ConfigMap, a checksum annotation rolls the pods, and the server reconciles the
-- document at startup. Adding a preference is a values.yaml edit and a helm upgrade.
--
-- managed_by mirrors permission/role: it records which config document owns the row, so
-- reconciliation can prune what left the document without ever touching something
-- created by hand.

CREATE TABLE aegis.preference_spec (
    realm_id      UUID        NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    key           TEXT        NOT NULL,
    value_type    TEXT        NOT NULL,
    default_value TEXT        NOT NULL DEFAULT '',
    -- The permitted set for an enum. JSONB rather than TEXT[] to match how the rest of
    -- this schema carries list-shaped config (organization.settings), and because the
    -- document it comes from is JSON already.
    allowed       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    max_len       INT         NOT NULL DEFAULT 0,
    org_scoped    BOOLEAN     NOT NULL DEFAULT FALSE,
    -- The OIDC standard claim this key populates, empty for most. Only locale and
    -- zoneinfo are standard claims; nothing else belongs in a token.
    claim         TEXT        NOT NULL DEFAULT '',
    managed_by    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (realm_id, key)
);

-- Values now reference their spec, and the realm is carried on the row to make that
-- reference expressible.
--
-- This is the part that answers "will a deleted key leave rows behind": no, and not
-- because some sweep remembers to tidy up. The foreign key makes a value for an
-- undeclared key UNREPRESENTABLE, and ON DELETE CASCADE means pruning a spec takes its
-- values with it in the same statement. A reconciler that forgot to clean up, or crashed
-- halfway, cannot leave the orphans behind — the database will not hold them.
--
-- These two tables were introduced one release ago and hold only the values a single
-- local account has set, so backfilling realm_id from the owner is safe here in a way it
-- would not be on an established table.
ALTER TABLE aegis.account_preference ADD COLUMN realm_id UUID;
UPDATE aegis.account_preference p
   SET realm_id = a.realm_id
  FROM aegis.account a
 WHERE a.id = p.account_id;

ALTER TABLE aegis.organization_preference ADD COLUMN realm_id UUID;
UPDATE aegis.organization_preference p
   SET realm_id = o.realm_id
  FROM aegis.organization o
 WHERE o.id = p.organization_id;

-- Any row whose key predates the spec table has no declaration to point at. There is
-- nothing to preserve — the key space it was written against no longer exists — and
-- keeping it would block the constraint below.
DELETE FROM aegis.account_preference      WHERE realm_id IS NULL;
DELETE FROM aegis.organization_preference WHERE realm_id IS NULL;

ALTER TABLE aegis.account_preference      ALTER COLUMN realm_id SET NOT NULL;
ALTER TABLE aegis.organization_preference ALTER COLUMN realm_id SET NOT NULL;

ALTER TABLE aegis.account_preference
    ADD CONSTRAINT account_preference_spec_fk
    FOREIGN KEY (realm_id, key) REFERENCES aegis.preference_spec (realm_id, key)
    ON DELETE CASCADE;

ALTER TABLE aegis.organization_preference
    ADD CONSTRAINT organization_preference_spec_fk
    FOREIGN KEY (realm_id, key) REFERENCES aegis.preference_spec (realm_id, key)
    ON DELETE CASCADE;

-- The prune-safety gate counts stored values for a key before destroying its spec, and
-- that count is the only query not already served by a primary key.
CREATE INDEX idx_account_preference_realm_key ON aegis.account_preference (realm_id, key);
