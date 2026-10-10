# ADR-0042: Teams, environments and shared-cost allocation

Status: accepted · 0.1.0-alpha.57

## Context
A project is the unit of cost and carbon, but organizations also ask "what does this team spend", "how much of it is
production" and "who pays for the shared platform". Cost lines carry no reliable tags (the AWS, GCP and Azure exports
used here do not bring resource tags into the aggregate), so tag-based allocation is not possible yet.

## Decision
- **Teams** own projects (`projects.projects.team_id`, nullable; deleting a team clears it).
- **Environments** belong to a project and carry a kind (production, staging, development, test, other). The cloud
  connections assigned to an environment are what puts cost in it: a connection is in at most one environment, and
  only connections of the environment's own project can be assigned. Cost of unassigned connections is shown as
  "not assigned". This uses data the platform already has (`usage_records.connection_id`) and needs no tags.
- **Allocation rules** split what a source project is billed for a service (or for everything) between target projects
  by relative weight. One rule per source and service; a service rule wins over the rule for everything. The source
  may list itself to keep a share.
- **Read-time application.** Rules never rewrite usage or carbon rows. The allocation report applies them when it is
  read, in a pure function (`domain.Allocate`) whose invariant is that the totals equal the direct cost. A changed
  rule therefore changes every period at once, and nothing needs recalculating. The cost is converted into the
  organization's reporting currency at the rate of each day (alpha.51-52); an amount without a rate is left out and
  counted, never guessed.
- **Allocation moves cost between projects, not between environments**, and applies to cost only: carbon and the
  projects' budgets, alerts and SCI keep reading each project's own figures. Extending allocation to carbon is
  possible (the same weights) but is a methodology decision (who carries the emissions of shared infrastructure) and is
  not made here.
- **Tenant isolation.** Plain foreign keys are checked without row-level security, so a request could have referenced
  another organization's project or connection. All new references are composite `(tenant_id, id)` foreign keys
  (`UNIQUE (tenant_id, id)` was added to `projects.projects` and `cloudaccounts.connections`); the integration test
  tries to cross the boundary through every new endpoint.

## Not done
- Allocation by resource tags or by usage share (needs tag ingestion).
- Weights as percentages that must sum to 100; weights are relative on purpose, so adding a target does not invalidate
  the others.
- History of rule changes beyond the audit log.
