CREATE TABLE aegis.outbox (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    kind        TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts    INTEGER     NOT NULL DEFAULT 0,
    retry_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error  TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'pending'
);

CREATE INDEX idx_aegis_outbox_pending_retry
    ON aegis.outbox (retry_at)
    WHERE status = 'pending';
