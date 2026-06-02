-- config_state records the last-applied declarative config per realm so the
-- GitOps surface (aegis apply) can detect drift — the live realm diverging from
-- what was last applied.
CREATE TABLE aegis.config_state (
    realm_id     UUID PRIMARY KEY REFERENCES aegis.realm(id) ON DELETE CASCADE,
    applied_hash TEXT NOT NULL,
    snapshot     JSONB NOT NULL,
    applied_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
