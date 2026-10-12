# ADR-0058: Cost queries compute the reporting currency once and convert only what needs it

Status: accepted

## Context
The first load measurement on a realistic volume (1.0 million usage rows a year for one organization, [performance.md](../operations/performance.md)) showed the
three-month summary taking 14.4 s. The cost queries called `finops.reporting_currency(...)` (two settings lookups) for every row and `finops.convert_cost(...)` for every row,
even when the row already was in the reporting currency. A SQL function with `SET search_path` and sub-selects cannot be inlined, so each call was a full function call: about
a second per quarter of a year of data.

## Decision
In every query that converts cost (finops summary, anomalies, unconverted amounts, total; the dashboard trend, services, regions, efficiency, EC2 regions and connections; the daily
cost, anomaly drivers, commitments and unit economics queries), the reporting currency is computed once in a CTE (`cur`), and each amount is
`CASE WHEN row currency = reporting currency THEN amount ELSE finops.convert_cost(...) END`. The conversion function is unchanged and still decides every row in another currency, so rates, the
ten-day look-back and the "left out when there is no rate" rule are as before. The change was made mechanically (a script that wraps each call) and checked: 16 answers byte for byte against the
old build on a large and a small organization, and 11 answers on an organization with EUR as reporting currency, USD and GBP rows and days without a rate.

Tried and rejected: a daily view (`usage.daily_cost`) grouping the usage rows, which gave only a 2 to 3 times gain (against 14 to 17 times) and needed a migration, since the view itself groups hundreds of thousands of rows on every
read; an inlinable version of the function (sub-selects prevent inlining); aggregating before converting by hand in each query (more code for the same effect once the per-row calls are gone).

## Consequences
The three-month summary of the large organization: 14.4 s to 1.0 s; the other conversions 10 to 17 times faster; no schema change; the results are identical. Still linear in the rows read:
the next step is a daily roll-up table kept by the ingestion, and it is not built. A new query that converts cost must follow the same shape (a `cur` CTE and the CASE), or it will show up in the load test.
