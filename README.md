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
make seed-demo      # needs psql: one AWS account, 60 days of FOCUS usage, grid intensity, audit events
make recalc         # runs the real carbon engine + recommendations on it (no Redis needed)
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
`make test` (unit, incl. OpenAPI <-> routes parity) | `make test-integration` (real PostgreSQL + Redis: RLS on every tenant table, role privileges, the end-to-end pipeline through the real HTTP router, real Asynq) | `make lint` (gosec + module-isolation rules) | `make vuln` | `make web-test` | `make sqlc` / `make api-types` regenerate code (CI fails on drift).

## Backend status (honest)
| Area | State |
|---|---|
| Auth0 JWKS verifier, DB-resolved tenancy/roles, onboarding, invitations, API keys, RBAC, audit log | done, tested (Auth0 itself not exercised live) |
| Schema (FOCUS usage, partitions, RLS, least-privilege roles) | done, verified on PostgreSQL 16 |
| AWS connector: ExternalId flow, verify, STS AssumeRole + Cost Explorer sync, incremental windows, access-denied handling | done, tested with fakes (not against live AWS) |
| Ingestion -> FOCUS, usage, carbon engine (usage/cost based), SCI, grid intensity job, methodology versioning | done |
| Recommendations: region_shift, approval workflow, compliance re-check | done |
| Reports CSV/JSON/PDF to S3-compatible storage; finops summary, budgets, anomalies; CI gate; dashboard read models | done |
| OpenAPI (46 paths) + generated TS types, route parity test | done |
| Azure, GCP, Kubernetes (Kepler), WattTime, rightsizing/time-shift/spot, Terraform automation, Stripe, email delivery | later phases |
| Frontend | Overview + routes exist; screens for recommendations, reports, members, API keys are not built yet |
