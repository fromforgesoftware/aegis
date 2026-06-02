CREATE TYPE aegis.account_type   AS ENUM ('USER', 'SERVICE');
CREATE TYPE aegis.account_status AS ENUM ('CREATED', 'ENABLED', 'DISABLED', 'BANNED');

CREATE TABLE aegis.account (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    realm_id     UUID NOT NULL REFERENCES aegis.realm(id),
    type         aegis.account_type   NOT NULL,
    status        aegis.account_status NOT NULL DEFAULT 'CREATED',
    banned_until  TIMESTAMPTZ,
    ban_reason    TEXT,
    last_login_at TIMESTAMPTZ
);

CREATE INDEX idx_account_realm ON aegis.account (realm_id);
