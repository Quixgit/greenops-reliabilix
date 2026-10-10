# ADR-0043: Azure through the customer's service principal and the Cost Management API

Status: accepted · 0.1.0-alpha.59 · extends ADR-0041

## Context
ADR-0041 read a Cost Management export from Blob Storage with the platform's own Entra application. In practice no
customer will create exports, a storage account and a role assignment just to be measured; what they can give is a
service principal that may read costs, the way an AWS role or a GCP service account is given. The Cost Management
Query API needs only that.

## Decision
- **Access:** the customer's own service principal with the built-in **Cost Management Reader** role on the
  subscription (read-only, cost data only). The connection carries `credential_ref = azcost://<tenant>@<client>` and
  the subscription as `account_ref`; the client secret is a write-only field of the create request.
- **The secret is the one customer secret the platform keeps.** AWS (role + ExternalId) and GCP (label) store nothing;
  Azure's Query API cannot be reached without the customer's credential unless the customer's tenant trusts the
  platform's multi-tenant application, which customers do not set up. The secret is sealed with AES-256-GCM under the
  platform key (the notifications key), its additional data binds it to the organization and the connection token
  (a copy in another row cannot be opened), it is `json:"-"` on the domain type, never selected by an export table
  (columns are listed explicitly), and the create response is tested to carry neither the secret nor its sealed form.
  Without a sealing key such a connection is refused (503) instead of stored unprotected. Revocation is the
  customer's: delete the role assignment or the secret in Entra ID.
- **No confused deputy:** possession of the secret is the proof of control, and it only ever opens the customer's own
  subscription, so there is no ownership label as in the export mode.
- **What is read:** per window of at most 92 days, two daily queries grouped by service and location (ActualCost for the
  billed amount, AmortizedCost for the effective one) and an ungrouped ActualCost query as an independent day total
  for reconciliation. Pagination follows `nextLink` only on the management host (the token never goes elsewhere);
  429/503 are waited out within bounds; 401/403 are access denied (not retried), anything else is retryable.
- **Carbon:** the API returns cost without quantities, SKUs or instance sizes, so every line is a cost-based estimate and
  nothing is measured. Measured VM hours need the export mode (ADR-0041). A subscription has one connection (the
  account is unique per organization and provider), so a customer chooses: costs of every service with cost-based
  estimates, or an export with measured VM hours and storage.
- **Locations** come back in words ("EU West", "US East 2"): they are mapped to region ids (rule-based, with a table for
  the irregular ones); an unmapped location simply has no grid data and is counted among the skipped lines.

## Not done / risks
- The shapes of the Query API answer and of the location names were taken from Microsoft's documentation and are not
  yet exercised against a live subscription; a mismatch fails the sync with a clear error (unexpected answer) or leaves
  a location without grid data, never a wrong total (the day totals are reconciled against the independent query).
- Cost Details (the asynchronous report) would give SKUs and quantities through the same service principal; it is the
  next step toward measured Azure usage without an export.
- Multi-tenant platform application mode (customer assigns a role to the platform's own principal, no secret stored)
  is not built: it needs an ownership proof for the subscription that Cost Management Reader alone cannot read.
