CREATE TABLE aegis.config_state (
    realm_id     UUID PRIMARY KEY REFERENCES aegis.realm(id) ON DELETE CASCADE,
    applied_hash TEXT NOT NULL,
    snapshot     JSONB NOT NULL,
    applied_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
