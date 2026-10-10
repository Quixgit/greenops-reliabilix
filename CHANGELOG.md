# Changelog

All notable changes. Format: [Keep a Changelog](https://keepachangelog.com), versions as in
[`docs/release/versioning.md`](docs/release/versioning.md). The version of every part is in [`VERSIONS.md`](VERSIONS.md).

## [Unreleased]

## [0.1.0-alpha.14] - 2026-10-10

### Added
- Kubernetes energy ([ADR-0030](docs/adr/0030-kubernetes-energy-agent.md)): a customer-side agent pushes measured workload
  energy (Kepler via Prometheus) with an API key of the new role `agent`; stored hourly per workload, carbon from the
  stored grid intensity and the active PUE (unknown stays unknown); Kubernetes page with clusters and workloads. It is
  attribution inside the cloud energy and is never added to totals. `POST /kubernetes/energy`, `GET /kubernetes/clusters`,
  `GET /kubernetes/workloads`; reference agent `scripts/kepler-push.sh` and `docs/setup/kubernetes.md`.
  API contract 0.7.0. Migration 0023.

## [0.1.0-alpha.13] - 2026-10-10

### Changed
- Sign-in page rebuilt: one calm form (user name and password in the test environment; Google, GitHub and e-mail with
  the identity provider) on real photographs: a defocused city of towers dissolving into a green forest, with a slow
  zoom and drifting mist. Photographs are CC0 and credited in `docs/credits.md`.

## [0.1.0-alpha.12] - 2026-10-10

### Changed
- Sign-in page redesigned as one full-screen industrial scene: dark control-room look with an engineering grid, cost and
  carbon telemetry traces, region nodes joined by flowing lines, server racks with status lights and instrument panels;
  the form sits in the middle on a light panel. Calm motion, switched off for people who prefer reduced motion.

## [0.1.0-alpha.11] - 2026-10-10

### Added
- Development sign-in on the product's own page (`DEV_LOGIN_PAGE=true` with `DEV_AUTH` and `DEV_BASIC_AUTH`): the
  browser's password prompt is replaced by the sign-in page; a signed http-only cookie carries the session; rate limited;
  absent outside the development mode.

### Fixed
- Sign out answered 404 in the development environment; `/auth/logout` now ends the session and returns to the sign-in page.

## [0.1.0-alpha.10] - 2026-10-10

### Added
- Operator console **Integrations** page ([ADR-0029](docs/adr/0029-integration-credentials-in-the-console.md)): the
  Electricity Maps key and zone overrides and the SMTP server, credentials and sender are set in the console, win over
  the environment, take effect without a restart, secrets are stored sealed and never shown, and each has a connection
  test. Fields fixed by the deployment are listed read-only with their state. API contract 0.6.0. Migration 0022.

### Changed
- The e-mail channel is now configured at run time: it is hidden (production) or only logs (development) until the
  server and the sender are set.

## [0.1.0-alpha.9] - 2026-10-10

### Added
- Sign-in page in the GreenOps style: landscape illustration, Google and GitHub buttons, e-mail entry that hands over
  to the identity provider (password and the two-step code are typed only there), password recovery form
  (`/api/auth/forgot-password`, same answer for any address, rate limited), responsive, with a clear development-mode
  notice. `/login?preview` shows it in the development environment.

## [0.1.0-alpha.8] - 2026-10-10

### Added
- Alert settings per organization: the warning level (`alerts.warn_pct`, default 80 %) and a monthly carbon limit
  (`alerts.carbon_monthly_kg`, default 0 = off) that opens a `carbon` alert for the calendar month so far
  ([ADR-0028](docs/adr/0028-alerts.md)). Migration 0021.

## [0.1.0-alpha.7] - 2026-10-10

### Added
- Alerts ([ADR-0028](docs/adr/0028-alerts.md)): stored alerts with a lifecycle (new, acknowledged, muted, resolved),
  opened and resolved by an hourly evaluation of budgets (80 % warning, 100 % critical) and connection health
  (errors, cost data that does not match the cloud); `alert.opened` notification; acknowledge/mute on the Alerts page.
  API contract 0.5.0. Migration 0020.

### Changed
- UI: softer card shadows and a fainter fading divider for a calmer, more uniform look.

## [0.1.0-alpha.6] - 2026-10-10

### Added
- Cloud account page (`/cloud/<id>`): connection state, the result of the last cost reconciliation with the cloud
  (matches / does not match / not checked, mismatched days), full sync history with per-run data quality, "Test
  connection" and "Sync now".

### Fixed
- `docker-compose.yml`: MinIO images come from `quay.io` (Docker Hub no longer serves `minio/minio`; the server image
  ships `mc`), so the local stack starts again.

## [0.1.0-alpha.5] - 2026-10-10

### Added
- Data lifecycle ([ADR-0027](docs/adr/0027-data-lifecycle.md)): owner-only export of all organization data (NDJSON,
  no secrets), erasure of an organization after a cancellable grace period (database in one transaction, files first,
  personal-data-free tombstone), and platform retention of usage and carbon history by whole months
  (`retention.data_months`, `lifecycle.deletion_grace_days`). Settings "Data" tab for owners.
- Object storage `DeletePrefix` (filesystem, memory, S3).

### Changed
- UI: white page background, slightly stronger card borders, a faint vertical rule that fades at both ends between the
  Overview content and its right column. The sidebar footer shows the full pre-release version (for example `alpha.5`),
  taken from the build.
- API contract 0.4.0 (export and deletion endpoints, additive).

## [0.1.0-alpha.4] - 2026-10-10

### Added
- Methodology registry: several immutable coefficient sets, one active; a test pins each file's checksum so a published
  set cannot be edited. `GET /carbon/methodology/versions`.
- Shadow recalculation under another version (`methodology_version` on `POST /carbon/recalculate`) and a read-only
  comparison of two versions over a window (`GET /carbon/compare`) ([ADR-0026](docs/adr/0026-methodology-registry-and-comparison.md)).

## [0.1.0-alpha.3] - 2026-10-10

### Added
- GCP cost data quality: every sync is reconciled with a second, ungrouped BigQuery query on the billing export
  ([ADR-0024](docs/adr/0024-cost-data-quality-reconciliation.md)). The AWS FOCUS export path stays unchecked on purpose.

## [0.1.0-alpha.2] - 2026-10-10

### Added
- Release workflow: a `v*` tag builds and pushes the five images with exact-version and build tags, never `latest`.
- Verified impact: an applied recommendation is measured (14 days before vs 14 days after, 3 days of billing lag
  skipped) and gets status `verified` with a separate result row; forecast and measurement are never mixed. New
  `verification` field on recommendations, `verified` status filter, `recommendation.verified` notification
  ([ADR-0025](docs/adr/0025-verified-impact.md)); cost data quality is documented in [ADR-0024](docs/adr/0024-cost-data-quality-reconciliation.md).
- Client menu completed to the 12 agreed items: Carbon & Energy (totals, trend, methodology and caveats), Connectors
  (catalog with honest availability), Alerts (cost anomalies, failing connections, undelivered notifications), CI/CD
  (placeholder with the CI gate). Pages without a backend say so.
- Product specification split into parts: `docs/product/` (client workspace, operator console, connectors, data and
  calculations, stack, gap analysis).
- Cost data quality: after each sync the stored daily totals are reconciled with the cloud's own totals (AWS Cost
  Explorer first); mismatching days rewind the sync cursor so the next sync re-reads them; the result is shown per run
  and on the connection, and raises a `connection.data_quality` notification.
