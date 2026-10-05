# Architecture overview (modular monolith)

```
Browser -> Next.js (UI, session, /api/proxy) -> Go API (chi) -> PostgreSQL (schema per domain, RLS)
                                                  |-> Redis (Asynq queue, rate limits)
cmd/scheduler --cron--> Redis --> cmd/worker --> cloud APIs, Electricity Maps, S3
```
One repository, one Go module, three processes (`api`, `worker`, `scheduler`). Each business domain is a package
`internal/<domain>/{domain,application,repository,http}` + `service.go` exposing a `Module`
(`Routes(chi.Router)`, optionally `RegisterJobs(*asynq.ServeMux)`). Extraction rules: ADR-0005.

## Boundaries (enforced)
- `domain/` imports no HTTP, SQL, queue or cloud SDK (golangci depguard).
- A domain never imports another domain's `domain|application|repository|http|providers` (depguard).
- Tenant id always comes from the verified token, never from request input; every tenant query runs inside `database.WithTenantTx` (sets `app.tenant_id` for RLS).
- `dashboard` is the one deliberate exception: a read-only composition over usage/carbon/audit tables (ADR-0009).

## Overview screen: widget -> data source
| Widget | Endpoint | Source |
|---|---|---|
| Stat cards (CO2e, cost, intensity, SCI) + deltas | `GET /carbon/summary`, `GET /finops/summary` | `carbon.calculations`, `usage.usage_records` (current vs previous window of equal length) |
| Cost vs Carbon | `GET /dashboard/trend` | both tables joined per day |
| Carbon by provider | `GET /dashboard/providers` | `carbon.calculations.provider` (only providers with data) |
| Details by service | `GET /dashboard/services?category=` | usage (cost) + calculations (CO2e, daily sparkline); `service_category` follows FOCUS |
| Carbon intensity by region | `GET /dashboard/regions` | `carbon.grid_intensity` (hourly Electricity Maps job), 24h change |
| Recent activity | `GET /dashboard/activity` | `audit.audit_logs`, allowlisted user-facing actions only |
| Connected accounts | `GET /cloud-accounts` | `cloudaccounts.connections` |
| Top recommendations | none (phase 2) | honest empty state; `RecommendationCard` is built but unused |

Editable copy (promo texts, empty-state messages, tagline) lives in `frontend/content/overview.json`, validated by `content/schema.ts`.
SCI is carbon per functional unit, so a **falling** SCI is the green direction.

## Phase 1 deviations from the written spec (and why)
- Added `tenant_id` to `cloudaccounts.connections`, `carbon.calculations`, `recommendations.recommendations` (each table carries its own RLS policy).
- Added FOCUS-aligned `service_name`/`service_category` to usage, and denormalized `provider/region/service_*` to calculations.
- Added the read-only `dashboard` module and `GET /dashboard/*`.
- `finops/summary` requires `usage:read` (not `billing:read`) so viewers/engineers see the cost card; billing-specific data stays behind `billing:read`.
- `CloudProvider.GetUsage` returns provider-specific `RawUsage`; the ingestion domain normalizes it into `UsageRecord` (keeps cloudaccounts independent of ingestion).
