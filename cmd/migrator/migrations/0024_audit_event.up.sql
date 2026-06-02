-- Built-in Postgres AuditSink: the zero-config fallback Aegis emits to when
-- @forge/hallmark isn't deployed. Monthly range-partitioned, append-only at
-- the DB level (a BEFORE UPDATE/DELETE trigger rejects mutation), GIN on
-- metadata. Same shape as hallmark.audit_event so the two are interchangeable.

CREATE TABLE aegis.audit_event (
    id            UUID NOT NULL DEFAULT uuid_generate_v4(),
    timestamp     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    realm_id      UUID,
    actor_id      TEXT,
    actor_type    TEXT,
    resource_type TEXT,
    resource_id   TEXT,
    action        TEXT NOT NULL,
    summary       TEXT,
    changes       JSONB,
    metadata      JSONB,
    ip            TEXT,
    request_id    TEXT,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

CREATE TABLE aegis.audit_event_2026_05 PARTITION OF aegis.audit_event
    FOR VALUES FROM ('2026-05-01 00:00:00+00') TO ('2026-06-01 00:00:00+00');
CREATE TABLE aegis.audit_event_2026_06 PARTITION OF aegis.audit_event
    FOR VALUES FROM ('2026-06-01 00:00:00+00') TO ('2026-07-01 00:00:00+00');
CREATE TABLE aegis.audit_event_default PARTITION OF aegis.audit_event DEFAULT;

CREATE INDEX idx_audit_event_metadata ON aegis.audit_event USING GIN (metadata);
CREATE INDEX idx_audit_event_actor ON aegis.audit_event (actor_id, timestamp);
CREATE INDEX idx_audit_event_resource ON aegis.audit_event (resource_type, resource_id, timestamp);
CREATE INDEX idx_audit_event_action ON aegis.audit_event (action, timestamp);

CREATE FUNCTION aegis.audit_event_immutable() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'aegis.audit_event is append-only: % is not permitted', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_event_no_mutation
    BEFORE UPDATE OR DELETE ON aegis.audit_event
    FOR EACH ROW EXECUTE FUNCTION aegis.audit_event_immutable();
