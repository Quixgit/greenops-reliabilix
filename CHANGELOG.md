# Changelog

All notable changes. Format: [Keep a Changelog](https://keepachangelog.com), versions as in
[`docs/release/versioning.md`](docs/release/versioning.md). The version of every part is in [`VERSIONS.md`](VERSIONS.md).

## [Unreleased]

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
