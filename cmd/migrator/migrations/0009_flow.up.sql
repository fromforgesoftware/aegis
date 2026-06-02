-- Interactive auth flows (login / registration / recovery / verification).
-- Persisted so a flow is resumable and drivable headless; short-lived,
-- swept by expires_at. result_account_id records the account a login or
-- registration flow resolved to on completion.
CREATE TABLE aegis.flow (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    realm_id          UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    type              TEXT NOT NULL,
    state             TEXT NOT NULL DEFAULT 'PENDING',
    result_account_id UUID REFERENCES aegis.account(id) ON DELETE SET NULL,
    expires_at        TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_flow_expires_at ON aegis.flow (expires_at);
