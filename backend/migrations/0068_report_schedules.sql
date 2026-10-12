-- +goose Up
-- Scheduled reports (ADR-0050). A schedule asks for the same report every week or month and tells chosen members of the organization by e-mail
-- when it is ready. The e-mail carries a link, not the file: the file stays behind the sign-in. Recipients must be members; the
-- membership is checked when the schedule is made and again every time a report is sent.
CREATE TABLE reports.schedules (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    project_id  uuid REFERENCES projects.projects(id) ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('carbon','sci','finops','sustainability')),
    format      text NOT NULL CHECK (format IN ('pdf','csv','json')),
    cadence     text NOT NULL CHECK (cadence IN ('weekly','monthly')),
    recipients  text[] NOT NULL DEFAULT '{}' CHECK (cardinality(recipients) <= 20),
    enabled     boolean NOT NULL DEFAULT true,
    next_run_at timestamptz NOT NULL,
    last_run_at timestamptz,
    created_by  text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON reports.schedules (tenant_id, created_at DESC);
CREATE INDEX ON reports.schedules (next_run_at) WHERE enabled;
SELECT platform.enable_tenant_rls('reports.schedules', 'tenant_id');
GRANT SELECT, INSERT, UPDATE, DELETE ON reports.schedules TO greenops_api;
GRANT SELECT, UPDATE ON reports.schedules TO greenops_worker;
GRANT SELECT ON reports.schedules TO greenops_fanout;

-- Which report a schedule made, so the worker knows whom to tell when it is ready.
CREATE TABLE reports.schedule_runs (
    report_id   uuid PRIMARY KEY REFERENCES reports.reports(id) ON DELETE CASCADE,
    tenant_id   uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    schedule_id uuid NOT NULL REFERENCES reports.schedules(id) ON DELETE CASCADE
);
SELECT platform.enable_tenant_rls('reports.schedule_runs', 'tenant_id');
GRANT SELECT ON reports.schedule_runs TO greenops_api;
GRANT SELECT, INSERT ON reports.schedule_runs TO greenops_worker;
GRANT SELECT ON reports.schedule_runs TO greenops_fanout;
-- The hourly job asks for the scheduled reports itself (the API does it for a person's request).
GRANT INSERT ON reports.reports TO greenops_worker;

-- Schedules whose time has come, across organizations. Called by the hourly job only.
-- +goose StatementBegin
CREATE FUNCTION platform.due_report_schedules(p_now timestamptz) RETURNS TABLE (tenant_id uuid, id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT s.tenant_id, s.id FROM reports.schedules s WHERE s.enabled AND s.next_run_at <= p_now ORDER BY s.next_run_at LIMIT 200
$$;
-- +goose StatementEnd

-- Of the given addresses, those that belong to a current member of the organization (lower case). Users are not readable by
-- the roles that send reports, so the check is made here and nowhere else.
-- +goose StatementBegin
CREATE FUNCTION platform.member_emails(p_tenant uuid, p_emails text[]) RETURNS text[]
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
  SELECT coalesce(array_agg(DISTINCT lower(u.email)), '{}') FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
   WHERE m.organization_id = p_tenant AND u.email <> '' AND lower(u.email) = ANY (SELECT lower(e) FROM unnest(p_emails) AS e)
$$;
-- +goose StatementEnd
ALTER FUNCTION platform.due_report_schedules(timestamptz) OWNER TO greenops_fanout;
ALTER FUNCTION platform.member_emails(uuid, text[]) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.due_report_schedules(timestamptz), platform.member_emails(uuid, text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.due_report_schedules(timestamptz) TO greenops_worker;
GRANT EXECUTE ON FUNCTION platform.member_emails(uuid, text[]) TO greenops_worker, greenops_api;

-- +goose Down
DROP FUNCTION platform.member_emails(uuid, text[]);
DROP FUNCTION platform.due_report_schedules(timestamptz);
DROP TABLE reports.schedule_runs, reports.schedules;
