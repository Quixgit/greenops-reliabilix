-- +goose Up
CREATE TABLE finops.budgets (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id uuid NOT NULL,
    amount     numeric(18,2) NOT NULL CHECK (amount >= 0),
    currency   char(3) NOT NULL,
    period     text NOT NULL CHECK (period IN ('monthly','quarterly','yearly'))
);
SELECT platform.enable_tenant_rls('finops.budgets', 'tenant_id');

CREATE TABLE recommendations.recommendations (
    id                             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                      uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id                     uuid NOT NULL,
    type                           text NOT NULL CHECK (type IN ('rightsizing','region_shift','time_shift','spot_migration')),
    estimated_carbon_reduction_pct numeric(6,2) NOT NULL,
    estimated_cost_impact          numeric(18,2) NOT NULL,   -- negative = saving
    confidence                     numeric(4,3) NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    compliance_check               jsonb NOT NULL DEFAULT '{}',
    status                         text NOT NULL DEFAULT 'open' CHECK (status IN ('open','approved','applied','dismissed')),
    created_at                     timestamptz NOT NULL DEFAULT now()
);
SELECT platform.enable_tenant_rls('recommendations.recommendations', 'tenant_id');

-- Metadata only; the file itself lives in S3 under tenants/{tenant_id}/reports/.
CREATE TABLE reports.reports (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    kind       text NOT NULL,
    format     text NOT NULL CHECK (format IN ('pdf','csv','json')),
    object_key text,
    status     text NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL DEFAULT now()
);
SELECT platform.enable_tenant_rls('reports.reports', 'tenant_id');

-- +goose Down
DROP TABLE reports.reports; DROP TABLE recommendations.recommendations; DROP TABLE finops.budgets;
