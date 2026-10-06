# ADR-0018: GCP connector on the Cloud Billing export in BigQuery

Status: accepted

## Context
The prompt asks for GCP via the Cloud Billing export to BigQuery (no constant API calls). The platform must keep
its promise of storing no customer credentials, and must not let one tenant read another customer's data through
the platform's own (legitimately authorized) identity.

## Decision
- **Access:** the customer grants the platform's service account `roles/bigquery.dataViewer` on the billing dataset.
  Queries run (and are billed) in the platform's own project, so the customer needs no extra role, and nothing
  secret is exchanged or stored.
- **Ownership proof (confused deputy):** a connection has an unguessable ExternalId-style token. Before any query
  the provider requires the dataset label `<token>: 1`. Only someone with write access to the dataset can set it, so
  a tenant that merely names another customer's dataset is refused. The check repeats on every sync.
- **Locations are references:** `account_ref` is the GCP project whose costs are read, `credential_ref` is
  `bq://<project>/<dataset>/<table>`. Identifiers are validated by strict patterns and the table must be a Cloud
  Billing export (`gcp_billing_export_v1_*` / `gcp_billing_export_resource_v1_*`); they are quoted into the SQL only
  after validation, while every value (project, time window) is a query parameter.
- **Cost and size control:** each query carries `maximumBytesBilled` (default 20 GiB), results are capped (500k
  lines) and the response size is bounded; validation is a dry run that reads and bills nothing.
- **Normalization:** the provider returns a compact daily aggregate by service, SKU, region, currency and pricing
  unit. The ingestion module files measurable SKUs (Compute Engine core and RAM, persistent-disk capacity, Cloud
  Storage capacity) as measured usage under their own service names, and everything else (GPUs, network, licences,
  snapshots, operations) stays a cost line, so measured usage replaces exactly what it corresponds to (ADR-0014).
  GiB are converted to the decimal GB of the methodology.
- **A customer without BigQuery** gets a specific, actionable error rather than a generic failure: API not enabled
  in their project, dataset or table missing (export not enabled or not yet delivered, up to 48 h), no read access,
  ownership label missing. A disabled API in the platform's own job project is reported as a platform problem and is
  never blamed on the customer. Customer-fixable errors are not retried and mark the connection as errored until
  the customer verifies again.

## Consequences
The export has no history before it was enabled, so the first syncs show little data. Standard usage cost is used
(not the detailed resource-level export); FOCUS-format BigQuery views can be added later behind the same provider.
Quota and cost of BigQuery jobs fall on the platform's project and are bounded per query.
