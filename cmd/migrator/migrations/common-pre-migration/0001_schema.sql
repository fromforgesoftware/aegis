-- Bootstrap that runs before the versioned migrations (and before
-- golang-migrate creates its schema_migrations tracking table), so a
-- fresh database has the uuid-ossp extension and the aegis schema in
-- place. Without this, golang-migrate would try to create its tracking
-- table in a search_path schema (aegis) that doesn't exist yet on the
-- very first deploy. Idempotent — re-runs harmlessly on every migrate.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp" SCHEMA public;
CREATE SCHEMA IF NOT EXISTS aegis;
