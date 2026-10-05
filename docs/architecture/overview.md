# Architecture overview (modular monolith)

```
Browser -> Next.js (UI, httpOnly session, /api/proxy) -> Go API (chi) -> PostgreSQL 18 (schema per domain, RLS)
                                                          |                  ^
                                                          +-> Redis (Asynq) -+- cmd/worker  -> AWS (STS+Cost Explorer), Electricity Maps, S3
                                                                              +- cmd/scheduler (cron -> jobs)
```

One repository, one Go module, three long-running processes (`api`, `worker`, `scheduler`) and an `admin` CLI.

## The Phase 1 pipeline (all jobs carry tenant_id; every step is idempotent)

```
scheduler: cloudaccounts:sync_all (6h) ---> one cloudaccounts:sync_aws_account per connection
   sync_aws_account:  STS AssumeRole(ExternalId) -> Cost Explorer (daily, SERVICE x REGION) -> raw payload archived to S3
                      -> ingestion normalizes to FOCUS -> usage.usage_records (upsert) -> connection.synced_through
        -> carbon:calculate (touched days):  usage_based or cost_based energy x grid intensity -> carbon.calculations,
                                             SCI where functional units are reported
              -> recommendations:calculate:  region_shift into allowed regions only -> recommendations (open)
scheduler also: ensure_partitions (daily), refresh_grid_intensity (hourly), carbon:recalculate_all (daily, last 35 days),
                recommendations:calculate_all (daily)
```

A failed sync is retried; "access denied" marks the connection `error` with a safe message and is not retried.
Re-running any step replaces rows keyed by natural keys, so redelivery never duplicates data.

## Modules (internal/)

| Module | Owns |
|---|---|
| `tenants` | organizations, memberships, invitations, API keys, onboarding, audit-log view, identity resolution |
| `users` | `GET /me` |
| `projects` | projects, SCI functional units, policies (data-residency allow-list, CI thresholds) |
| `cloudaccounts` | connections, ExternalId, verify, sync runs; AWS provider (Azure/GCP: phase 2) |
| `ingestion` | raw -> FOCUS normalizers (AWS Cost Explorer now), raw archive, validation |
| `usage` | FOCUS usage storage (partitioned, idempotent upsert), listing, daily aggregates |
| `carbon` | versioned methodology use, energy/CO2e engine, SCI, grid intensity (Electricity Maps), CI gate |
| `finops` | spend summary, budgets with state, anomalies |
| `recommendations` | region_shift generator, workflow open -> approved -> applied, compliance re-check |
| `reports` | carbon / SCI / cost reports as CSV, JSON, PDF in object storage |
| `dashboard` | read models of the Overview screen (ADR-0012) |
| `kubernetes`, `automation` | phase 2 / phase 3: layers in place, no logic (Plan.CanExecute already refuses without approval and rollback) |
| `platform/*` | config, http hardening, auth, RBAC, database (tenant tx), queue, observability, storage, audit, focus, methodology |
| `app` | composition root: builds modules, wires ports, builds the router |

Rules (enforced): domains never import each other; `domain/` has no HTTP/SQL/queue/SDK imports; every tenant query runs in
`database.WithTenantTx`; the tenant comes from the verified identity; every calculation stores `methodology_version`.

## Roles

| Role | Can |
|---|---|
| owner | everything, including approving recommendations |
| admin | people, connections, projects, settings, budgets, audit; **not** approving infrastructure changes |
| engineer | read all, compute carbon, approve/apply/dismiss recommendations, reports, CI gate |
| viewer | read |
| billing | usage, carbon, cost, budgets, reports |
| ci (API keys only) | `POST /ci/evaluate` and nothing else |

## Honest limitations (Phase 1)

- AWS is the only connector and uses Cost Explorer, which has no resource or usage granularity: carbon is **cost_based**
  (low confidence) until the CUR/FOCUS export path lands. The coefficient set is **provisional** (`GET /carbon/methodology`).
- Cost-based estimates need USD; other currencies are skipped (FX normalization is phase 2). Global services have no grid and are skipped.
- Cost impact of region_shift is `null` (not estimated) unless `finops.region_price_index` is populated.
- Only region_shift is generated; rightsizing, time_shift and spot need utilization or forecast data and are not implemented.
- Invitations return a token; there is no email delivery. Azure, GCP, Kepler, WattTime, Stripe, Terraform automation and white-label are later phases.
- The Auth0 integration and the AWS provider are tested against fakes and a fake S3, not against live services.
