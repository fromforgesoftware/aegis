-- The uuid-ossp extension and the aegis schema are created by
-- migrations/common-pre-migration/0001_schema.sql, which runs before any
-- versioned migration.
CREATE TABLE aegis.realm (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    name         TEXT NOT NULL UNIQUE,
    display_name TEXT
);
