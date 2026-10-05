# Architecture overview

```
Browser -> Next.js -> Gateway -> { tenant, project, cloudintegration, ingestion, carbon }
                                      |  gRPC (sync, internal)   |  NATS JetStream (async events)
                                      v                          v
                                 PostgreSQL (db per service)   S3 (raw billing, reports)   Redis (cache, limits)
```

Data flow: `cloud.sync_completed -> ingestion -> usage.normalized -> carbon -> carbon.calculated -> recommendation/reporting`.

## Services

MVP (scaffolded): gateway, tenant, project, cloudintegration, ingestion, carbon.
Next: finops, recommendation (OPEN -> APPROVED -> APPLIED, human approval), reporting (PDF/CSV to S3), automation (policy + approval + rollback).

## Rules

1. A service owns its database; no cross-service SQL. Sync calls via gRPC, state changes via events.
2. Services never import each other (Go `internal/` + CI lint). Shared code lives only in `pkg/` and must stay domain-neutral.
3. `domain/` is pure: no HTTP, SQL, SDK or NATS imports (depguard).
4. Every tenant-scoped table has `tenant_id` + RLS; every query runs through `pkg/db.WithTenantTx`.
5. Tenant id comes from the verified token, never from request input.
6. Every calculation stores `methodology_version`.
7. Providers (cloud, grid carbon, k8s energy) are plugins behind interfaces.

## Changes vs. the original proposal

| Original | Here | Why |
|---|---|---|
| Chi router | stdlib `net/http` ServeMux | Go 1.22+ supports method/path patterns; no dependency needed, one less supply-chain surface |
| Separate Identity service | IdP (Zitadel/Keycloak/Auth0) + tenant service for memberships | Do not own passwords/MFA/OAuth; services only verify tokens |
| Gateway trusted by services | Services re-verify the token (zero trust) | A bypassed gateway must not bypass authz |
| Redis Streams / Asynq for events | NATS JetStream (Redis only for cache/limits) | One bus; durable, replayable, per-consumer ack |
| Repo with go.mod per service | Single Go module, many binaries | Simpler tooling at 6-10 services; split later if teams need it |
| 10 services at once | 6 MVP services | Avoid a distributed monolith before the domain is proven |
