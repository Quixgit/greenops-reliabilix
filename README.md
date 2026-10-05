# Reliabilix GreenOps

Multi-tenant FinOps + GreenOps platform: cloud cost and carbon footprint in one view.
Microservices-first, event-driven, API-first, security-first. Go 1.27 backend, Next.js 16 frontend.

## Layout

```
pkg/          shared platform libs (config, httpx, auth/RBAC, events, db, service bootstrap)
services/     one directory per bounded context (cmd/ + internal/{domain,application,infrastructure,transport} + migrations/)
  gateway/          public entry: authn, routing            :8080
  tenant/           tenants, memberships, audit log, /v1/me  :8081
  project/          projects                                 :8082
  cloudintegration/ AWS/Azure/GCP connectors (plugin iface)  :8083
  ingestion/        normalization -> usage_records           :8084
  carbon/           energy + CO2e engine, methodology ver.   :8085
proto/        internal gRPC contracts (buf)
events/       event catalog (NATS JetStream subjects)
frontend/     Next.js 16 + React 19 + Tailwind + shadcn/ui + ECharts
deploy/       Dockerfile, docker-compose (local infra), kubernetes/
docs/         architecture, security, ADRs
```

Later services (not scaffolded yet, see docs/architecture): finops, recommendation, reporting, automation.

## Quick start

```bash
make up                 # postgres, nats, redis, minio
make test               # go test -race ./...
make run-project        # ENV=dev, listens on :8082
curl -H 'Authorization: Bearer dev:alice:tenant-1:admin' localhost:8082/v1/projects
```

`ENV=dev` enables a dev-only token verifier (`dev:<sub>:<tenant>:<role>`). Outside dev, services refuse to
start until the OIDC verifier is configured (fail closed).

## Status

Implemented: shared HTTP/security baseline, RBAC matrix, tenant-isolation tests, carbon calculation with
methodology versioning, usage model validation, provider plugin interface, SQL schemas with Row Level Security.
Stubs / TODO: OIDC verifier, PostgreSQL adapters, NATS JetStream bus, AWS connector, gRPC wiring, OpenAPI,
OpenTelemetry, frontend screens.
