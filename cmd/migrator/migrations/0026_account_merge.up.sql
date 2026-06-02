-- account_merge_event records each merge so the operation is auditable and a
-- merged-away account id can be traced to its survivor.
CREATE TABLE aegis.account_merge_event (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id  UUID NOT NULL,
    target_id  UUID NOT NULL,
    realm_id   UUID NOT NULL,
    summary    JSONB NOT NULL DEFAULT '{}'::jsonb,
    merged_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_account_merge_event_target ON aegis.account_merge_event (target_id);
CREATE INDEX idx_account_merge_event_source ON aegis.account_merge_event (source_id);
