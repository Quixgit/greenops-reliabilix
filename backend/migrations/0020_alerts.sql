-- +goose Up
-- Alerts (ADR-0028): persistent, deduplicated events with a lifecycle, produced by an hourly evaluation of the
-- organization's budgets and connections. The evaluation opens an alert when a condition starts and resolves it when
-- the condition ends; people can acknowledge or mute it in between.
CREATE SCHEMA alerts;

CREATE TABLE alerts.alerts (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid NOT NULL REFERENCES tenants.organizations(id),
    kind            text NOT NULL CHECK (kind IN ('budget','connection_error','data_quality')),
    severity        text NOT NULL CHECK (severity IN ('info','warning','critical')),
    status          text NOT NULL DEFAULT 'new' CHECK (status IN ('new','acknowledged','muted','resolved')),
    dedupe_key      text NOT NULL CHECK (char_length(dedupe_key) BETWEEN 1 AND 200),
    title           text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 300),
    detail          text NOT NULL DEFAULT '' CHECK (char_length(detail) <= 2000),
    project_id      uuid REFERENCES projects.projects(id) ON DELETE SET NULL,
    object_ref      text NOT NULL DEFAULT '',       -- budget id or connection id the alert is about
    value           numeric(18,4),                  -- the measured value (for example percent of budget used)
    threshold       numeric(18,4),
    first_seen_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    acknowledged_by text,
    acknowledged_at timestamptz,
    resolved_at     timestamptz
);
-- At most one live alert per condition; a resolved one is history and a new occurrence opens a new alert.
CREATE UNIQUE INDEX alerts_one_live_per_key ON alerts.alerts (tenant_id, dedupe_key) WHERE status <> 'resolved';
CREATE INDEX ON alerts.alerts (tenant_id, status, last_seen_at DESC);
SELECT platform.enable_tenant_rls('alerts.alerts', 'tenant_id');

GRANT USAGE ON SCHEMA alerts TO greenops_api, greenops_worker, greenops_fanout;
GRANT SELECT, UPDATE ON alerts.alerts TO greenops_api;
GRANT SELECT, INSERT, UPDATE ON alerts.alerts TO greenops_worker;
GRANT SELECT, DELETE ON alerts.alerts TO greenops_fanout; -- organization erasure (ADR-0027)

-- +goose Down
DROP TABLE alerts.alerts;
DROP SCHEMA alerts;
