# Version manifest

The single place that says which version every part of Reliabilix is at. Rules for changing it are in
[`docs/release/versioning.md`](docs/release/versioning.md); what changed is in [`CHANGELOG.md`](CHANGELOG.md).
A test (`backend/internal/platform/version`) fails the build when this file, `VERSION`, `frontend/package.json`,
`backend/api/openapi.yaml` and the migrations disagree, or when a service or module has no row.

**Platform release: `0.1.0-alpha.14`** · stage **alpha** · shown in the UI as `v0.1.0` + alpha badge.

Row format (the test reads it): `` | `key` | version | ... | ``.

## Platform

| Item | Version | Notes |
|---|---|---|
| `platform` | 0.1.0-alpha.14 | Same as the `VERSION` file and `frontend/package.json`. |

## Services and container images

Every service has its own version and image. While the platform is in alpha all of them move together; once the
platform is stable they are bumped independently (a service version changes only when that service changes).
Image name: `<registry>/reliabilix/<service>:<version>`. Never deploy `latest` (see the policy).

| Item | Version | Image | Built from |
|---|---|---|---|
| `service/api` | 0.1.0-alpha.14 | `reliabilix/api` | `backend/cmd/api`, `deploy/docker/Dockerfile` (CMD=api) |
| `service/worker` | 0.1.0-alpha.14 | `reliabilix/worker` | `backend/cmd/worker` |
| `service/scheduler` | 0.1.0-alpha.14 | `reliabilix/scheduler` | `backend/cmd/scheduler` |
| `service/migrate` | 0.1.0-alpha.14 | `reliabilix/migrate` | `backend/migrations` + goose (target `migrate`) |
| `service/web` | 0.1.0-alpha.14 | `reliabilix/web` | `frontend/`, `deploy/docker/Dockerfile.web` |
| `service/admin` | 0.1.0-alpha.14 | (not an image: run from the api image or `go run`) | `backend/cmd/admin` |

## Backend domain modules

Versioned by the behaviour they expose (their HTTP API, their ports, their data). A module's version is bumped when
that module changes in a way listed in the policy.

| Item | Version | Notes |
|---|---|---|
| `internal/app` | 0.1.0-alpha.14 | Composition root, router, gates |
| `internal/alerts` | 0.1.0-alpha.14 | Budget and connection alerts |
| `internal/automation` | 0.1.0-alpha.14 | Plans only, no executors |
| `internal/carbon` | 0.1.0-alpha.14 | Engine, SCI, methodology use |
| `internal/cloudaccounts` | 0.1.0-alpha.14 | Connections, sync, reconciliation |
| `internal/dashboard` | 0.1.0-alpha.14 | Read models |
| `internal/finops` | 0.1.0-alpha.14 | Spend, budgets, anomalies |
| `internal/ingestion` | 0.1.0-alpha.14 | Normalizers to FOCUS |
| `internal/lifecycle` | 0.1.0-alpha.14 | Export, scheduled erasure, retention |
| `internal/kubernetes` | 0.1.0-alpha.14 | Placeholder layers |
| `internal/notifications` | 0.1.0-alpha.14 | Channels, outbox |
| `internal/operator` | 0.1.0-alpha.14 | Operator console |
| `internal/projects` | 0.1.0-alpha.14 | Projects, policies |
| `internal/recommendations` | 0.1.0-alpha.14 | region_shift, rightsizing |
| `internal/reports` | 0.1.0-alpha.14 | CSV, JSON, PDF |
| `internal/settings` | 0.1.0-alpha.14 | Settings engine |
| `internal/tenants` | 0.1.0-alpha.14 | Organizations, members, keys |
| `internal/usage` | 0.1.0-alpha.14 | FOCUS records |
| `internal/users` | 0.1.0-alpha.14 | Identity |

## Platform packages

| Item | Version | Notes |
|---|---|---|
| `platform/audit` | 0.1.0-alpha.14 | |
| `platform/auth` | 0.1.0-alpha.14 | OIDC, API keys, RBAC |
| `platform/config` | 0.1.0-alpha.14 | |
| `platform/database` | 0.1.0-alpha.14 | |
| `platform/ec2spec` | 0.1.0-alpha.14 | |
| `platform/entitlements` | 0.1.0-alpha.14 | Limits port |
| `platform/errlog` | 0.1.0-alpha.14 | Error log |
| `platform/focus` | 0.1.0-alpha.14 | FOCUS record contract |
| `platform/integrations` | 0.1.0-alpha.14 | External-service credentials |
| `platform/http` | 0.1.0-alpha.14 | |
| `platform/methodology` | 0.1.0-alpha.14 | |
| `platform/notify` | 0.1.0-alpha.14 | Producer port, event catalog |
| `platform/observability` | 0.1.0-alpha.14 | |
| `platform/queue` | 0.1.0-alpha.14 | |
| `platform/secretbox` | 0.1.0-alpha.14 | |
| `platform/settings` | 0.1.0-alpha.14 | Settings catalog |
| `platform/storage` | 0.1.0-alpha.14 | |
| `platform/version` | 0.1.0-alpha.14 | |

## Connectors and plugins

Each connector is versioned on its own, because a connector is what talks to a third party and is what changes when
that third party changes.

| Item | Version | Notes |
|---|---|---|
| `connector/aws` | 0.1.0-alpha.14 | Cost Explorer, FOCUS export, rightsizing |
| `connector/gcp` | 0.1.0-alpha.14 | Billing export in BigQuery |
| `connector/electricitymaps` | 0.1.0-alpha.14 | Grid carbon intensity |
| `channel/email` | 0.1.0-alpha.14 | SMTP |
| `channel/webhook` | 0.1.0-alpha.14 | Signed JSON |
| `channel/slack` | 0.1.0-alpha.14 | Incoming webhook |

## Contracts and data

These are what other systems and stored data depend on; they follow stricter rules (see the policy).

| Item | Version | Notes |
|---|---|---|
| `contract/api` | 0.7.0 | `backend/api/openapi.yaml` `info.version`; paths stay under `/api/v1` |
| `contract/webhook-payload` | 1 | Notification webhook body (`version` field) |
| `contract/webhook-signature` | 1 | `X-Reliabilix-Signature: t=,v1=` |
| `contract/focus` | 1.4 | FOCUS specification followed by `platform/focus` |
| `data/db-schema` | 23 | Number of the newest migration (`backend/migrations`) |
| `data/methodology` | RLX-PROVISIONAL-1 | Immutable; a new set is a new version, never an edit |
| `data/settings-catalog` | 1 | Bump when a setting is removed or its meaning changes |

## Third-party images (pinned)

| Item | Version | Notes |
|---|---|---|
| `base/postgres` | 18 | `postgres:18` |
| `base/redis` | 7 | `redis:7-alpine` |
| `base/go` | 1.27 | build image `golang:1.27`; runtime `distroless/static-debian12:nonroot` |
| `base/node` | 22 | `node:22-alpine` |
