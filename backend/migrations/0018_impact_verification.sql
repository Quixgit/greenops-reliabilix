-- +goose Up
-- Verified impact (ADR-0025). A recommendation that was applied gets a measured result: what the project's
-- cost and carbon did in the weeks after the change compared with the weeks before it. The forecast stays on the
-- recommendation; the measurement is a separate row, so "expected" and "achieved" can never be mixed up.
ALTER TABLE recommendations.recommendations DROP CONSTRAINT recommendations_status_check;
ALTER TABLE recommendations.recommendations
    ADD CONSTRAINT recommendations_status_check CHECK (status IN ('open','approved','applied','verified','dismissed'));

CREATE TABLE recommendations.impact_verifications (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                uuid NOT NULL REFERENCES tenants.organizations(id),
    recommendation_id        uuid NOT NULL REFERENCES recommendations.recommendations(id) ON DELETE CASCADE,
    project_id               uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
    outcome                  text NOT NULL CHECK (outcome IN ('realized','partial','not_realized','inconclusive')),
    method                   text NOT NULL,
    baseline_from            timestamptz NOT NULL,
    baseline_to              timestamptz NOT NULL,
    after_from               timestamptz NOT NULL,
    after_to                 timestamptz NOT NULL,
    baseline_days            int NOT NULL CHECK (baseline_days >= 0),
    after_days               int NOT NULL CHECK (after_days >= 0),
    -- Measured, per 30 days; positive = saved. NULL when the windows had too little data to measure.
    measured_carbon_kg_month numeric(18,3),
    measured_cost_month      numeric(18,2),
    expected_carbon_kg_month numeric(18,3) NOT NULL,
    expected_cost_month      numeric(18,2),
    realization_pct          numeric(8,1),
    currency                 char(3),
    note                     text NOT NULL,
    verified_at              timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, recommendation_id)
);
SELECT platform.enable_tenant_rls('recommendations.impact_verifications', 'tenant_id');
GRANT SELECT ON recommendations.impact_verifications TO greenops_api;
GRANT SELECT, INSERT ON recommendations.impact_verifications TO greenops_worker;

-- +goose Down
DROP TABLE recommendations.impact_verifications;
ALTER TABLE recommendations.recommendations DROP CONSTRAINT recommendations_status_check;
UPDATE recommendations.recommendations SET status = 'applied' WHERE status = 'verified';
ALTER TABLE recommendations.recommendations
    ADD CONSTRAINT recommendations_status_check CHECK (status IN ('open','approved','applied','dismissed'));
