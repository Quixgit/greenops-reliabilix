-- +goose Up
CREATE TABLE finops.budgets (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id uuid REFERENCES projects.projects(id) ON DELETE CASCADE,   -- NULL = whole tenant
    amount     numeric(18,2) NOT NULL CHECK (amount >= 0),
    currency   char(3) NOT NULL,
    period     text NOT NULL CHECK (period IN ('monthly','quarterly','yearly')),
    created_at timestamptz NOT NULL DEFAULT now()
);
SELECT platform.enable_tenant_rls('finops.budgets', 'tenant_id');
-- Optional global reference: relative on-demand price index per region (1.0 = baseline).
-- Empty by default: without it region_shift reports carbon impact only and says cost is not estimated.
CREATE TABLE finops.region_price_index (
    provider   text NOT NULL,
    region_id  text NOT NULL,
    idx        numeric(8,4) NOT NULL CHECK (idx > 0),
    source     text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, region_id)
);

CREATE TABLE recommendations.recommendations (
    id                             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                      uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id                     uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
    type                           text NOT NULL CHECK (type IN ('rightsizing','region_shift','time_shift','spot_migration')),
    title                          text NOT NULL,
    provider                       text NOT NULL,
    service_name                   text NOT NULL DEFAULT '',
    current_region                 text NOT NULL DEFAULT '',
    recommended_region             text NOT NULL DEFAULT '',
    estimated_carbon_reduction_pct numeric(6,2) NOT NULL,
    carbon_reduction_kg_month      numeric(18,3) NOT NULL,
    estimated_cost_impact          numeric(18,2),                -- NULL = not estimated (negative = saving)
    cost_basis                     text NOT NULL DEFAULT 'not_estimated',
    confidence                     numeric(4,3) NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    compliance_check               jsonb NOT NULL,
    status                         text NOT NULL DEFAULT 'open' CHECK (status IN ('open','approved','applied','dismissed')),
    fingerprint                    text NOT NULL,                -- dedupe key of the finding
    decided_by                     text,
    decided_at                     timestamptz,
    applied_at                     timestamptz,
    created_at                     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, fingerprint)
);
CREATE INDEX ON recommendations.recommendations (tenant_id, status, created_at DESC);
SELECT platform.enable_tenant_rls('recommendations.recommendations', 'tenant_id');

-- Metadata only; the file lives in S3 under tenants/{tenant_id}/reports/.
CREATE TABLE reports.reports (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id   uuid REFERENCES projects.projects(id) ON DELETE SET NULL,
    kind         text NOT NULL CHECK (kind IN ('carbon','sci','finops')),
    format       text NOT NULL CHECK (format IN ('pdf','csv','json')),
    period_start date NOT NULL,
    period_end   date NOT NULL CHECK (period_end >= period_start),
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','ready','failed')),
    object_key   text,
    error        text,
    requested_by text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX ON reports.reports (tenant_id, created_at DESC);
SELECT platform.enable_tenant_rls('reports.reports', 'tenant_id');

-- +goose Down
DROP TABLE reports.reports; DROP TABLE recommendations.recommendations;
DROP TABLE finops.region_price_index; DROP TABLE finops.budgets;
