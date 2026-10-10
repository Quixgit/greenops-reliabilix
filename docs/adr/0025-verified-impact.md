# ADR-0025: Verified impact: measure what an applied recommendation actually achieved

Status: accepted (implemented, migration 0018)

## Context
A recommendation carries a forecast (carbon and cost). Showing the forecast as if it were the result misleads, and a
customer who applied a change deserves to know whether it worked. The platform does not execute changes (ADR-0017), so
the only honest evidence is what the billing and carbon data say afterwards.

## Decision
- A recommendation gains the status **`verified`**: `applied -> verified`, system-made, terminal. It means "a measured
  result exists", not "it worked"; the **outcome** says that: `realized` (>= 80 % of the expected carbon reduction),
  `partial` (20-80 %), `not_realized` (< 20 % or worse), `inconclusive` (not enough data).
- The result is a **separate row** (`recommendations.impact_verifications`), never written over the forecast fields.
- **Method `before_after_daily_mean`:** daily mean of the recommendation's scope (project and service, all regions, so
  a region shift counts as the workload total) over the 14 days before the day of the change, versus the 14 days that
  start 4 days after it (3 days skipped for late billing data); scaled to 30 days. Cost and carbon are reported
  separately; positive means saved.
- **Never extrapolate.** Fewer than 10 days of data in a window means no number. The change waits for late data and is
  closed as `inconclusive` 60 days after it was applied.
- **Honest limits** are stored with the result: workload changes are not controlled for, so it reports what happened, not
  proof of cause. The currency change between windows suppresses the cost comparison.
- It runs inside the existing per-project `recommendations:calculate` job, so no new schedule or cross-tenant function
  exists. The worker role may insert verifications; the API role may only read them.

## Consequences
- The API `Recommendation` has an optional `verification`; the UI shows forecast and measurement side by side.
- A noisy project can mask or exaggerate a single change. A control-group or seasonal method can be added as a new
  `method` value without changing storage.
- Verified savings are the number to use in reports; forecasts stay labelled as forecasts.
