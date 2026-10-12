-- +goose Up
-- Sample data (ADR-0052). One click gives an organization a project with 60 days of clearly synthetic cloud spend, so the platform can be
-- judged before a real cloud account is connected; one click removes it again. The data is marked three ways: the project is listed in
-- projects.demo_markers, its usage rows have source 'demo', and the connection list is untouched (no connection is made, so the sample
-- never uses the plan's connection limit). Removal deletes only what is marked.
CREATE TABLE projects.demo_markers (
    project_id uuid PRIMARY KEY REFERENCES projects.projects(id) ON DELETE CASCADE,
    tenant_id  uuid NOT NULL REFERENCES tenants.organizations(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
SELECT platform.enable_tenant_rls('projects.demo_markers', 'tenant_id');
GRANT SELECT ON projects.demo_markers TO greenops_api, greenops_worker;
GRANT SELECT, INSERT, DELETE ON projects.demo_markers TO greenops_fanout;

-- What the two functions below need (they run as greenops_fanout): to add the sample rows and to take back only those.
GRANT SELECT, INSERT, DELETE ON projects.projects, projects.policies TO greenops_fanout;
GRANT SELECT, INSERT ON projects.functional_unit_counts TO greenops_fanout;
GRANT SELECT, INSERT, DELETE ON usage.usage_records TO greenops_fanout;
GRANT SELECT, DELETE ON carbon.calculations, recommendations.recommendations TO greenops_fanout;
GRANT SELECT ON cloudaccounts.connections TO greenops_fanout;

-- +goose StatementBegin
CREATE FUNCTION platform.demo_create(p_tenant uuid, p_name text) RETURNS uuid
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE v_project uuid;
BEGIN
  SELECT m.project_id INTO v_project FROM projects.demo_markers m WHERE m.tenant_id = p_tenant;
  IF v_project IS NOT NULL THEN RETURN v_project; END IF;

  INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES (p_tenant, left(p_name, 120), 'request') RETURNING id INTO v_project;
  INSERT INTO projects.demo_markers (project_id, tenant_id) VALUES (v_project, p_tenant);
  INSERT INTO projects.functional_unit_counts (tenant_id, project_id, period_start, period_end, units)
    VALUES (p_tenant, v_project, current_date - 59, current_date, 60000000);
  INSERT INTO projects.policies (project_id, tenant_id, allowed_regions)
    VALUES (v_project, p_tenant, ARRAY['us-east-1','us-west-2','eu-central-1','eu-west-1','eu-north-1']);

  -- six services over 60 days with a weekly rhythm and a slow climb; the amounts are invented
  INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, resource_id, resource_type, service_name, service_category,
                                   region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
  SELECT p_tenant, v_project, 'demo', 'aws', '', '', s.name, s.category, s.region,
         round(c::numeric, 4), round(c::numeric, 4), 'USD',
         date_trunc('day', now()) - (g || ' days')::interval, date_trunc('day', now()) - ((g - 1) || ' days')::interval
  FROM (VALUES
    ('Amazon Elastic Compute Cloud - Compute','compute','us-east-1',   110.0),
    ('AWS Lambda','compute','eu-central-1',                             18.0),
    ('Amazon Relational Database Service','database','eu-central-1',    62.0),
    ('Amazon Simple Storage Service','storage','us-east-1',             24.0),
    ('Amazon Elastic Block Store','storage','us-east-1',                19.0),
    ('Amazon Virtual Private Cloud','networking','us-east-1',           31.0)) AS s(name, category, region, base),
       generate_series(1, 60) AS g,
       LATERAL (SELECT s.base * (0.9 + 0.2 * random()) * (1 + (60 - g) * 0.002) * (CASE WHEN extract(isodow FROM now() - (g || ' days')::interval) >= 6 THEN 0.8 ELSE 1 END) AS c) AS v(c)
  ON CONFLICT DO NOTHING;
  RETURN v_project;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION platform.demo_remove(p_tenant uuid) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, pg_temp AS $$
DECLARE v_project uuid;
BEGIN
  SELECT m.project_id INTO v_project FROM projects.demo_markers m WHERE m.tenant_id = p_tenant;
  IF v_project IS NULL THEN RETURN false; END IF;
  -- a project a real connection was later attached to is no longer a sample: only the marker goes
  IF EXISTS (SELECT 1 FROM cloudaccounts.connections c WHERE c.project_id = v_project) THEN
    DELETE FROM projects.demo_markers WHERE project_id = v_project;
    DELETE FROM usage.usage_records WHERE tenant_id = p_tenant AND project_id = v_project AND source = 'demo';
    RETURN true;
  END IF;
  DELETE FROM recommendations.recommendations WHERE tenant_id = p_tenant AND project_id = v_project;
  DELETE FROM carbon.calculations WHERE tenant_id = p_tenant AND project_id = v_project;
  DELETE FROM usage.usage_records WHERE tenant_id = p_tenant AND project_id = v_project;
  DELETE FROM projects.policies WHERE tenant_id = p_tenant AND project_id = v_project;
  DELETE FROM projects.projects WHERE tenant_id = p_tenant AND id = v_project;
  RETURN true;
END $$;
-- +goose StatementEnd
ALTER FUNCTION platform.demo_create(uuid, text) OWNER TO greenops_fanout;
ALTER FUNCTION platform.demo_remove(uuid) OWNER TO greenops_fanout;
REVOKE ALL ON FUNCTION platform.demo_create(uuid, text), platform.demo_remove(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.demo_create(uuid, text), platform.demo_remove(uuid) TO greenops_api;

-- +goose Down
DROP FUNCTION platform.demo_remove(uuid);
DROP FUNCTION platform.demo_create(uuid, text);
DROP TABLE projects.demo_markers;
