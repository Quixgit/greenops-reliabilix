# Version manifest

The single place that says which version every part of Reliabilix is at. Rules for changing it are in
[`docs/release/versioning.md`](docs/release/versioning.md); what changed is in [`CHANGELOG.md`](CHANGELOG.md).
A test (`backend/internal/platform/version`) fails the build when this file, `VERSION`, `frontend/package.json`,
`backend/api/openapi.yaml` and the migrations disagree, or when a service or module has no row.

**Platform release: `0.1.0-alpha.167`** · stage **alpha** · shown in the UI as `v0.1.0` + alpha badge.

Row format (the test reads it): `` | `key` | version | ... | ``.

## Platform

| Item | Version | Notes |
|---|---|---|
| `platform` | 0.1.0-alpha.167 | Same as the `VERSION` file and `frontend/package.json`. |

## Services and container images

Every service has its own version and image. While the platform is in alpha all of them move together; once the
platform is stable they are bumped independently (a service version changes only when that service changes).
Image name: `<registry>/reliabilix/<service>:<version>`. Never deploy `latest` (see the policy).

| Item | Version | Image | Built from |
|---|---|---|---|
| `service/api` | 0.1.0-alpha.167 | `reliabilix/api` | `backend/cmd/api`, `deploy/docker/Dockerfile` (CMD=api) |
| `service/worker` | 0.1.0-alpha.167 | `reliabilix/worker` | `backend/cmd/worker` |
| `service/scheduler` | 0.1.0-alpha.167 | `reliabilix/scheduler` | `backend/cmd/scheduler` |
| `service/migrate` | 0.1.0-alpha.167 | `reliabilix/migrate` | `backend/migrations` + goose (target `migrate`) |
| `service/web` | 0.1.0-alpha.167 | `reliabilix/web` | `frontend/`, `deploy/docker/Dockerfile.web` |
| `service/agent` | 0.1.0-alpha.167 | `reliabilix/agent` | `scripts/kepler-push.sh`, `deploy/docker/Dockerfile.agent` (runs in the customer's cluster) |
| `tool/terraform-provider` | 0.1.0-alpha.167 | `terraform-provider-reliabilix` | `terraform-provider-reliabilix/` (own Go module; not an image) |
| `service/admin` | 0.1.0-alpha.167 | (not an image: run from the api image or `go run`) | `backend/cmd/admin` |

## Backend domain modules

Versioned by the behaviour they expose (their HTTP API, their ports, their data). A module's version is bumped when
that module changes in a way listed in the policy.

| Item | Version | Notes |
|---|---|---|
| `internal/app` | 0.1.0-alpha.167 | Composition root, router, gates |
| `internal/alerts` | 0.1.0-alpha.167 | Budget and connection alerts |
| `internal/automation` | 0.1.0-alpha.167 | Plans only, no executors |
| `internal/carbon` | 0.1.0-alpha.167 | Engine, SCI, methodology use |
| `internal/cloudaccounts` | 0.1.0-alpha.167 | Connections, sync, reconciliation |
| `internal/dashboard` | 0.1.0-alpha.167 | Read models |
| `internal/finops` | 0.1.0-alpha.167 | Spend, budgets, anomalies |
| `internal/ingestion` | 0.1.0-alpha.167 | Normalizers to FOCUS |
| `internal/licensing` | 0.1.0-alpha.167 | Plan view, checkout, payment webhooks, subscription expiry |
| `internal/lifecycle` | 0.1.0-alpha.167 | Export, scheduled erasure, retention |
| `internal/kubernetes` | 0.1.0-alpha.167 | Placeholder layers |
| `internal/notifications` | 0.1.0-alpha.167 | Channels, outbox |
| `internal/operator` | 0.1.0-alpha.167 | Operator console |
| `internal/projects` | 0.1.0-alpha.167 | Projects, policies |
| `internal/recommendations` | 0.1.0-alpha.167 | region_shift, rightsizing |
| `internal/reports` | 0.1.0-alpha.167 | CSV, JSON, PDF |
| `internal/settings` | 0.1.0-alpha.167 | Settings engine |
| `internal/tenants` | 0.1.0-alpha.167 | Organizations, members, keys |
| `internal/usage` | 0.1.0-alpha.167 | FOCUS records |
| `internal/users` | 0.1.0-alpha.167 | Identity |

## Platform packages

| Item | Version | Notes |
|---|---|---|
| `platform/audit` | 0.1.0-alpha.167 | |
| `platform/apiusage` | 0.1.0-alpha.167 | Request metering and daily cap for paid APIs |
| `platform/auth` | 0.1.0-alpha.167 | OIDC, API keys, RBAC |
| `platform/config` | 0.1.0-alpha.167 | |
| `platform/database` | 0.1.0-alpha.167 | |
| `platform/ec2spec` | 0.1.0-alpha.167 | |
| `platform/awsprice` | 0.1.0-alpha.167 | Relative price indexes per region: EC2 (pricing pages), RDS and ElastiCache (bulk price list) |
| `platform/azureprice` | 0.1.0-alpha.167 | Azure Retail Prices: relative price index per region for VMs, managed PostgreSQL and Redis |
| `platform/gcpprice` | 0.1.0-alpha.167 | Cloud Billing Catalog: relative Compute Engine, Cloud SQL and Memorystore price indexes per region (needs a key) |
| `platform/pricing` | 0.1.0-alpha.167 | What the price-index clients share: the "not offered in this region" answer |
| `platform/fxrates` | 0.1.0-alpha.167 | ECB euro reference rates for currency conversion |
| `platform/entitlements` | 0.1.0-alpha.167 | Limits port |
| `platform/errlog` | 0.1.0-alpha.167 | Error log |
| `platform/geo` | 0.1.0-alpha.167 | Where cloud regions are; measured (Microsoft, CloudPing) or distance-estimated round-trip time between them, with its basis |
| `platform/focus` | 0.1.0-alpha.167 | FOCUS record contract |
| `platform/integrations` | 0.1.0-alpha.167 | External-service credentials |
| `platform/billing` | 0.1.0-alpha.167 | Payment provider port: Stripe, Paddle, LiqPay, WayForPay, invoice (ADR-0038) |
| `platform/http` | 0.1.0-alpha.167 | |
| `platform/i18n` | 0.1.0-alpha.167 | Server-side language catalogs and negotiation (ADR-0036) |
| `platform/methodology` | 0.1.0-alpha.167 | |
| `platform/notify` | 0.1.0-alpha.167 | Producer port, event catalog |
| `platform/observability` | 0.1.0-alpha.167 | |
| `platform/plans` | 0.1.0-alpha.167 | Plan catalog: presets of limits and modules (ADR-0033) |
| `platform/queue` | 0.1.0-alpha.167 | |
| `platform/secretbox` | 0.1.0-alpha.167 | |
| `platform/settings` | 0.1.0-alpha.167 | Settings catalog |
| `platform/storage` | 0.1.0-alpha.167 | |
| `platform/version` | 0.1.0-alpha.167 | |

## Connectors and plugins

Each connector is versioned on its own, because a connector is what talks to a third party and is what changes when
that third party changes.

| Item | Version | Notes |
|---|---|---|
| `connector/aws` | 0.1.0-alpha.167 | Cost Explorer, FOCUS export, rightsizing |
| `connector/gcp` | 0.1.0-alpha.167 | Billing export in BigQuery |
| `connector/azure` | 0.1.0-alpha.167 | Cost Management export in Blob Storage (FOCUS) |
| `connector/electricitymaps` | 0.1.0-alpha.167 | Grid carbon intensity |
| `connector/watttime` | 0.1.0-alpha.167 | Marginal intensity client (ADR-0049); written, not connected to ingestion |
| `channel/email` | 0.1.0-alpha.167 | SMTP |
| `channel/webhook` | 0.1.0-alpha.167 | Signed JSON |
| `channel/slack` | 0.1.0-alpha.167 | Incoming webhook |

## Contracts and data

These are what other systems and stored data depend on; they follow stricter rules (see the policy).

| Item | Version | Notes |
|---|---|---|
| `contract/api` | 0.45.0 | `backend/api/openapi.yaml` `info.version`; paths stay under `/api/v1` |
| `contract/webhook-payload` | 1 | Notification webhook body (`version` field) |
| `contract/webhook-signature` | 1 | `X-Reliabilix-Signature: t=,v1=` |
| `contract/focus` | 1.4 | FOCUS specification followed by `platform/focus` |
| `data/db-schema` | 74 | Number of the newest migration (`backend/migrations`) |
| `data/methodology` | RLX-PROVISIONAL-1 | Immutable; a new set is a new version, never an edit |
| `data/settings-catalog` | 1 | Bump when a setting is removed or its meaning changes |

## Third-party images (pinned)

| Item | Version | Notes |
|---|---|---|
| `base/postgres` | 18 | `postgres:18` |
| `base/redis` | 7 | `redis:7-alpine` |
| `base/go` | 1.27 | build image `golang:1.27`; runtime `distroless/static-debian12:nonroot` |
| `base/node` | 22 | `node:22-alpine` |
