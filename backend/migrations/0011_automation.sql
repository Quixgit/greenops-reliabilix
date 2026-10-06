-- +goose Up
-- Automation jobs turn an APPROVED recommendation into a reviewed change plan. The platform never executes
-- changes in a customer's cloud: a human approves the plan, applies it with their own tooling and reports the
-- result. Every step is audited; the plan is generated from validated identifiers only.
CREATE SCHEMA automation;

-- Structured facts of a recommendation (resource id, instance types ...) so plans do not parse titles.
ALTER TABLE recommendations.recommendations ADD COLUMN details jsonb NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE automation.jobs (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id        uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
    recommendation_id uuid NOT NULL REFERENCES recommendations.recommendations(id) ON DELETE CASCADE,
    kind              text NOT NULL CHECK (kind IN ('ec2_resize','ec2_terminate','region_shift')),
    status            text NOT NULL DEFAULT 'planned'
                      CHECK (status IN ('planned','approved','completed','failed','rolled_back','cancelled')),
    plan              jsonb NOT NULL,
    risk_level        text NOT NULL CHECK (risk_level IN ('low','medium','high')),
    risk_factors      jsonb NOT NULL,
    rollback_plan     text NOT NULL CHECK (char_length(rollback_plan) > 0),
    created_by        text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    approved_by       text,
    approved_at       timestamptz,
    finished_by       text,
    finished_at       timestamptz,
    result_note       text CHECK (result_note IS NULL OR char_length(result_note) <= 2000),
    -- an approved job always names its approver (the application also enforces it)
    CONSTRAINT approved_has_approver CHECK (status NOT IN ('approved','completed','rolled_back') OR approved_by IS NOT NULL)
);
-- At most one live job per recommendation.
CREATE UNIQUE INDEX jobs_one_live_per_recommendation ON automation.jobs (tenant_id, recommendation_id)
    WHERE status IN ('planned','approved');
CREATE INDEX ON automation.jobs (tenant_id, created_at DESC);
SELECT platform.enable_tenant_rls('automation.jobs', 'tenant_id');

GRANT USAGE ON SCHEMA automation TO greenops_api;
GRANT SELECT, INSERT, UPDATE ON automation.jobs TO greenops_api;
-- no DELETE for anyone at runtime, and the worker has no role in automation: jobs live and die with the user's decisions

-- +goose Down
DROP TABLE automation.jobs;
ALTER TABLE recommendations.recommendations DROP COLUMN details;
DROP SCHEMA automation;
