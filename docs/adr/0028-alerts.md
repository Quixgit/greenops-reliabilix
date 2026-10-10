# ADR-0028: Alerts: persistent, deduplicated, evaluated hourly

Status: accepted (implemented, migration 0020)

## Context
The Alerts page only listed what could be computed on the spot (cost anomalies, failing connections, undelivered
notifications). There was nothing to acknowledge, nothing remembered a problem between visits, and nobody was told
when a budget was crossed.

## Decision
- **Stored alerts** in `alerts.alerts` with a lifecycle `new -> acknowledged | muted`, and `resolved` set only by the
  system when the condition clears. A resolved alert is history; if the condition returns a **new** alert is opened.
- **One live alert per condition** (`dedupe_key`, partial unique index), so an hourly check never piles up duplicates.
- **Conditions today:** a budget at 80 % (warning) or over 100 % (critical) of its current period; a connection in
  `error`; a connection whose last reconciliation does not match the cloud (ADR-0024). Cost anomalies stay a computed
  list: they are statistical noise, not a state with a start and an end.
- **Evaluation** is an hourly job over every organization with a project. Signals come through ports implemented in
  `internal/app` from finops and cloudaccounts (domains do not import each other).
- **A source that fails never resolves alerts.** A failed read leaves the alerts of its kinds untouched; clearing an
  alert on a read error would hide a real problem.
- **Muted alerts stay quiet** (no notification, no reopening) while the condition holds and still resolve by themselves.
- **Notification:** opening an alert publishes `alert.opened` (once per alert) through the notify port.
- **People** acknowledge, mute and unmute (`alert:write`: owner, admin, engineer; people only, not API keys).

## Consequences
- Threshold values (80/100 %) are constants; if customers need to tune them they become organization settings and
  per-budget fields.
- Carbon thresholds and "no data for N days" are natural next conditions; each is a pure function in the domain
  plus a signal port.
- The evaluation runs in the worker; a stack without a worker shows the page but opens no alerts.
