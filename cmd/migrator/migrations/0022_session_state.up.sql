-- Stateful session topology (opt-in). session_state tracks where a session
-- currently is — realm, shard, region — for game/tenant deployments that need
-- presence and shard-transition awareness; an idle-purge sweeper reclaims
-- stale rows. realm_quota_policy caps a countable resource per realm (e.g.
-- characters-per-realm), enforced against a pluggable usage counter.

CREATE TABLE aegis.session_state (
  session_id        UUID PRIMARY KEY REFERENCES aegis.session(id) ON DELETE CASCADE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  account_id        UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
  current_realm_id  UUID REFERENCES aegis.realm(id) ON DELETE SET NULL,
  current_shard     TEXT,
  region            TEXT NOT NULL DEFAULT 'default',
  ip                TEXT,
  user_agent        TEXT,
  last_active       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_session_state_account ON aegis.session_state (account_id);
CREATE INDEX idx_session_state_last_active ON aegis.session_state (last_active);

CREATE TABLE aegis.realm_quota_policy (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at    TIMESTAMPTZ,
  realm_id      UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
  resource_type TEXT NOT NULL,
  max_count     INTEGER NOT NULL,
  UNIQUE (realm_id, resource_type)
);
