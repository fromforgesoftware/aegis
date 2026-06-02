-- Brute-force lockout state on the account aggregate: consecutive failed
-- logins and, once the threshold is crossed, the time the lock expires.
ALTER TABLE aegis.account
    ADD COLUMN failed_login_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN locked_until       TIMESTAMPTZ;
