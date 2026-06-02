-- GitOps declarative-config surface removed: env-bootstrap seeds the realm on
-- boot, so last-applied/drift tracking is unused. (Was 0032_config_state.)
DROP TABLE IF EXISTS aegis.config_state;
