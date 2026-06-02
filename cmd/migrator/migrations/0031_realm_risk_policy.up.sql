-- realm_risk_policy lets a realm tune the login-risk weights and thresholds.
-- Absent a row, the service-wide DefaultRiskPolicy applies.
CREATE TABLE aegis.realm_risk_policy (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    realm_id          UUID NOT NULL UNIQUE REFERENCES aegis.realm(id) ON DELETE CASCADE,
    new_ip_weight     INT NOT NULL,
    new_device_weight INT NOT NULL,
    failure_weight    INT NOT NULL,
    step_up_threshold INT NOT NULL,
    deny_threshold    INT NOT NULL
);
