DROP INDEX IF EXISTS aegis.idx_actor_set_organization;
ALTER TABLE aegis.actor_set DROP COLUMN IF EXISTS organization_id;
