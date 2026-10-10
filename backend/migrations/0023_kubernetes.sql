-- +goose Up
-- Kubernetes workload energy (ADR-0030). A customer-side agent reads container energy (Kepler, via Prometheus) and
-- pushes hourly kWh per workload with an agent API key. Rows are *attribution inside the nodes' energy*: the cloud
-- bill already carries that energy, so these figures are never added to the organization's totals.
ALTER TABLE tenants.api_keys DROP CONSTRAINT api_keys_role_check;
ALTER TABLE tenants.api_keys ADD CONSTRAINT api_keys_role_check CHECK (role IN ('ci','viewer','agent'));

CREATE SCHEMA kubernetes;

CREATE TABLE kubernetes.workload_energy (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           uuid NOT NULL REFERENCES tenants.organizations(id),
    project_id          uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
    cluster             text NOT NULL CHECK (cluster ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,126}$'),
    namespace           text NOT NULL CHECK (namespace ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,126}$'),
    workload            text NOT NULL CHECK (workload ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,126}$'),
    kind                text NOT NULL CHECK (kind IN ('deployment','statefulset','daemonset','job','cronjob','pod','other')),
    period_start        timestamptz NOT NULL,                  -- the start of a UTC hour
    energy_kwh          numeric(14,6) NOT NULL CHECK (energy_kwh >= 0),  -- measured at the node (no PUE)
    region_id           text NOT NULL,
    intensity_g_per_kwh numeric(10,3),                          -- the grid reading used; NULL = no grid data then
    carbon_kg_co2e      numeric(14,6),                          -- energy x PUE x intensity; NULL when intensity is unknown
    source              text NOT NULL DEFAULT 'kepler' CHECK (source IN ('kepler','other')),
    methodology_version text NOT NULL,
    ingested_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, project_id, cluster, namespace, workload, period_start)
);
CREATE INDEX ON kubernetes.workload_energy (tenant_id, project_id, period_start);
SELECT platform.enable_tenant_rls('kubernetes.workload_energy', 'tenant_id');

GRANT USAGE ON SCHEMA kubernetes TO greenops_api, greenops_fanout;
GRANT SELECT, INSERT, UPDATE ON kubernetes.workload_energy TO greenops_api;
GRANT SELECT, DELETE ON kubernetes.workload_energy TO greenops_fanout; -- organization erasure (ADR-0027)

-- +goose Down
DROP TABLE kubernetes.workload_energy;
DROP SCHEMA kubernetes;
DELETE FROM tenants.api_keys WHERE role = 'agent';
ALTER TABLE tenants.api_keys DROP CONSTRAINT api_keys_role_check;
ALTER TABLE tenants.api_keys ADD CONSTRAINT api_keys_role_check CHECK (role IN ('ci','viewer'));
