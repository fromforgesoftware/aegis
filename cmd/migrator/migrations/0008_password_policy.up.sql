-- Per-realm password policy. One row per realm; realms with no row fall
-- back to the default policy in the app layer (min length 8, no
-- character-class rules), so this table is purely opt-in stricter config.
CREATE TABLE aegis.password_policy (
    realm_id          UUID PRIMARY KEY REFERENCES aegis.realm(id) ON DELETE CASCADE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    min_length        INTEGER NOT NULL DEFAULT 8,
    max_length        INTEGER NOT NULL DEFAULT 0,   -- 0 = no maximum
    require_uppercase BOOLEAN NOT NULL DEFAULT FALSE,
    require_lowercase BOOLEAN NOT NULL DEFAULT FALSE,
    require_digit     BOOLEAN NOT NULL DEFAULT FALSE,
    require_symbol    BOOLEAN NOT NULL DEFAULT FALSE
);
