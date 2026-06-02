-- Read-after-write versioning. A monotonic write_version is bumped by a
-- statement-level trigger on every authz source table, so any mutation —
-- through any code path — advances it without per-usecase instrumentation.
-- projection_version records the write_version captured when the projection
-- was last rebuilt. A caller that wrote at version W can pass MinVersion=W to
-- Check; the read is served only once projection_version >= W, otherwise it is
-- reported stale. Single-row counter: authz writes serialize briefly on it,
-- which is fine for a config-write workload (Check never touches it).

CREATE TABLE aegis.authz_version (
    id                 BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    write_version      BIGINT NOT NULL DEFAULT 0,
    projection_version BIGINT NOT NULL DEFAULT 0
);
INSERT INTO aegis.authz_version (id) VALUES (TRUE);

CREATE FUNCTION aegis.bump_authz_version() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE aegis.authz_version SET write_version = write_version + 1;
    RETURN NULL;
END;
$$;

CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.acl
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.role
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.role_permission
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.role_composition
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.permission
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.permission_inheritance
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.actor_set
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.actor_set_member
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
CREATE TRIGGER bump_authz_version AFTER INSERT OR UPDATE OR DELETE ON aegis.resource
    FOR EACH STATEMENT EXECUTE FUNCTION aegis.bump_authz_version();
