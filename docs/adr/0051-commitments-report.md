# ADR-0051: A commitments report (what a Savings Plan or reservation would save)

Status: accepted (a report; nothing is bought)

## Context
The biggest saving the FinOps competitors sell is the management of commitments (Savings Plans, reservations, committed-use discounts);
Vantage takes 5% of the saving for it. We do not buy anything for a customer and do not hold credentials that could. What we can say
honestly is how much of the bill is steady and what committing to part of it would save and risk.

## Decision
`GET /finops/commitments` and the "Commitments" tab of Recommendations:
- **Committable spend**: `compute` and `database` categories, per provider and day, in the reporting currency. Storage, network and
  other services are not committable here. The window ends at the start of today.
- **Baseline**: the daily spend reached or exceeded on 80% of the days (nearest-rank 20th percentile). Days without spend are not zeros:
  a gap in the data never lowers the baseline. Fewer than 14 days with data: `not_enough_data`, no figure.
- **Saving** = baseline x coverage (default 70%) x discount x 30.4 days. **Commitment per hour** = baseline x coverage x (1 - discount) / 24.
- **Risk**: a commitment is paid whether used or not, so the saving falls with utilization and turns negative below `1 - discount`
  (the break-even); the page shows 100%, 80% and 60% use.
- **Discounts** are planning defaults per cloud and term (AWS 25/40%, Azure 15/35%, Google Cloud 28/46% for 1/3 years), typical figures
  for a flexible plan and **not taken from a live vendor page or a quote**. The real rate depends on family, region, payment option and
  the customer's agreement, so the page has an input per cloud that overrides them, and says so.
- If the amortized (`effective_cost`) and billed price differ by more than 3% the page warns that part of the spend may already be
  under a commitment; we do not read the cloud's existing commitments yet.

## Not done
Reading existing commitments and their utilization from the clouds (needs more permissions per connector); recommending a purchase per
instance family and region; buying. The report is also not forecast-based: it assumes the next months look like the window.