- Release and version policy, version manifest, `/version`, `build_info` metric, OCI image labels.

### Security
- AWS: pagination cap is now an error instead of a silent truncation; FOCUS export reading is bounded (total decompressed
  bytes, line length, compressed size) and pinned to the connection's AWS account (`ExpectedBucketOwner`).
- FOCUS records reject NaN/Inf amounts and malformed currency codes before they reach the database.

### Changed
- API contract 0.3.0: methodology versions and comparison endpoints, optional `methodology_version` on recalculate (additive).
- API contract 0.2.0: optional `verification` on recommendations and the `verified` status (additive).
- A failed carbon-calculation enqueue after a successful sync no longer fails (and re-bills) the whole sync.

## [0.1.0-alpha.1] - 2026-10-09

First alpha. It was never tagged on its own: its content shipped as part of alpha.2, the first tagged build. Everything below exists but is provisional; carbon figures use the unverified `RLX-PROVISIONAL-1` set.

### Added
- Multi-tenant core (organizations, members, invitations, API keys, audit log, RLS with least-privilege database roles).
- Connectors: AWS (Cost Explorer, FOCUS data export, rightsizing), GCP (Cloud Billing export in BigQuery with ownership
  proof), Electricity Maps (grid intensity).
- FOCUS v1.4 ingestion, usage- and cost-based carbon engine with SCI, versioned methodology, recommendations
  (region shift, rightsizing) with human approval, automation plans, reports (CSV/JSON/PDF), CI gate.
- Notifications: producer port, e-mail / webhook / Slack channels, database outbox with retries ([ADR-0019](docs/adr/0019-notifications-port-and-channels.md)).
- Operator console and error log ([ADR-0020](docs/adr/0020-operator-console.md), [ADR-0021](docs/adr/0021-error-log.md)).
- Settings engine: platform / organization / operator scopes, locks, history, limits, module switches, maintenance and
  sign-up gates ([ADR-0023](docs/adr/0023-settings-engine.md)).
- Client and operator admin screens; dashboard redesigned after the design mock-up.
