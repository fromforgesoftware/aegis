-- ACL bindings: the slice that finally connects a subject to a role on a
-- resource. A subject is either an account or an actor_set (a named group of
-- accounts), so groups can be granted access once and members inherit it.
-- The closure materialised view that fans these bindings out across the
-- resource hierarchy lands in the next slice.

CREATE TABLE aegis.actor_set (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    realm_id    UUID NOT NULL REFERENCES aegis.realm(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    UNIQUE (realm_id, name)
);

CREATE TABLE aegis.actor_set_member (
    actor_set_id UUID NOT NULL REFERENCES aegis.actor_set(id) ON DELETE CASCADE,
    account_id   UUID NOT NULL REFERENCES aegis.account(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (actor_set_id, account_id)
);
CREATE INDEX idx_actor_set_member_account ON aegis.actor_set_member (account_id);

CREATE TYPE aegis.subject_type AS ENUM ('ACCOUNT', 'ACTOR_SET');

-- subject_id is polymorphic (account or actor_set per subject_type), so it
-- carries no FK; the usecase verifies the subject exists and shares the
-- resource's realm before inserting.
CREATE TABLE aegis.acl (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    resource_id  UUID NOT NULL REFERENCES aegis.resource(id) ON DELETE CASCADE,
    role_id      UUID NOT NULL REFERENCES aegis.role(id) ON DELETE CASCADE,
    subject_type aegis.subject_type NOT NULL,
    subject_id   UUID NOT NULL,
    UNIQUE (resource_id, role_id, subject_type, subject_id)
);
CREATE INDEX idx_acl_resource ON aegis.acl (resource_id);
CREATE INDEX idx_acl_subject ON aegis.acl (subject_type, subject_id);
CREATE INDEX idx_acl_role ON aegis.acl (role_id);
