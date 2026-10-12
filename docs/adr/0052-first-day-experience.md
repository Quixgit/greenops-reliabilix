# ADR-0052: The first day: findings, access as code, sample data

Status: accepted

## Context
A new customer judges the product in the first hour: can I connect a cloud without a wiki, and what do I get for it? Connecting meant keys,
roles and buckets typed from instructions; after a sync the Overview showed many numbers but no answer to "what did I find". Without a cloud
account nothing could be judged at all.

## Decision
- **"What we found"** (Overview, top): after the first sync, one card with what the open recommendations are worth a year (money and
  carbon), how many there are, the commitment saving (ADR-0051, shown apart so it is not added to the first), and the three best moves. It reads
  existing endpoints, claims only estimates ("estimates, not promises") and can be hidden (kept in the browser).
- **Access as code** (connection wizard, details step, own-credential ways): Terraform and CLI that create the read-only credential for
  AWS (IAM user with `ce:GetCostAndUsage`, `ce:GetRightsizingRecommendation`), Azure (service principal with Cost Management Reader on one
  subscription) and Google Cloud (service account with BigQuery Data Viewer on the export dataset and Job User on its project). The ids the
  person typed are filled in; the snippets are tested never to ask for more than read access.
- **Plain hints for a failed check** (denied, not found, no data yet, network) added under the platform's own message.
- **Sample data**: `POST /demo` adds a project "Sample data (demo)" with 60 days of invented spend (six services, a weekly rhythm) and asks for
  the carbon calculation and the recommendations now; `DELETE /demo` removes it. It is made by two SECURITY DEFINER functions (migration 0069)
  that touch only what `projects.demo_markers` lists, so a person's own project is never deleted. **No connection is created**, so the sample does
  not use the plan's connection limit; it does use one project of the project limit. If a real connection is later attached to the sample project,
  removal only drops the marker and the synthetic rows. The Overview says plainly that the numbers are invented, and offers the way out.

## Consequences
- `greenops_fanout` gets INSERT/DELETE on the tables the sample touches (it already bypasses row security for erasure).
- The sample's grid figures come from the platform's reference data for its regions (us-east-1, eu-central-1), so recommendations appear when
  those regions have readings.
- Not done: a guided "first sync progress" screen with live status per step, and a sample for Azure and Google Cloud (the sample is AWS-shaped).
