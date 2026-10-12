-- Synthetic data for scripts/loadtest (docs/operations/performance.md): 21 organizations, ~3.0M usage rows and ~2.4M carbon rows over 365 days, one large
-- organization (id ...0001: 2740 resources, 20 projects) and twenty small ones. Run it on a throwaway database that has every migration applied.
-- The data is random: only the sizes are meant to look like a real organization, not the values.
\timing on
-- partitions for the 13 months of the data
SELECT platform.ensure_month_partition('usage.usage_records','recorded_at', d::date) FROM generate_series('2025-10-01'::date,'2026-10-01'::date,'1 month') d;
SELECT platform.ensure_month_partition('carbon.calculations','period_start', d::date) FROM generate_series('2025-10-01'::date,'2026-10-01'::date,'1 month') d;

-- 21 organizations: one large (id ...0001), twenty small
INSERT INTO tenants.organizations (id, name, plan)
SELECT ('00000000-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid, CASE WHEN n = 1 THEN 'Large Org' ELSE 'Small Org ' || n END, 'business' FROM generate_series(1, 21) n;
-- projects: 20 for the large one, 5 for each small one
INSERT INTO projects.projects (id, tenant_id, name, functional_unit)
SELECT ('10000000-0000-4000-8000-' || lpad((n*100 + p)::text, 12, '0'))::uuid, ('00000000-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid, 'project-' || p, 'api_request'
FROM generate_series(1, 21) n, generate_series(1, CASE WHEN n = 1 THEN 20 ELSE 5 END) p WHERE p <= CASE WHEN n = 1 THEN 20 ELSE 5 END;

-- usage: every resource has one row a day for 365 days. Large: 2740 resources (~1.0M rows), small: 274 (~100k rows)
INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, resource_id, resource_type, service_name, service_category, region_id, consumed_quantity, consumed_unit,
                                 billed_cost, effective_cost, currency, recorded_at, charge_period_end, account_ref)
SELECT t.id, ('10000000-0000-4000-8000-' || lpad((t.n*100 + 1 + (r % t.np))::text, 12, '0'))::uuid, 'cost_export',
       (ARRAY['aws','aws','aws','azure','gcp'])[1 + r % 5], 'res-' || r, 'instance',
       (ARRAY['Amazon EC2','Amazon RDS','Amazon S3','Azure VMs','Cloud SQL','Amazon EBS','Lambda','Amazon EKS'])[1 + r % 8],
       (ARRAY['compute','database','storage','compute','database','storage','compute','compute'])[1 + r % 8],
       (ARRAY['eu-central-1','eu-west-1','us-east-1','us-west-2','swedencentral','europe-west4','ap-southeast-1','eu-north-1','us-east-2','ca-central-1','eastus','westeurope'])[1 + r % 12],
       (random() * 24)::numeric(24,6), 'Hours', (0.2 + random() * 9)::numeric(18,6), (0.15 + random() * 8)::numeric(18,6), 'USD',
       d, d + interval '1 day', 'acct-' || (r % 3)
FROM (SELECT ('00000000-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid AS id, n, CASE WHEN n = 1 THEN 20 ELSE 5 END AS np, CASE WHEN n = 1 THEN 2740 ELSE 274 END AS nr FROM generate_series(1, 21) n) t
CROSS JOIN LATERAL generate_series(1, t.nr) r
CROSS JOIN LATERAL generate_series('2025-10-13'::timestamptz, '2026-10-12'::timestamptz, '1 day') d;

-- carbon: one row per day, project, service, region and method (aggregated, as the engine writes it)
INSERT INTO carbon.calculations (tenant_id, project_id, provider, region_id, service_name, service_category, method, period_start, period_end, energy_kwh, intensity_g_per_kwh,
                                 carbon_kg_co2e, methodology_version, intensity_source, intensity_basis)
SELECT t.id, ('10000000-0000-4000-8000-' || lpad((t.n*100 + p)::text, 12, '0'))::uuid, 'aws',
       (ARRAY['eu-central-1','eu-west-1','us-east-1','us-west-2','swedencentral','europe-west4','ap-southeast-1','eu-north-1','us-east-2','ca-central-1','eastus','westeurope'])[1 + g % 12],
       (ARRAY['Amazon EC2','Amazon RDS','Amazon S3','Azure VMs','Cloud SQL','Amazon EBS','Lambda','Amazon EKS'])[1 + s % 8], 'compute',
       CASE WHEN (s + g) % 3 = 0 THEN 'usage_based' ELSE 'cost_based' END, d, d + interval '1 day',
       (random() * 40)::numeric(18,6), (50 + random() * 400)::numeric(10,3), (random() * 12)::numeric(18,6), 'RLX-CCF-1', 'seed', 'day_mean'
FROM (SELECT ('00000000-0000-4000-8000-' || lpad(n::text, 12, '0'))::uuid AS id, n, CASE WHEN n = 1 THEN 20 ELSE 5 END AS np, CASE WHEN n = 1 THEN 12 ELSE 6 END AS ng FROM generate_series(1, 21) n) t
CROSS JOIN LATERAL generate_series(1, t.np) p
CROSS JOIN LATERAL generate_series(0, t.ng - 1) g
CROSS JOIN LATERAL generate_series(0, 7) s
CROSS JOIN LATERAL generate_series('2025-10-13'::timestamptz, '2026-10-12'::timestamptz, '1 day') d
ON CONFLICT DO NOTHING;
ANALYZE;
SELECT 'usage', count(*) FROM usage.usage_records UNION ALL SELECT 'carbon', count(*) FROM carbon.calculations;
