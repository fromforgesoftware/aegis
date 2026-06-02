-- login_signal records the context of each login attempt so the risk evaluator
-- can tell whether an IP/device is new for the account and how many recent
-- attempts failed. Append-only; pruned by retention later.
CREATE TABLE aegis.login_signal (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    account_id UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
    realm_id   UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    ip         TEXT NOT NULL,
    device_id  TEXT NOT NULL DEFAULT '',
    succeeded  BOOLEAN NOT NULL
);

CREATE INDEX idx_login_signal_account_ip ON aegis.login_signal (account_id, ip);
CREATE INDEX idx_login_signal_account_device ON aegis.login_signal (account_id, device_id);
CREATE INDEX idx_login_signal_account_time ON aegis.login_signal (account_id, created_at);
