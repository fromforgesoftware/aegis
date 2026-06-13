-- Avatar/logo blobs live in dedicated 1:1 tables (not columns on account /
-- organization) so the large BYTEA is never loaded by the frequent
-- account/organization reads — only the explicit serve endpoint selects it.

CREATE TABLE aegis.account_avatar (
    account_id   UUID PRIMARY KEY REFERENCES aegis.account(id) ON DELETE CASCADE,
    image        BYTEA       NOT NULL,
    content_type TEXT        NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE aegis.organization_logo (
    organization_id UUID PRIMARY KEY REFERENCES aegis.organization(id) ON DELETE CASCADE,
    image           BYTEA       NOT NULL,
    content_type    TEXT        NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
