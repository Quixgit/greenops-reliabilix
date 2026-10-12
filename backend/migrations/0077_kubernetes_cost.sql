-- +goose Up
-- Kubernetes cost allocation (ADR-0063): what a cluster's nodes could hold (so that what nobody holds is idle), and which billed services pay for a cluster.
CREATE TABLE kubernetes.cluster_capacity (
    tenant_id    uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    project_id   uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
    cluster      text NOT NULL CHECK (cluster ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,126}$'),
    period_start timestamptz NOT NULL,                        -- the start of a UTC hour
    cpu_cores    numeric(12,3) NOT NULL CHECK (cpu_cores >= 0),    -- allocatable CPU of the nodes in the hour
    memory_bytes numeric(20,0) NOT NULL CHECK (memory_bytes >= 0), -- allocatable memory
    ingested_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, project_id, cluster, period_start)
);
SELECT platform.enable_tenant_rls('kubernetes.cluster_capacity', 'tenant_id');
GRANT SELECT, INSERT, UPDATE ON kubernetes.cluster_capacity TO greenops_api;
GRANT SELECT, DELETE ON kubernetes.cluster_capacity TO greenops_fanout;

-- The billed services that pay for a cluster's nodes (for example "Virtual Machines" or "Amazon Elastic Compute Cloud - Compute"). Empty: every compute
-- cost of the project, which is only right when the project's compute is the cluster.
CREATE TABLE kubernetes.cluster_pools (
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects.projects(id) ON DELETE CASCADE,
    cluster    text NOT NULL CHECK (cluster ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,126}$'),
    services   text[] NOT NULL DEFAULT '{}' CHECK (cardinality(services) <= 50),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, project_id, cluster)
);
SELECT platform.enable_tenant_rls('kubernetes.cluster_pools', 'tenant_id');
GRANT SELECT, INSERT, UPDATE, DELETE ON kubernetes.cluster_pools TO greenops_api;
GRANT SELECT, DELETE ON kubernetes.cluster_pools TO greenops_fanout;

-- +goose Down
DROP TABLE kubernetes.cluster_pools, kubernetes.cluster_capacity;
