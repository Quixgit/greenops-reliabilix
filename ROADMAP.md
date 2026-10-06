# Roadmap

**Measure → Recommend → Automate.** Each phase builds on the previous one without reworking the foundation.

Legend: ✅ done · 🚧 in progress · 🗓️ planned

---

## Phase 1 — Measure (MVP) ✅ backend · 🚧 frontend

Connect one cloud, see cost and carbon together, get an SCI score.

| | Capability |
|---|---|
| ✅ | Multi-tenant core: onboarding, members, invitations, RBAC, API keys, audit log |
| ✅ | Tenant isolation with PostgreSQL Row Level Security and least-privilege DB roles |
| ✅ | AWS connector (read-only role with External ID, Cost Explorer) |
| ✅ | FOCUS v1.4 usage records, partitioned, idempotent ingestion |
| ✅ | AWS FOCUS data export (S3) as an invoice-level source, read-only role, no credentials stored |
| ✅ | Usage-based carbon for AWS EC2 hours and S3 storage (Cost Explorer quantities, no extra customer setup) |
| ✅ | Rightsizing recommendations from AWS findings with a modelled carbon effect and approval workflow |
| ✅ | Carbon engine with SCI, versioned methodology, grid intensity (Electricity Maps) with caching |
| ✅ | Recommendations (region shift) with approval workflow and residency re-check |
| ✅ | Reports (CSV / JSON / PDF), budgets, anomalies, CI gate, OpenAPI |
| ✅ | Observability, `admin doctor`, setup guide, CI with security scanning |
| 🚧 | Frontend: Overview, recommendations, reports done; members / API keys / policy screens pending |

## Phase 2 — Recommend 🗓️ (backend first)

Broader coverage and smarter advice.

| | Capability |
|---|---|
| 🗓️ | Citable carbon coefficients: verified import of a published dataset (new methodology version) and embodied emissions (SCI's M) |
| 🗓️ | Azure connector (Cost Management API) |
| ✅ | GCP connector (Cloud Billing export in BigQuery, ownership proof, measured compute and storage) |
| 🗓️ | Kubernetes: Kepler → Prometheus → agent → ingestion API |
| 🗓️ | WattTime as secondary grid-data provider |
| 🗓️ | More recommendation types: rightsizing, time shifting, spot migration |
| 🗓️ | Scheduled reports, e-mail delivery, SCI self-certification assistant |

## Phase 3 — Automate 🗓️

Controlled change execution, always behind explicit approval.

| | Capability |
|---|---|
| ✅ | Automation jobs: approved recommendation → risk analysis → reviewed plan → human approval → reported result (the platform never executes changes) |
| 🗓️ | Terraform and Kubernetes executors with dry-run, behind separately granted credentials |
| 🗓️ | Reusable CI action (`greenops-ci-gate`) for GitHub Actions / Azure DevOps |
| 🗓️ | Billing (Stripe), white-label, benchmarking |

## Guiding rules

1. **No guessing.** Missing data is shown as missing; estimates are labelled with their method and confidence.
2. **Nothing changes production without a human decision** and an audit record.
3. **Security by construction:** isolation in the database, secrets never stored in the application database, least privilege everywhere.
4. **Extract a service only when a measured trigger fires** ([ADR-0005](docs/adr/0005-service-extraction-triggers.md)).
