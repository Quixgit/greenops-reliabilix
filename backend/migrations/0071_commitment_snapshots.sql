-- +goose Up
-- The Savings Plans and reservations a connected cloud account already holds, as last read (ADR-0054): one row per connection and kind,
-- replaced at every reading. Read-only data about the customer's own account; nothing here can buy or change a commitment.
CREATE TABLE cloudaccounts.commitment_snapshots (
    connection_id   uuid NOT NULL REFERENCES cloudaccounts.connections(id) ON DELETE CASCADE,
    tenant_id       uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('savings_plans', 'reservations')),
    period_from     date NOT NULL,
    period_to       date NOT NULL,
    currency        char(3) NOT NULL,
    utilization_pct numeric(6,1),
    coverage_pct    numeric(6,1),
    commitment      numeric(18,2),
    unused          numeric(18,2),
    net_savings     numeric(18,2),
    fetched_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (connection_id, kind)
);
CREATE INDEX ON cloudaccounts.commitment_snapshots (tenant_id);
SELECT platform.enable_tenant_rls('cloudaccounts.commitment_snapshots', 'tenant_id');
GRANT SELECT ON cloudaccounts.commitment_snapshots TO greenops_api, greenops_fanout;
GRANT SELECT, INSERT, UPDATE, DELETE ON cloudaccounts.commitment_snapshots TO greenops_worker;

-- +goose Down
DROP TABLE cloudaccounts.commitment_snapshots;
