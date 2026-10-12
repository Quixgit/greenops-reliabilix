# Version manifest

The single place that says which version every part of Reliabilix is at. Rules for changing it are in
[`docs/release/versioning.md`](docs/release/versioning.md); what changed is in [`CHANGELOG.md`](CHANGELOG.md).
A test (`backend/internal/platform/version`) fails the build when this file, `VERSION`, `frontend/package.json`,
`backend/api/openapi.yaml` and the migrations disagree, or when a service or module has no row.

**Platform release: `0.1.0-alpha.174`** · stage **alpha** · shown in the UI as `v0.1.0` + alpha badge.

Row format (the test reads it): `` | `key` | version | ... | ``.

## Platform

| Item | Version | Notes |
|---|---|---|
| `platform` | 0.1.0-alpha.174 | Same as the `VERSION` file and `frontend/package.json`. |

## Services and container images

Every service has its own version and image. While the platform is in alpha all of them move together; once the
platform is stable they are bumped independently (a service version changes only when that service changes).
Image name: `<registry>/reliabilix/<service>:<version>`. Never deploy `latest` (see the policy).

| Item | Version | Image | Built from |
|---|---|---|---|
| `service/api` | 0.1.0-alpha.174 | `reliabilix/api` | `backend/cmd/api`, `deploy/docker/Dockerfile` (CMD=api) |
| `service/worker` | 0.1.0-alpha.174 | `reliabilix/worker` | `backend/cmd/worker` |
| `service/scheduler` | 0.1.0-alpha.174 | `reliabilix/scheduler` | `backend/cmd/scheduler` |
| `service/migrate` | 0.1.0-alpha.174 | `reliabilix/migrate` | `backend/migrations` + goose (target `migrate`) |
| `service/web` | 0.1.0-alpha.174 | `reliabilix/web` | `frontend/`, `deploy/docker/Dockerfile.web` |
| `service/agent` | 0.1.0-alpha.174 | `reliabilix/agent` | `scripts/kepler-push.sh`, `deploy/docker/Dockerfile.agent` (runs in the customer's cluster) |
| `tool/terraform-provider` | 0.1.0-alpha.174 | `terraform-provider-reliabilix` | `terraform-provider-reliabilix/` (own Go module; not an image) |
| `service/admin` | 0.1.0-alpha.174 | (not an image: run from the api image or `go run`) | `backend/cmd/admin` |

## Backend domain modules

Versioned by the behaviour they expose (their HTTP API, their ports, their data). A module's version is bumped when
that module changes in a way listed in the policy.

| Item | Version | Notes |
|---|---|---|
| `internal/app` | 0.1.0-alpha.174 | Composition root, router, gates |
| `internal/alerts` | 0.1.0-alpha.174 | Budget and connection alerts |
| `internal/automation` | 0.1.0-alpha.174 | Plans only, no executors |
| `internal/carbon` | 0.1.0-alpha.174 | Engine, SCI, methodology use |
| `internal/cloudaccounts` | 0.1.0-alpha.174 | Connections, sync, reconciliation |
| `internal/dashboard` | 0.1.0-alpha.174 | Read models |
| `internal/finops` | 0.1.0-alpha.174 | Spend, budgets, anomalies |
| `internal/ingestion` | 0.1.0-alpha.174 | Normalizers to FOCUS |
| `internal/licensing` | 0.1.0-alpha.174 | Plan view, checkout, payment webhooks, subscription expiry |
| `internal/lifecycle` | 0.1.0-alpha.174 | Export, scheduled erasure, retention |
| `internal/kubernetes` | 0.1.0-alpha.174 | Placeholder layers |
| `internal/notifications` | 0.1.0-alpha.174 | Channels, outbox |
| `internal/operator` | 0.1.0-alpha.174 | Operator console |
| `internal/projects` | 0.1.0-alpha.174 | Projects, policies |
| `internal/recommendations` | 0.1.0-alpha.174 | region_shift, rightsizing |
| `internal/reports` | 0.1.0-alpha.174 | CSV, JSON, PDF |
| `internal/settings` | 0.1.0-alpha.174 | Settings engine |
| `internal/tenants` | 0.1.0-alpha.174 | Organizations, members, keys |
| `internal/usage` | 0.1.0-alpha.174 | FOCUS records |
| `internal/users` | 0.1.0-alpha.174 | Identity |

## Platform packages

| Item | Version | Notes |
|---|---|---|
| `platform/audit` | 0.1.0-alpha.174 | |
| `platform/apiusage` | 0.1.0-alpha.174 | Request metering and daily cap for paid APIs |
| `platform/auth` | 0.1.0-alpha.174 | OIDC, API keys, RBAC |
| `platform/config` | 0.1.0-alpha.174 | |
| `platform/database` | 0.1.0-alpha.174 | |
| `platform/ec2spec` | 0.1.0-alpha.174 | |
| `platform/awsprice` | 0.1.0-alpha.174 | Relative price indexes per region: EC2 (pricing pages), RDS and ElastiCache (bulk price list) |
| `platform/azureprice` | 0.1.0-alpha.174 | Azure Retail Prices: relative price index per region for VMs, managed PostgreSQL and Redis |
| `platform/gcpprice` | 0.1.0-alpha.174 | Cloud Billing Catalog: relative Compute Engine, Cloud SQL and Memorystore price indexes per region (needs a key) |
| `platform/readcache` | 0.1.0-alpha.174 | A few seconds of memory and one shared run for the heavy dashboard answers (per organization) |
| `platform/pricing` | 0.1.0-alpha.174 | What the price-index clients share: the "not offered in this region" answer |
| `platform/fxrates` | 0.1.0-alpha.174 | ECB euro reference rates for currency conversion |
| `platform/entitlements` | 0.1.0-alpha.174 | Limits port |
| `platform/errlog` | 0.1.0-alpha.174 | Error log |
| `platform/geo` | 0.1.0-alpha.174 | Where cloud regions are; measured (Microsoft, CloudPing) or distance-estimated round-trip time between them, with its basis |
| `platform/focus` | 0.1.0-alpha.174 | FOCUS record contract |
| `platform/integrations` | 0.1.0-alpha.174 | External-service credentials |
| `platform/billing` | 0.1.0-alpha.174 | Payment provider port: Stripe, Paddle, LiqPay, WayForPay, invoice (ADR-0038) |
| `platform/http` | 0.1.0-alpha.174 | |
| `platform/i18n` | 0.1.0-alpha.174 | Server-side language catalogs and negotiation (ADR-0036) |
| `platform/methodology` | 0.1.0-alpha.174 | |
| `platform/notify` | 0.1.0-alpha.174 | Producer port, event catalog |
| `platform/observability` | 0.1.0-alpha.174 | |
| `platform/plans` | 0.1.0-alpha.174 | Plan catalog: presets of limits and modules (ADR-0033) |
| `platform/queue` | 0.1.0-alpha.174 | |
| `platform/secretbox` | 0.1.0-alpha.174 | |
| `platform/settings` | 0.1.0-alpha.174 | Settings catalog |
| `platform/storage` | 0.1.0-alpha.174 | |
| `platform/version` | 0.1.0-alpha.174 | |

## Connectors and plugins

Each connector is versioned on its own, because a connector is what talks to a third party and is what changes when
that third party changes.

| Item | Version | Notes |
|---|---|---|
| `connector/aws` | 0.1.0-alpha.174 | Cost Explorer, FOCUS export, rightsizing |
| `connector/gcp` | 0.1.0-alpha.174 | Billing export in BigQuery |
| `connector/azure` | 0.1.0-alpha.174 | Cost Management export in Blob Storage (FOCUS) |
| `connector/electricitymaps` | 0.1.0-alpha.174 | Grid carbon intensity |
| `connector/watttime` | 0.1.0-alpha.174 | Marginal intensity client (ADR-0049); written, not connected to ingestion |
| `channel/email` | 0.1.0-alpha.174 | SMTP |
| `channel/webhook` | 0.1.0-alpha.174 | Signed JSON |
| `channel/slack` | 0.1.0-alpha.174 | Incoming webhook |

## Contracts and data

These are what other systems and stored data depend on; they follow stricter rules (see the policy).

| Item | Version | Notes |
|---|---|---|
| `contract/api` | 0.51.0 | `backend/api/openapi.yaml` `info.version`; paths stay under `/api/v1` |
| `contract/webhook-payload` | 1 | Notification webhook body (`version` field) |
| `contract/webhook-signature` | 1 | `X-Reliabilix-Signature: t=,v1=` |
| `contract/focus` | 1.4 | FOCUS specification followed by `platform/focus` |
| `data/db-schema` | 80 | Number of the newest migration (`backend/migrations`) |
| `data/methodology` | RLX-PROVISIONAL-1 | Immutable; a new set is a new version, never an edit |
| `data/settings-catalog` | 1 | Bump when a setting is removed or its meaning changes |

## Third-party images (pinned)

| Item | Version | Notes |
|---|---|---|
| `base/postgres` | 18 | `postgres:18` |
| `base/redis` | 7 | `redis:7-alpine` |
| `base/go` | 1.27 | build image `golang:1.27`; runtime `distroless/static-debian12:nonroot` |
| `base/node` | 22 | `node:22-alpine` |
