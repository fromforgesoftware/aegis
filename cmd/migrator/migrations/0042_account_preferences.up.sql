-- User-scoped preferences: locale, theme, notification channels.
--
-- Rows of (owner, key, value) rather than a JSONB document on the account, and
-- rather than a typed column per preference. Both alternatives were considered:
--
--   A JSONB blob means every write is a read-modify-write of the whole document,
--   so two browser tabs saving different preferences silently lose one of them;
--   it also makes "which key changed" unanswerable, which matters because these
--   writes are audited.
--
--   A column per preference validates at the database, but needs a migration —
--   and an aegis release, and a chart release — every time a product wants one
--   more setting. That is the cost that kills settings systems as a product grows.
--
-- Rows give partial updates, per-key audit, and sparse reads. The type safety a
-- column would have given is provided instead by the registry in
-- internal/domain/preference.go, which validates on the way in and is also what
-- the settings UI renders from.
--
-- Two tables, not one with a nullable owner: an organization default and an
-- account override are resolved together on every read, and a single table with
-- "either organization_id or account_id" would need a CHECK constraint plus two
-- partial indexes to say what two tables say plainly. Both cascade from their
-- owner, so deleting an account or an organization takes its preferences with it.

CREATE TABLE aegis.account_preference (
    account_id UUID        NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
    key        TEXT        NOT NULL,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, key)
);

CREATE TABLE aegis.organization_preference (
    organization_id UUID        NOT NULL REFERENCES aegis.organization(id) ON DELETE CASCADE,
    key             TEXT        NOT NULL,
    value           TEXT        NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, key)
);

-- The composite primary keys already serve the only two read patterns — every
-- read is "all preferences for this owner" or "these keys for this owner", both
-- of which are a prefix scan on the leading owner column. No further index is
-- warranted, and an index on `key` alone would serve no query this API can issue.
