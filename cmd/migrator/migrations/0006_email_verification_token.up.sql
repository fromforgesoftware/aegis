CREATE TABLE aegis.email_verification_token (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    account_id  UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,   -- sha256(raw token); the raw token is emailed, never stored
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

CREATE INDEX idx_email_verification_token_account ON aegis.email_verification_token (account_id);
