-- MFA + step-up. mfa_enrollment holds a per-account factor (TOTP secret sealed
-- at rest); recovery_code stores one-time fallback codes (hashed); stepup_token
-- is a short-lived proof of fresh re-auth for sensitive operations; and
-- realm_acr_policy declares whether a realm requires MFA and at what assurance
-- level. WebAuthn/passkey ceremonies land in a follow-up.

CREATE TYPE aegis.mfa_factor AS ENUM ('TOTP', 'RECOVERY', 'WEBAUTHN');

CREATE TABLE aegis.mfa_enrollment (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at   TIMESTAMPTZ,
  account_id   UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
  factor       aegis.mfa_factor NOT NULL,
  secret       TEXT,                                -- sealed TOTP secret (cryptox)
  confirmed_at TIMESTAMPTZ,
  UNIQUE (account_id, factor)
);

CREATE TABLE aegis.recovery_code (
  id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  account_id UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
  code_hash  TEXT NOT NULL,
  used_at    TIMESTAMPTZ
);
CREATE INDEX idx_recovery_code_account ON aegis.recovery_code (account_id);

CREATE TABLE aegis.stepup_token (
  id          TEXT PRIMARY KEY,                     -- sha256 hex of the opaque token
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  account_id  UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
  factor      aegis.mfa_factor NOT NULL,
  acr         TEXT NOT NULL,
  expires_at  TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ
);
CREATE INDEX idx_stepup_token_account ON aegis.stepup_token (account_id);

CREATE TABLE aegis.realm_acr_policy (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at   TIMESTAMPTZ,
  realm_id     UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
  mfa_required BOOLEAN NOT NULL DEFAULT FALSE,
  required_acr TEXT NOT NULL DEFAULT 'aal2',
  UNIQUE (realm_id)
);
