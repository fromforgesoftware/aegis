ALTER TABLE aegis.organization_preference DROP CONSTRAINT IF EXISTS organization_preference_spec_fk;
ALTER TABLE aegis.account_preference      DROP CONSTRAINT IF EXISTS account_preference_spec_fk;

DROP INDEX IF EXISTS aegis.idx_account_preference_realm_key;

ALTER TABLE aegis.organization_preference DROP COLUMN IF EXISTS realm_id;
ALTER TABLE aegis.account_preference      DROP COLUMN IF EXISTS realm_id;

DROP TABLE IF EXISTS aegis.preference_spec;
