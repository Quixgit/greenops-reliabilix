<div align="center">

# Reliabilix GreenOps Platform

**Cloud cost and carbon footprint, one cloud at a time, each in its own currency.**
A multi-tenant FinOps + GreenOps SaaS that turns cloud billing into cost, SCI scores, actionable
recommendations, verified savings and audit-ready reports.

`Go 1.27` · `PostgreSQL 18` · `Redis + Asynq` · `Next.js 16` · `OpenAPI 3` · `FOCUS v1.4`

[Українська версія з повним описом](README.md) ·
[Roadmap](ROADMAP.md) · [Changelog](CHANGELOG.md) · [Versions](VERSIONS.md) · [Decisions (ADR)](docs/adr/)

</div>

---

## What it does

```
Connect clouds ─▶ Sync billing ─▶ Normalize (FOCUS) ─▶ Energy × grid intensity ─▶ CO₂e, SCI
                                                                       │
                              Verified saving ◀─ Human applies ◀─ Recommendation ◀─ Approval
```

- **Cost and carbon together.** Billing from AWS, Azure, Google Cloud (and vendor CSV files) is normalized to the
  FinOps Open Cost and Usage Specification; carbon follows the Green Software Foundation SCI formula under a versioned,
  immutable methodology.
- **One cloud at a time.** Clouds are separate providers with their own currency, metrics and emissions: the Overview,
  Carbon, Recommendations and Projects pages show one cloud in the currency of its bill, on its own arithmetic
  (forecast, anomalies, budgets). Nothing adds US dollars to Canadian dollars ([ADR-0065](docs/adr/0065-cloud-scope.md)).
- **FinOps.** Per-cloud month-end forecast with a weekday pattern and billing lag, per-cloud anomalies with drivers,
  budgets per cloud, commitment (Savings Plan / reservation) sizing with break-even, showback by cloud tags, unit economics,
  Kubernetes cost allocation.
- **Recommendations that are measured.** Region moves (data-residency and latency aware), rightsizing, Kubernetes
  rightsizing; a human approves, the platform plans (it never executes), and the effect is **measured** 14 days before and
  after, against a control group.
- **Reports.** CSV / JSON / PDF, GHG Protocol Scope 3 / ESRS E1, schedules by e-mail; a report covers the clouds you tick.
- **CI gate.** PASS / WARN / BLOCK for planned infrastructure changes, callable with a machine API key; a Terraform provider
  manages projects, policies and budgets.
- **Honest data.** Missing data is shown as missing, estimates carry their method, every figure says which grid source and
  basis it used, and a cloud that stopped delivering data is flagged instead of quietly lowering a total.

## Architecture in one minute

