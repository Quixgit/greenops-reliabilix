# Version manifest

The single place that says which version every part of Reliabilix is at. Rules for changing it are in
[`docs/release/versioning.md`](docs/release/versioning.md); what changed is in [`CHANGELOG.md`](CHANGELOG.md).
A test (`backend/internal/platform/version`) fails the build when this file, `VERSION`, `frontend/package.json`,
`backend/api/openapi.yaml` and the migrations disagree, or when a service or module has no row.

**Platform release: `0.1.0-alpha.141`** · stage **alpha** · shown in the UI as `v0.1.0` + alpha badge.

Row format (the test reads it): `` | `key` | version | ... | ``.

## Platform

| Item | Version | Notes |
|---|---|---|
| `platform` | 0.1.0-alpha.141 | Same as the `VERSION` file and `frontend/package.json`. |

## Services and container images

Every service has its own version and image. While the platform is in alpha all of them move together; once the
platform is stable they are bumped independently (a service version changes only when that service changes).
Image name: `<registry>/reliabilix/<service>:<version>`. Never deploy `latest` (see the policy).

| Item | Version | Image | Built from |
|---|---|---|---|
| `service/api` | 0.1.0-alpha.141 | `reliabilix/api` | `backend/cmd/api`, `deploy/docker/Dockerfile` (CMD=api) |
| `service/worker` | 0.1.0-alpha.141 | `reliabilix/worker` | `backend/cmd/worker` |
| `service/scheduler` | 0.1.0-alpha.141 | `reliabilix/scheduler` | `backend/cmd/scheduler` |
| `service/migrate` | 0.1.0-alpha.141 | `reliabilix/migrate` | `backend/migrations` + goose (target `migrate`) |
| `service/web` | 0.1.0-alpha.141 | `reliabilix/web` | `frontend/`, `deploy/docker/Dockerfile.web` |
| `service/agent` | 0.1.0-alpha.141 | `reliabilix/agent` | `scripts/kepler-push.sh`, `deploy/docker/Dockerfile.agent` (runs in the customer's cluster) |
| `service/admin` | 0.1.0-alpha.141 | (not an image: run from the api image or `go run`) | `backend/cmd/admin` |

## Backend domain modules

Versioned by the behaviour they expose (their HTTP API, their ports, their data). A module's version is bumped when
that module changes in a way listed in the policy.

| Item | Version | Notes |
|---|---|---|
| `internal/app` | 0.1.0-alpha.141 | Composition root, router, gates |
| `internal/alerts` | 0.1.0-alpha.141 | Budget and connection alerts |
| `internal/automation` | 0.1.0-alpha.141 | Plans only, no executors |
| `internal/carbon` | 0.1.0-alpha.141 | Engine, SCI, methodology use |
| `internal/cloudaccounts` | 0.1.0-alpha.141 | Connections, sync, reconciliation |
| `internal/dashboard` | 0.1.0-alpha.141 | Read models |
| `internal/finops` | 0.1.0-alpha.141 | Spend, budgets, anomalies |
| `internal/ingestion` | 0.1.0-alpha.141 | Normalizers to FOCUS |
| `internal/licensing` | 0.1.0-alpha.141 | Plan view, checkout, payment webhooks, subscription expiry |
| `internal/lifecycle` | 0.1.0-alpha.141 | Export, scheduled erasure, retention |
| `internal/kubernetes` | 0.1.0-alpha.141 | Placeholder layers |
| `internal/notifications` | 0.1.0-alpha.141 | Channels, outbox |
| `internal/operator` | 0.1.0-alpha.141 | Operator console |
| `internal/projects` | 0.1.0-alpha.141 | Projects, policies |
| `internal/recommendations` | 0.1.0-alpha.141 | region_shift, rightsizing |
| `internal/reports` | 0.1.0-alpha.141 | CSV, JSON, PDF |
| `internal/settings` | 0.1.0-alpha.141 | Settings engine |
| `internal/tenants` | 0.1.0-alpha.141 | Organizations, members, keys |
| `internal/usage` | 0.1.0-alpha.141 | FOCUS records |
| `internal/users` | 0.1.0-alpha.141 | Identity |

## Platform packages

| Item | Version | Notes |
|---|---|---|
| `platform/audit` | 0.1.0-alpha.141 | |
| `platform/apiusage` | 0.1.0-alpha.141 | Request metering and daily cap for paid APIs |
| `platform/auth` | 0.1.0-alpha.141 | OIDC, API keys, RBAC |
| `platform/config` | 0.1.0-alpha.141 | |
| `platform/database` | 0.1.0-alpha.141 | |
| `platform/ec2spec` | 0.1.0-alpha.141 | |
| `platform/awsprice` | 0.1.0-alpha.141 | Relative EC2 price index per region (AWS price list) |
| `platform/azureprice` | 0.1.0-alpha.141 | Azure Retail Prices: relative price index per region for VMs, managed PostgreSQL and Redis |
| `platform/gcpprice` | 0.1.0-alpha.141 | Cloud Billing Catalog: relative Compute Engine price index per region (needs a key) |
| `platform/fxrates` | 0.1.0-alpha.141 | ECB euro reference rates for currency conversion |
| `platform/entitlements` | 0.1.0-alpha.141 | Limits port |
| `platform/errlog` | 0.1.0-alpha.141 | Error log |
| `platform/geo` | 0.1.0-alpha.141 | Where cloud regions are; measured (Microsoft, CloudPing) or distance-estimated round-trip time between them, with its basis |
| `platform/focus` | 0.1.0-alpha.141 | FOCUS record contract |
| `platform/integrations` | 0.1.0-alpha.141 | External-service credentials |
| `platform/billing` | 0.1.0-alpha.141 | Payment provider port: Stripe, Paddle, LiqPay, WayForPay, invoice (ADR-0038) |
| `platform/http` | 0.1.0-alpha.141 | |
| `platform/i18n` | 0.1.0-alpha.141 | Server-side language catalogs and negotiation (ADR-0036) |
| `platform/methodology` | 0.1.0-alpha.141 | |
| `platform/notify` | 0.1.0-alpha.141 | Producer port, event catalog |
| `platform/observability` | 0.1.0-alpha.141 | |
| `platform/plans` | 0.1.0-alpha.141 | Plan catalog: presets of limits and modules (ADR-0033) |
| `platform/queue` | 0.1.0-alpha.141 | |
| `platform/secretbox` | 0.1.0-alpha.141 | |
| `platform/settings` | 0.1.0-alpha.141 | Settings catalog |
| `platform/storage` | 0.1.0-alpha.141 | |
| `platform/version` | 0.1.0-alpha.141 | |

## Connectors and plugins

Each connector is versioned on its own, because a connector is what talks to a third party and is what changes when
that third party changes.

| Item | Version | Notes |
|---|---|---|
| `connector/aws` | 0.1.0-alpha.141 | Cost Explorer, FOCUS export, rightsizing |
| `connector/gcp` | 0.1.0-alpha.141 | Billing export in BigQuery |
| `connector/azure` | 0.1.0-alpha.141 | Cost Management export in Blob Storage (FOCUS) |
| `connector/electricitymaps` | 0.1.0-alpha.141 | Grid carbon intensity |
| `channel/email` | 0.1.0-alpha.141 | SMTP |
| `channel/webhook` | 0.1.0-alpha.141 | Signed JSON |
| `channel/slack` | 0.1.0-alpha.141 | Incoming webhook |

## Contracts and data

These are what other systems and stored data depend on; they follow stricter rules (see the policy).

| Item | Version | Notes |
|---|---|---|
| `contract/api` | 0.39.0 | `backend/api/openapi.yaml` `info.version`; paths stay under `/api/v1` |
| `contract/webhook-payload` | 1 | Notification webhook body (`version` field) |
| `contract/webhook-signature` | 1 | `X-Reliabilix-Signature: t=,v1=` |
| `contract/focus` | 1.4 | FOCUS specification followed by `platform/focus` |
| `data/db-schema` | 66 | Number of the newest migration (`backend/migrations`) |
| `data/methodology` | RLX-PROVISIONAL-1 | Immutable; a new set is a new version, never an edit |
| `data/settings-catalog` | 1 | Bump when a setting is removed or its meaning changes |

## Third-party images (pinned)

| Item | Version | Notes |
|---|---|---|
| `base/postgres` | 18 | `postgres:18` |
| `base/redis` | 7 | `redis:7-alpine` |
| `base/go` | 1.27 | build image `golang:1.27`; runtime `distroless/static-debian12:nonroot` |
| `base/node` | 22 | `node:22-alpine` |
