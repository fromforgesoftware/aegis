-- Per-account active organization: which org an account's freshly-issued
-- access tokens carry (the org_id claim). Set by POST /organizations/{id}/activate
-- after a membership check; read at token issuance/refresh.
CREATE TABLE aegis.account_active_org (
    account_id      UUID PRIMARY KEY REFERENCES aegis.account(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES aegis.organization(id) ON DELETE CASCADE,
    org_role        TEXT NOT NULL DEFAULT '',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
