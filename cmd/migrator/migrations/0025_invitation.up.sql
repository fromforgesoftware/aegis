-- Invitations: an admin invites an email to a pre-assigned role on a resource;
-- on accept, the accepting account is bound to that (role, resource). The
-- token is delivered out-of-band (via NotificationSender) and only its hash is
-- stored.

CREATE TABLE aegis.invitation (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at  TIMESTAMPTZ,
  realm_id    UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
  email       TEXT NOT NULL,
  invited_by  UUID REFERENCES aegis.account(id) ON DELETE SET NULL,
  role_id     UUID REFERENCES aegis.role(id) ON DELETE CASCADE,
  resource_id UUID REFERENCES aegis.resource(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,
  status      TEXT NOT NULL DEFAULT 'PENDING',
  expires_at  TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ
);
CREATE INDEX idx_invitation_realm ON aegis.invitation (realm_id);
CREATE INDEX idx_invitation_email ON aegis.invitation (email);
