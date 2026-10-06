# ADR-0014: Usage-based carbon for AWS from Cost Explorer

Status: accepted

## Context
Cost-only data gives a low-confidence carbon estimate (kWh per USD). Cost Explorer can also return
`UsageQuantity`, which for EC2 running hours (by instance type) and S3 storage is a physical quantity.
CUR via S3/Athena gives resource-level detail but needs customer-side setup; it follows later.

## Decision
- The AWS provider issues two extra read-only queries per sync (EC2 running hours by region and instance
  type, S3 storage by region) and returns them as a second batch (`cost_explorer_usage`). They use the same
  `ce:GetCostAndUsage` permission, so customers change nothing. Cost: two more result pages per sync.
- The batch is best effort: if it fails (except for access denied) the cost data still syncs and carbon stays
  cost-based. A permission problem still fails the sync.
- `platform/ec2spec` derives vCPU and memory from the instance type for general-purpose, compute, memory and
  burstable families. Unknown shapes (GPU, metal, high-memory) are not guessed: if any type of a region-day is
  unknown, the whole region-day is left cost-based, because a partial figure would undercount.
- Measured records carry quantity but zero cost, so cost totals are unchanged.
- The carbon engine drops a cost-based row when measured usage exists for the same service, region and day
  (`superseded_by_measured_usage`), preventing double counting.
- A recalculated window now replaces the old rows of the current methodology version in one transaction, so
  superseded rows disappear; rows of older methodology versions are history and stay.

## Consequences
Usage-based figures use provisional coefficients (`RLX-PROVISIONAL-1`) and operational energy only (embodied = 0).
They are more specific than spend-based estimates but remain estimates, labelled by `method`.
