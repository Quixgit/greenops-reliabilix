-- DEV ONLY demo dataset (make seed-demo). One AWS account, 30 days of usage, calculations,
-- grid intensity and audit events - written to the DATABASE, never mocked in the frontend.
-- Numbers are synthetic and clearly labelled by the "demo" project/account names.
\set tenant '00000000-0000-4000-8000-000000000001'
BEGIN;
INSERT INTO tenants.organizations (id, name) VALUES (:'tenant', 'Dev Organization') ON CONFLICT DO NOTHING;
INSERT INTO projects.projects (id, tenant_id, name, functional_unit)
VALUES ('00000000-0000-4000-8000-0000000000a1', :'tenant', 'demo-production', 'request') ON CONFLICT DO NOTHING;
INSERT INTO cloudaccounts.connections (tenant_id, project_id, provider, account_ref, credential_ref, sync_status, last_sync_at)
VALUES (:'tenant', '00000000-0000-4000-8000-0000000000a1', 'aws', '000000000000-demo',
        'arn:aws:iam::000000000000:role/ReliabilixReadOnly-demo', 'healthy', now())
ON CONFLICT DO NOTHING;

-- usage: 6 services x 30 days
WITH svc(name, category, resource_type, base_cost, base_amt) AS (VALUES
  ('Amazon EC2','compute','instance',   110, 2400),
  ('AWS Lambda','compute','function',    18,  300),
  ('Amazon RDS','database','db_instance', 62, 1100),
  ('Amazon S3','storage','bucket',        24,  900),
  ('Amazon EBS','storage','volume',       19,  700),
  ('Amazon VPC','networking','nat_gateway',31,  200))
INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, resource_id, resource_type, service_name,
                                 service_category, region, usage_amount, usage_unit, cost, currency, recorded_at)
SELECT :'tenant', '00000000-0000-4000-8000-0000000000a1', 'cost_explorer', 'aws',
       'demo-' || lower(replace(s.name,' ','-')), s.resource_type, s.name, s.category,
       CASE WHEN s.name IN ('Amazon EC2','Amazon RDS') THEN 'eu-central-1' ELSE 'us-east-1' END,
       s.base_amt * (0.9 + 0.2 * random()), 'vcpu_hours',
       s.base_cost * (0.85 + 0.3 * random()) * (1 + (30 - g) * 0.004), 'USD',
       date_trunc('day', now()) - (g || ' days')::interval
FROM svc s, generate_series(0, 59) g
ON CONFLICT DO NOTHING;

-- carbon: derived from the same rows with a service-dependent factor (demo only)
INSERT INTO carbon.calculations (tenant_id, project_id, provider, region, service_name, service_category,
                                 period_start, period_end, energy_kwh, carbon_kg_co2e, sci_score, methodology_version)
SELECT u.tenant_id, u.project_id, u.provider, u.region, u.service_name, u.service_category,
       u.recorded_at, u.recorded_at + interval '1 day',
       u.usage_amount * 0.0025,
       u.usage_amount * 0.0025 * CASE u.region WHEN 'eu-central-1' THEN 0.31 ELSE 0.38 END,
       40 + 10 * random(), 'CCF-2026.1'
FROM usage.usage_records u WHERE u.tenant_id = :'tenant';

-- grid intensity: two readings per region (now and 24h+ earlier) from the "latest" provider
INSERT INTO carbon.grid_intensity (provider, region, ts, g_co2e_per_kwh)
SELECT 'electricitymaps', r.region, now() - (h || ' hours')::interval, r.g * (1 + (h / 24) * 0.03 * (CASE WHEN r.region IN ('eu-north-1','eu-west-1') THEN -1 ELSE 1 END))
FROM (VALUES ('eu-north-1', 28), ('eu-west-1', 290), ('eu-central-1', 340), ('us-west-2', 150), ('us-east-1', 380)) r(region, g),
     (VALUES (0), (26)) hh(h)
ON CONFLICT DO NOTHING;

INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, created_at) VALUES
  (:'tenant', 'dev-user', 'project.created',          'project:demo-production', now() - interval '29 days'),
  (:'tenant', 'dev-user', 'cloud_connection.created', 'connection:aws', now() - interval '29 days'),
  (:'tenant', 'system',   'cloud_sync.completed',     'connection:aws', now() - interval '2 hours');
COMMIT;
