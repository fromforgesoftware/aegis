CREATE TABLE aegis.password_credential (
    account_id UUID PRIMARY KEY REFERENCES aegis.account(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    hash       TEXT  NOT NULL,   -- PHC-encoded argon2id string
    algo       TEXT  NOT NULL,   -- 'argon2id'
    params     JSONB NOT NULL    -- {m,t,p,keyLen,saltLen} for tuning visibility
);
