-- Per-realm JWT signing keys (RS256). The private key is stored
-- envelope-encrypted (never plaintext); the public half is kept as a JWK
-- for the realm's JWKS. status drives rotation: ACTIVE signs, GRACE still
-- verifies (kept in JWKS), RETIRED is removed.
CREATE TABLE aegis.signing_key (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    realm_id    UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    kid         TEXT NOT NULL,
    algorithm   TEXT NOT NULL,
    public_jwk  JSONB NOT NULL,
    private_key BYTEA NOT NULL,                 -- envelope-encrypted PKCS#8
    status      TEXT NOT NULL DEFAULT 'ACTIVE', -- ACTIVE / GRACE / RETIRED
    not_before  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    not_after   TIMESTAMPTZ,
    UNIQUE (realm_id, kid)
);

CREATE INDEX idx_signing_key_realm_status ON aegis.signing_key (realm_id, status);
