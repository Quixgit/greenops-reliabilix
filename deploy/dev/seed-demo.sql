-- DEV ONLY demo dataset (make seed-demo): one AWS account, 60 days of FOCUS usage, grid intensity and audit
-- events, written to the DATABASE (the frontend has no mock data). Carbon rows are computed by the real
-- engine: after loading, run `make recalc` (queues carbon:recalculate_all) or POST /api/v1/carbon/recalculate.
-- Numbers are synthetic and clearly labelled by the "demo" project/account names.
\set tenant '00000000-0000-4000-8000-000000000001'
BEGIN;
INSERT INTO tenants.organizations (id, name) VALUES (:'tenant', 'Dev Organization') ON CONFLICT DO NOTHING;
INSERT INTO projects.projects (id, tenant_id, name, functional_unit)
VALUES ('00000000-0000-4000-8000-0000000000a1', :'tenant', 'demo-production', 'request') ON CONFLICT DO NOTHING;
INSERT INTO cloudaccounts.connections (tenant_id, project_id, provider, account_ref, credential_ref, external_id, sync_status, last_sync_at, synced_through)
VALUES (:'tenant', '00000000-0000-4000-8000-0000000000a1', 'aws', '000000000000',
        'arn:aws:iam::000000000000:role/ReliabilixReadOnly-demo', 'rlx-demo0000000000000000000000000000000000', 'healthy', now(),
        (current_date - 1)) ON CONFLICT DO NOTHING;
INSERT INTO projects.functional_unit_counts (tenant_id, project_id, period_start, period_end, units)
VALUES (:'tenant', '00000000-0000-4000-8000-0000000000a1', current_date - 59, current_date, 60000000) ON CONFLICT DO NOTHING;
INSERT INTO projects.policies (project_id, tenant_id, allowed_regions)
VALUES ('00000000-0000-4000-8000-0000000000a1', :'tenant', ARRAY['us-east-1','us-west-2','eu-central-1','eu-west-1']) ON CONFLICT DO NOTHING;

-- FOCUS usage: 6 services x 60 days (cost only, like Cost Explorer)
WITH svc(name, category, rtype, region, base_cost) AS (VALUES
  ('Amazon Elastic Compute Cloud - Compute','compute','instance','us-east-1',   110),
  ('AWS Lambda','compute','function','eu-central-1',                             18),
  ('Amazon Relational Database Service','database','db_instance','eu-central-1', 62),
  ('Amazon Simple Storage Service','storage','bucket','us-east-1',               24),
  ('Amazon Elastic Block Store','storage','volume','us-east-1',                  19),
  ('Amazon Virtual Private Cloud','networking','nat_gateway','us-east-1',        31))
INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, resource_id, resource_type, service_name, service_category,
                                 region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
SELECT :'tenant', '00000000-0000-4000-8000-0000000000a1', 'cost_explorer', 'aws', '', '', s.name, s.category, s.region,
       round((s.base_cost * (0.85 + 0.3 * random()) * (1 + (60 - g) * 0.002))::numeric, 4),
       round((s.base_cost * (0.85 + 0.3 * random()) * (1 + (60 - g) * 0.002))::numeric, 4), 'USD',
       date_trunc('day', now()) - (g || ' days')::interval, date_trunc('day', now()) - ((g - 1) || ' days')::interval
FROM svc s, generate_series(1, 60) g
ON CONFLICT DO NOTHING;

-- grid intensity (reference data): a reading now and one 26h earlier, per region
INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh)
SELECT 'electricitymaps', r.region, now() - (h || ' hours')::interval,
       r.g * (1 + (h / 24) * 0.03 * (CASE WHEN r.region IN ('eu-north-1','eu-west-1') THEN -1 ELSE 1 END))
FROM (VALUES ('eu-north-1', 28), ('eu-west-1', 290), ('eu-central-1', 340), ('us-west-2', 150), ('us-east-1', 380)) r(region, g),
     (VALUES (1), (27)) hh(h)
ON CONFLICT DO NOTHING;

INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, created_at) VALUES
  (:'tenant', 'dev-user', 'project.created',          'project:demo-production', now() - interval '29 days'),
  (:'tenant', 'dev-user', 'cloud_connection.created', 'connection:aws',          now() - interval '29 days'),
  (:'tenant', 'system',   'cloud_sync.completed',     'connection:aws',          now() - interval '2 hours');
COMMIT;
