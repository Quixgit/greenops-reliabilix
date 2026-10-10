# ADR-0032: Operator console structure, data source registry and audit view

Status: accepted · 0.1.0-alpha.34

## Context
The console grew by accretion (ADR-0020, 0021, 0023, 0029): a flat list of seven pages. The product specification
(`docs/product/02-platform-admin.md`) asks for a structure an operator can scan, a registry of data sources with their
terms and state, and a view of the operators' own audit trail.

## Decision
1. **Menu in five groups** (Overview, Customers, Data, Operations, Governance), defined once in
   `frontend/components/operator/nav.ts`. A section that is not built is in the menu marked `planned` and opens a page that
   states what exists today and what comes next (`planned.ts`). Nothing in a planned page is a number.
2. **Data source registry is a catalog in code** (`internal/app/operator.go`), because a source exists only when there is a
   connector for it. State is read from the real configuration; connection counts come from
   `cloudaccounts.operator_provider_usage()` (counts only). A source that is only planned is listed and never counted.
   Cost metering, per-source limits and priorities are not part of this change and the page says so.
3. **Audit view** reads `platform.operator_audit(limit)`. It is read-only; the log stays append-only and only the API role
   can read it through the function.
4. **Same frame as the client workspace** (dark left panel, white header, white content), so the two contours read as one product; only the menu differs.

## Consequences
- Adding a source is one entry in the catalog plus, for a customer connector, one line mapping it to a provider.
- A real, editable source registry (priorities, fallback, cost limits) needs its own tables and a methodology decision;
  it is the next step of the "Aggregation & policies" section.

## Update (0.1.0-alpha.34): one place for sources and their settings
Integrations are merged into Data sources. A data source and the keys or addresses it needs are one thing, so the keys are
edited on the source's own page (`/operator/data-sources/<id>`); platform services without a data source (mail, object
storage, error tracking) are in the "Platform services" tab of the same section. The registry entry carries
`integration_id` to the catalog entry that holds its settings. A fact lives on one page: System shows the build and
the schema only, Connections is the only list of failing customer connections, an organization's suspension is the
Status panel only.
