# ADR-0024: Cost data quality: reconcile every sync with the cloud's own totals

Status: accepted (implemented, migration 0017)

## Context
The product's first promise is that cost and carbon figures can be trusted. Billing APIs paginate, exports arrive late,
and a failure that drops a page looks exactly like a quiet day. A silent hole in a report is discovered months later,
by the customer, which is the worst way.

## Decision
- **Never truncate silently.** A pagination cap, an oversized export line or an unexpected export owner is an error,
  not a shortened result (AWS connector).
- **Reconcile after every sync.** The day totals stored by the platform are compared with an independent total from the
  cloud (AWS: a Cost Explorer query without grouping). Tolerance: 0.5 % of the day's total or 0.05 in the billing
  currency. A day that the cloud reports but we have no cost for counts as a full mismatch.
- **Self-heal.** Mismatching days rewind the sync cursor to the earliest bad day so the next sync reads them again.
- **Say so.** The result is stored on the run and on the connection (`quality`, `last_quality`), shown in the product and
  raised as the `connection.data_quality` notification. Without an independent total the status is `unchecked` with the
  reason, never `ok`.
- **Missing is not zero.** Absent cost stays absent; amounts that are NaN/Inf or have malformed currency are rejected
  before storage.

## Consequences
- One extra, ungrouped Cost Explorer query per sync (a few cents per month per connection at most).
- GCP and the FOCUS export path report `unchecked` until they get a control total (tracked in `docs/product/06-gap-analysis.md`).
- The tolerance is a constant; if customers need to tune it it becomes a setting in the settings catalog.
