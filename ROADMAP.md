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
| ✅ | Carbon engine with SCI, versioned methodology, grid intensity (Electricity Maps) with caching |
| ✅ | Recommendations (region shift) with approval workflow and residency re-check |
| ✅ | Reports (CSV / JSON / PDF), budgets, anomalies, CI gate, OpenAPI |
| ✅ | Observability, `admin doctor`, setup guide, CI with security scanning |
| 🚧 | Frontend: Overview, recommendations, reports done; members / API keys / policy screens pending |

## Phase 2 — Recommend 🗓️ (backend first)

Broader coverage and smarter advice.

| | Capability |
|---|---|
| 🗓️ | Azure connector (Cost Management API) |
| 🗓️ | GCP connector (Cloud Billing export in BigQuery) |
| 🗓️ | AWS CUR → S3 → Athena for resource-level, usage-based carbon |
| 🗓️ | Kubernetes: Kepler → Prometheus → agent → ingestion API |
| 🗓️ | WattTime as secondary grid-data provider |
| 🗓️ | More recommendation types: rightsizing, time shifting, spot migration |
| 🗓️ | Scheduled reports, e-mail delivery, SCI self-certification assistant |

## Phase 3 — Automate 🗓️

Controlled change execution, always behind explicit approval.

| | Capability |
|---|---|
| 🗓️ | Automation jobs: approved recommendation → risk analysis → plan → execution → verification |
| 🗓️ | Terraform and Kubernetes executors with dry-run and rollback information |
| 🗓️ | Reusable CI action (`greenops-ci-gate`) for GitHub Actions / Azure DevOps |
| 🗓️ | Billing (Stripe), white-label, benchmarking |

## Guiding rules

1. **No guessing.** Missing data is shown as missing; estimates are labelled with their method and confidence.
2. **Nothing changes production without a human decision** and an audit record.
3. **Security by construction:** isolation in the database, secrets never stored in the application database, least privilege everywhere.
4. **Extract a service only when a measured trigger fires** ([ADR-0005](docs/adr/0005-service-extraction-triggers.md)).
