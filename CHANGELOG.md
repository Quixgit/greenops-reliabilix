# Changelog

All notable changes. Format: [Keep a Changelog](https://keepachangelog.com), versions as in
[`docs/release/versioning.md`](docs/release/versioning.md). The version of every part is in [`VERSIONS.md`](VERSIONS.md).

## [Unreleased]

## [0.1.0-alpha.33] - 2026-10-10

### Added
- **UK Carbon Intensity** as a free hourly source for Great Britain (no key; National Grid ESO): half-hourly measured values
  for the cloud regions whose zone is GB, refreshed hourly, loaded for the past with the same backfill, second in the
  default source order. It is "ready" with no setup. Migration 0039 (default order).

## [0.1.0-alpha.33] - 2026-10-10

### Added
- **Ember as a free second grid source** ([ADR-0039](docs/adr/0039-ember-and-climate-trace.md)): monthly national carbon
  intensity (free key, CC-BY 4.0), set up in Data sources, refreshed daily for the regions in use, with a key test. It fills
  days that have no hourly reading and is labelled "monthly national average" in the chart, the origin card and reports.
  Default source order: Electricity Maps, then Ember.
- **Climate TRACE** is listed as evaluated: its public API gives annual emission totals in tonnes without generation, so it
  cannot provide gCO2e/kWh; the registry says so.
- **Source per region** table in the console: for each region in use, what each source has, which one is used now, and a
  selector to pin a source (replaces the free-text `region=source` list).
- **Cost of serving an organization**: per organization, counts of usage rows, calculations, sync runs, deliveries, reports
  and Kubernetes samples over 30 days with an estimated cost from unit costs in the platform settings (new group "Cost of
  serving an organization"). `GET /operator/usage`.
- `GET /operator/carbon/regions`. API contract 0.21.0. Migrations 0037 and 0038.

### Changed
- Calculation rows may carry the intensity basis `month_mean`.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- **Cost control for paid APIs**: every request to the grid data provider is counted per day (and failures), and a daily
  cap (platform setting `carbon.max_requests_per_day`, default 2000, 0 = unlimited) stops further requests until the
  next UTC day; stored readings are used meanwhile. The source page shows today against the cap and the last 7 and 30
  days. `GET /operator/api-usage`. API contract 0.20.0. Migration 0036.

## [0.1.0-alpha.33] - 2026-10-10

### Changed
- Payment provider settings moved to where they are looked for: **Plans & billing → Payment providers** (each opens its own
  settings page). They are no longer listed among the platform services under Data sources.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- **Subscriptions and payments** ([ADR-0038](docs/adr/0038-payments-and-subscriptions.md)): a subscription per organization
  (plan, status active / trial / past due / ended, trial end, paid-until date); the scheduler returns an organization to Free
  when its period has ended. Operators set status and dates when assigning a plan.
- **Payment providers**, set up with keys in the console (Data sources → Platform services): **Stripe**, **Paddle**
  (merchant of record), **LiqPay** and **WayForPay** (Ukraine, UAH), **payment link** (Payoneer, Wise, PayPal.me …) and
  **invoice**. Apple Pay and Google Pay are offered on the providers' payment pages. Notifications are verified by signature,
  applied once, and visible to the operator on Plans & billing. `POST /plan/checkout`, `POST /billing/webhooks/{provider}`.
- Customer **Settings → Plan**: plan and status, usage against limits with bars, modules, change plan at a provider that is
  set up. `GET /plan`.
- Operator **Methodology & SCI** page: every coefficient set with provenance, coefficients, caveats and checksum.
  `GET /operator/methodology`. API contract 0.19.0. Migration 0035.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- **Custom roles per organization** ([ADR-0037](docs/adr/0037-custom-roles.md)): define a role as a named set of
  permissions (Settings → Roles), give it to a member in Members (one select holds built-in and custom roles), withdraw it by
  choosing another role, delete it (members return to their built-in role). Guard rails: administration, data export and
  machine permissions can never be in a custom role; you can only grant permissions you hold; owners are not given custom
  roles; every change is audited. `GET/POST /roles`, `PUT/DELETE /roles/{id}`, `PATCH /members/{id}` with
  `custom_role_id`. API contract 0.18.0. Migration 0034.
- The public `GET /locales` honours the platform language settings (the languages on offer and the default).

## [0.1.0-alpha.33] - 2026-10-10

### Added
- **Multi-language platform** ([ADR-0036](docs/adr/0036-multi-language-platform.md)): English and Russian. Language
  switcher in both headers (cookie, falling back to the browser's language), `<html lang>`, catalogs with parity tests,
  plural forms, numbers, money and dates written by language. API problems carry a stable `code` and a translated `title`
  (`Accept-Language`). Settings `i18n.default_locale` and `i18n.enabled_locales`. The menus, headers and Carbon & Energy
  headings are translated; other page bodies are translated as their keys are added (English fallback).
- **Load past grid data** (Data sources → Which source is used): fetches past hourly readings of the regions in use from the
  provider (a provider plan option), in 9-day requests, with the number of requests shown before anything is asked and hard
  limits (400 days, 2000 requests). Days calculated before readings existed can then use measured intensity after a
  recalculation. `POST /operator/carbon/backfill`, job `carbon:backfill_intensity`. API contract 0.17.0. Migration 0033.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- **Grid intensity from several sources** ([ADR-0035](docs/adr/0035-multi-source-grid-intensity.md)): a platform source policy
  (priority, a source per region, readings needed per day, oldest reading used) edited in the console next to the
  sources ("Which source is used"). For each region and day the first trusted source with enough readings wins; sources
  are never averaged together and marginal readings are never mixed with average ones.
- Every calculation row records the **source and basis** of its intensity (day mean, partial day, last reading before,
  latest overall, live). Rows calculated earlier stay unknown. Migration 0032.
- Client **Carbon & Energy**: grid intensity history per region (chosen source, daily range, other sources beside it, gaps
  left as gaps) and a card saying where the figures' intensities came from with the share that is a same-day mean.
  `GET /carbon/intensity`; the carbon summary carries `intensity_origins`. API contract 0.16.0.

### Changed
- Dashboard region intensities and recommendation inputs read the most trusted source (not "the latest of any provider").

## [0.1.0-alpha.33] - 2026-10-10

### Added
- Operator roles can be assigned and withdrawn from **Access & security** ([ADR-0034](docs/adr/0034-operator-roles-from-the-console.md)):
  add a person who has signed in by e-mail, change a role, withdraw access. The database refuses to change your own role
  and to leave the platform without an admin; every change is in the audit log. `PUT /operator/operators`,
  `POST /operator/operators/withdraw`. API contract 0.15.0. Migration 0031.

## [0.1.0-alpha.33] - 2026-10-10

### Fixed
- **Mixed currencies.** Cost totals, the dashboard cost trend, cost by service, cost anomalies and the cost basis of
  recommendations summed amounts of different currencies together. They now use the dominant currency of the window
  (largest total absolute cost); other currencies are left out, never added. A test pins it.
- **Grid intensity of a day.** A day of usage was calculated with the last reading stored before the end of the day.
  It now uses the mean of that day's readings, and falls back to the latest reading only when the day has none.
  Re-run a calculation to apply it to stored history.

### Changed
- Operator console: **Integrations is merged into Data sources.** Every source and every platform service has its own page
  with its terms, state and settings together (`/operator/data-sources/<id>`); the old page redirects. System no longer
  repeats the integration list, an organization's suspension switch is not repeated in its settings list, and the
  source page links to Connections instead of repeating its list.
- Left menu scrollbar (client workspace and console): thin, inset from the edge, a little wider under the pointer.

### Added
- Suspended organizations are skipped by the scheduler fan-out (no synchronization or calculation jobs). Migration 0030.
- A hand-computed reference test of the carbon arithmetic (energy, PUE, intensity, SCI shares).

## [0.1.0-alpha.33] - 2026-10-10

### Added
- Plan catalog and assignment ([ADR-0033](docs/adr/0033-plan-catalog-and-assignment.md)): Free, Team, Business and
  Enterprise as presets of limits and module switches; **Plans & billing** page with the comparison and the number of
  organizations per plan; assign a plan on an organization's page with a required reason (locked values in the
  settings history, plan and reason in the organization's audit log). `GET /operator/plans`,
  `PUT /operator/tenants/{id}/plan`. API contract 0.13.0. Migration 0029.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- Suspend and restore an organization from its console page. Suspension is the locked operator setting
  `organization.suspended` with a message that people see: they keep read access and exports, every change is refused
  with 403 until an operator restores access. The change is in the settings history and the organization's audit log.
  Background synchronization of a suspended organization is not stopped yet (see the gap list).

## [0.1.0-alpha.33] - 2026-10-10

### Added
- Integrations for sources that are planned: **WattTime** (user name and password) and **UK Carbon Intensity API**
  (address, no key). They are marked "Prepared · not used yet": the values are stored sealed and no platform code reads
  them until the connector is released; the data source registry links to them.

## [0.1.0-alpha.33] - 2026-10-10

### Changed
- Operator console uses the same frame as the client workspace: the dark left panel with the same logo, links and
  version, the white header and a white content area. The pastel theme of alpha.18 is removed.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- Operator console **Jobs & scheduler**: queues (waiting, running, scheduled, retrying, gave up, done and failed today,
  oldest wait), tasks that failed with their last error, and the periodic schedules the scheduler reported. Read-only;
  payloads are never returned. `GET /operator/jobs`.
- Operator console **Access & security**: the operators with role, date added and recent activity (read-only on purpose).
  `GET /operator/operators`. API contract 0.12.0. Migration 0028.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- Operator console rebuilt ([ADR-0032](docs/adr/0032-operator-console-structure.md)): a calm pastel theme, a grouped menu
  (Overview, Customers, Data, Operations, Governance), page search from the header, a mobile drawer and animated tiles
  that respect reduced motion.
- **Overview**: counts, a "needs attention" list built from the real errors, failing connections, failed deliveries,
  missing integrations and failing data sources, and the latest operator activity.
- **Data sources**: registry of everything the platform reads (access type, metrics, coverage, terms, state). State comes
  from the real configuration and connection counts from real customer connections; planned sources are listed and carry
  no numbers. `GET /operator/data-sources`.
- **Audit log**: the operators' own access log, newest first, with a "changes only" view. `GET /operator/audit`.
- Menu entries for sections that are not built yet (plans and billing, aggregation and policies, methodology and SCI,
  jobs and scheduler, access and security) open a page that says what exists today and what is next.
- API contract 0.11.0. Migration 0027 (two read-only SECURITY DEFINER functions).

- `scripts/stand-deploy.sh`: builds and restarts the stand in one step, and applies the newest migration to the stand
  database (`docs/setup/stand.md`).

### Fixed
- Stand: pages answered with a chunk-loading error when the frontend was rebuilt under a running server; the deploy script
  restarts the web unit after every build.

### Changed
- Organizations moved to `/operator/organizations` and can be sorted and narrowed to those with problems; `/operator` is
  now the Overview.

## [0.1.0-alpha.33] - 2026-10-10

### Added
- Kubernetes right-sizing recommendations from measured usage ([ADR-0031](docs/adr/0031-kubernetes-rightsizing.md)): smaller
  CPU and memory requests for workloads whose peak hourly usage is far below what they request, with the measured peak next
  to the request; no saving is claimed. API contract 0.10.0. Migration 0026.
- Google Cloud can be connected from the Connectors page (project, billing export table, setup steps).

### Changed
- Cloud Accounts and Connectors are one page: connected accounts and the connect flow on top, the catalog below.
  `/cloud` redirects to `/connectors`; the account page stays at `/cloud/<id>`.

## [0.1.0-alpha.16] - 2026-10-10

### Added
- Kubernetes usage next to energy: optional CPU time, memory and the requests per workload and hour; the Kubernetes page
  shows CPU and memory utilization (used / requested, over the hours that report both; unknown stays a dash and low use
  is flagged). The agent reads cAdvisor and kube-state-metrics when present. API contract 0.9.0. Migration 0025.
- Systemd user units for a demonstration stand (`deploy/systemd`, `docs/setup/stand.md`) and `SCHEDULER_SKIP` to keep a
  stand without real cloud accounts from scheduling cloud synchronization.

## [0.1.0-alpha.15] - 2026-10-10

### Added
- Console **Integrations** now also hold the settings that are read when a service starts: the platform AWS account,
  the platform Google Cloud project and service account, object storage (S3) and error tracking / tracing. They are
  marked "Applied at restart" and win over the environment ([ADR-0029](docs/adr/0029-integration-credentials-in-the-console.md)).
- Monthly carbon limit per project (project policy, `alert_carbon_monthly_kg`) with its own carbon alert; project policy
  screen (allowed regions, CI gate thresholds, carbon limit) on the Projects page. API contract 0.8.0. Migration 0024.
- Packaged Kubernetes agent image `reliabilix/agent` (built by the release workflow, runs as an unprivileged user).

### Changed
- Sentry and tracing start after the database is reachable so they can use console values.

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
