# Reliabilix GreenOps Platform

Multi-tenant FinOps + GreenOps SaaS: cloud **cost and carbon in one view**.
Modular monolith (Go 1.27) + Next.js 16 frontend. Architecture decisions: `docs/adr/`.

```
backend/   Go: cmd/{api,worker,scheduler}, internal/{platform,<12 domains>,dashboard,app}, migrations/, api/openapi.yaml, tests/
frontend/  Next.js 16 · React 19 · TS · Tailwind 4 · TanStack Query · Zustand · RHF+Zod · ECharts · Auth0
deploy/    docker/ (Dockerfile, Dockerfile.web, initdb), dev/ (seeds)
docs/      architecture/, security/, adr/
```

## Run everything

```bash
docker compose up --build        # postgres, redis, migrate, seed-dev, api, worker, scheduler, web
open http://localhost:3000       # dev login (no Auth0 needed); API on :8080
```

The dev tenant starts **empty**: every Overview widget shows its honest empty state
("Connect a cloud account to see data"). To see the populated screen, load synthetic data
(clearly labelled demo rows in the *database*; the frontend contains no mock data):

```bash
make seed-demo      # needs psql; one AWS account, 60 days of usage/carbon, grid intensity, audit events
```

### Without Docker
```bash
docker compose up -d postgres redis && make migrate && make seed
make run-api        # :8080   (ENV=dev -> dev tokens)
make web            # :3000   (copy frontend/.env.example to frontend/.env.local)
```

### Real Auth0
Set `AUTH0_*` (see `frontend/.env.example`), `ENV=prod`, and an Auth0 Action that adds
`https://reliabilix.com/tenant_id` and `https://reliabilix.com/role` claims to the access token.
With `ENV!=dev` the API only accepts RS256 tokens verified against your Auth0 JWKS (iss/aud/exp/nbf).

## Quality gates
`make test` · `make test-integration` (real PostgreSQL: RLS on every tenant table, tenant isolation,
least-privilege roles, dashboard queries) · `make lint` (gosec + module-boundary rules) · `make vuln` · `make web-test`.

## Status (honest)
| Area | State |
|---|---|
| Platform (config, http hardening, Auth0 JWKS verifier, RBAC, RLS tx helper, Asynq, OTel/Prometheus, storage iface, audit) | done, tested |
| Schemas, RLS, roles, partitioning | done, verified on PostgreSQL 16 |
| projects, cloudaccounts (list/create), usage, carbon (calc/summary/trend/SCI), finops summary, dashboard read models, audit-log | done |
| Grid intensity (Electricity Maps) hourly job -> `carbon.grid_intensity` | done (needs API key; dev uses a static provider) |
| **AWS connector / sync / normalization -> usage & carbon rows** | **not implemented**: the pipeline that fills the tables is the next Phase 1 task; until then data comes only from `make seed-demo`. Connecting a real AWS account currently fails with "could not verify cloud access". |
| recommendations, reports, kubernetes, automation | Phase 2/3: empty modules; UI shows empty states |
| Frontend: Overview + all routes | done; Auth0 login code written but **not exercised against a live Auth0 tenant** |
