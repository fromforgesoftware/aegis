CREATE TABLE aegis.user_account (
    account_id     UUID PRIMARY KEY REFERENCES aegis.account(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ,
    email          TEXT NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    display_name   TEXT,
    photo_url      TEXT
);

-- Lookup index for login. Emails are stored normalized (lower-case) by
-- the Register usecase, so an exact-match filter uses this plain index.
-- Per-realm email uniqueness is enforced in the usecase since the realm
-- key lives on the parent aegis.account row, not here.
CREATE INDEX idx_user_account_email ON aegis.user_account (email);