A **modular monolith**: three processes (`api`, `worker`, `scheduler`) plus an admin CLI, built from one codebase and split by
domain. The full picture, with diagrams, every module, every formula and the weak spots, is in the
[Ukrainian README](README.md#2-архітектура-діаграми) (and its source, [`docs/architecture/platform-full.uk.md`](docs/architecture/platform-full.uk.md)).

| Principle | How |
|---|---|
| Domain isolation | 19 domain modules with identical layers (`domain` / `application` / `repository` / `http`); modules never import each other (enforced by lint), they talk through ports wired in `internal/app` |
| Tenant isolation | explicit tenant argument in every query **and** PostgreSQL Row Level Security; runtime DB roles cannot bypass it |
| Cloud scope | a request for one cloud binds `app.cloud`; restrictive read policies hide the other clouds in every query of that request |
| Identity | Auth0 (OIDC) proves who you are; tenant and role come from the database, never from the token |
| Async by default | sync → ingest → carbon → recommendations is a chain of Asynq jobs, no event bus |
| Configuration | one settings catalog in code; platform, organization and operator scopes with inheritance, locks and history ([ADR-0023](docs/adr/0023-settings-engine.md)) |
| Delivery | domains publish to a `notify` port; e-mail, webhook and Slack are plug-in channels behind a database outbox ([ADR-0019](docs/adr/0019-notifications-port-and-channels.md)) |
| Contract first | `backend/api/openapi.yaml` is checked against the routes by a test; frontend types are generated from it |

```
backend/    Go: cmd/{api,worker,scheduler,admin}, internal/{platform,<domains>,app}, migrations/, api/openapi.yaml, tests/
frontend/   Next.js 16 · React 19 · TypeScript · Tailwind · TanStack Query · Zustand · ECharts
deploy/     docker/, aws/ azure/ gcp/ (customer access templates), observability/, systemd/, dev/ (seeds)
docs/       setup/, architecture/, product/, security/, operations/, api/, adr/, release/
terraform-provider-reliabilix/   Terraform provider (projects, policies, budgets)
```

## Quick start

```bash
docker compose up --build        # postgres, redis, migrations, api, worker, scheduler, web
open http://localhost:3000       # dev login, no Auth0 needed (ENV=dev)
```

The dev tenant starts empty on purpose. To see a populated dashboard:

```bash
make seed-demo     # clearly labelled demo rows in the database (the frontend has no mock data)
make recalc        # runs the real carbon engine and recommendations on them
```

Without Docker: `docker compose up -d postgres redis && make migrate && make seed && make run-api`, then `make web`.

## Everyday commands

| Command | Purpose |
|---|---|
| `make test` | unit tests, including the OpenAPI ↔ routes parity test |
| `make test-integration` | real PostgreSQL + Redis: RLS on every tenant table, role privileges, erasure, the end-to-end pipeline |
| `make lint` / `make vuln` | gosec + module-isolation rules / govulncheck |
| `make sqlc` / `make api-types` | regenerate repositories / frontend API types (CI fails on drift) |
| `make roadmap` | regenerate the roadmap blocks of this file and `ROADMAP.md` from `VERSION`, `CHANGELOG.md`, `docs/product/08-phases.md` |
| `make doctor` | verify every configured external integration |
| `make smoke` | end-to-end check against a running stack |

## Connecting external services

Step-by-step guide: [`docs/setup/README.md`](docs/setup/README.md) (database, Redis, Auth0, AWS, Azure, Google Cloud, grid
carbon data, object storage, observability). Run `make doctor` after each step. Keys of external services live in the
operator console, sealed, never in environment variables read at the call site ([ADR-0029](docs/adr/0029-integration-credentials-in-the-console.md)).

## Roadmap

> **Measure → Recommend → Automate**, then make it sellable. Each phase builds on the previous one without reworking the
> foundation. The block below is generated, do not edit it by hand; the full roadmap is in [`ROADMAP.md`](ROADMAP.md).

<!-- roadmap:start -->
<!-- generated by scripts/gen-roadmap.py from VERSION, CHANGELOG.md and docs/product/08-phases.md: do not edit between the markers -->

**Now:** `0.1.0-alpha.180` · stage **alpha** · API contract `0.52.0` · schema migration `83`

| Phase | The pain it answers | State |
|---|---|---|
| 1 | «Мне нужно быстро увидеть пользу» | Done · alpha.154 (ADR-0052); остаётся живой экран хода первой синхронизации, пример для Azure и GCP |
| 2 | «Разнесите расходы по командам» | Done · alpha.155–156 (ADR-0053): прогноз, причины аномалий, единичная экономика; команды, окружения и правила распределения уже были. Остаётся showback по тегам облака (нужны теги в коннекторах) |
| 3 | «Платформа показывает, но не делает» | Done · alpha.157 (ADR-0054): чтение у AWS, совет по покупке, проверка с контрольной группой. Азия/GCP не читаются (нужны реальные аккаунты), для них ручной ввод |
| 4 | «Мне нужно пройти аудит и выдать отчётность» | In progress · alpha.158 (ADR-0055): отчёт GHG Protocol Scope 3 / ESRS E1 готов; страница доверия `/trust` и черновик DPA готовы (alpha.159); SCIM готов (alpha.160, ADR-0056)… |
| 5 | «Встройте это в мой процесс» | In progress · alpha.161–162: приглашения письмом со страницей `/invite` и дайджест в письмах по расписанию готовы; alpha.163: событие «план ждёт утверждения» и кнопка в Slack (ссылка в приложение… |
| 6 | «Мне есть что продавать» | In progress · alpha.165: нагрузочные цифры измерены и опубликованы (`docs/operations/performance.md`), найден и исправлен медленный расчёт валюты (14 с → 1 с)… |

**Latest releases**

- **0.1.0-alpha.180** (2026-10-12): Top recommendations on the Overview use one soft grey
- **0.1.0-alpha.179** (2026-10-12): The cloud switch is a slim pill
- **0.1.0-alpha.178** (2026-10-12): The README is the full description
- **0.1.0-alpha.177** (2026-10-12): README rewritten; Full platform description in Ukrainian; Commercial licence
- **0.1.0-alpha.176** (2026-10-12): Erasing an organization failed whenever it had a report schedule, a commitment snapshot or a support view
- **0.1.0-alpha.175** (2026-10-12): No "all clouds" total anywhere on the Overview; Connectors and an account's page show a connection's cost in the currency it is billed in; Reports choose their clouds; Budgets can belong to a cloud

<!-- roadmap:end -->

**Guiding rules**

1. **No guessing:** missing data is shown as missing; estimates carry their method and confidence.
2. **Nothing changes production without a human decision** and an audit record.
3. **Security by construction:** isolation in the database, customer secrets never stored, least privilege everywhere.
4. **Extract a service only when a measured trigger fires** ([ADR-0005](docs/adr/0005-service-extraction-triggers.md)).

## Honest status

- **The carbon coefficients are provisional** (`RLX-CCF-1`): the usage-based ones come from the published Cloud Carbon Footprint
  constants, the spend-based ones are the authors' estimates without a source. **Embodied emissions (SCI's M) are not
  included.** Every API answer, report and dashboard says so next to the numbers. Do not use the figures for external reporting
  until the methodology is verified.
- Carbon is **usage-based** where the bill carries quantities (EC2 hours, storage, Azure VM hours, GCP compute and storage, FOCUS
  exports) and **spend-based** for everything else; grid intensity for many regions is **computed** from the generation mix.
- AWS Cost Explorer marks the open month as an estimate; those days are read and re-read until AWS finalizes them.
- The platform plans changes, it never executes them; the Terraform and Kubernetes executors are a later phase.
- A DigitalOcean connector does not exist yet (the interface is ready for one); other SaaS vendors come in as CSV files.

## License

Commercial and proprietary. Copyright (c) 2026 Reliabilix Systems. All rights reserved. Use of the platform, hosted or
self-managed, requires an agreement with Reliabilix Systems; see [`LICENSE`](LICENSE). Third-party components and public data
sources keep their own licences and terms.
