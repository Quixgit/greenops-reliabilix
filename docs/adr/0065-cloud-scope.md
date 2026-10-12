# ADR-0065: One cloud at a time, in its own currency

Status: accepted · 0.1.0-alpha.174, extended in 0.1.0-alpha.175

## Context
A project can hold two clouds billed in two currencies (AWS in US dollars, Azure in Canadian dollars). The Overview poured both into one
figure in the reporting currency. That is a valid total, but it hid three real problems:

- a cloud whose data stops arriving (AWS Cost Explorer marks the open month as *estimated*, and the normalizer dropped estimated days)
  looked like a drop of the whole bill, and the month-end forecast, "spent so far" and the budget were wrong;
- the automatic reporting currency was the dominant currency **of the window a query covered**, so a 30-day window said CAD and the
  56-day window the forecast reads said USD: a forecast in dollars was printed with the Canadian symbol;
- every chart added amounts that nobody was billed in: the person who pays AWS in dollars and Azure in Canadian dollars wanted to see
  each as it is billed, with its own arithmetic (forecast, anomalies, rhythm, lag).

## Decision
1. **Scope.** A GET request carrying `?cloud=aws|azure|gcp|...` is scoped to that cloud. `httpx.CloudScope` puts the value in the
   context; `database.WithTenantTx` binds the transaction setting `app.cloud`; migration 0080 adds `RESTRICTIVE` read policies
   (`cloud_scope`) on `usage.usage_records`, `usage.tagged_cost`, `carbon.calculations`, `recommendations.recommendations` and
   `cloudaccounts.connections`. Every query of the request, current and future, then sees one cloud: no `WHERE provider` to remember.
   Writes are never scoped (the middleware looks at GET only; policies are `FOR SELECT`).
2. **Own currency.** `finops.reporting_currency()` ignores the organization's choice in a scoped request and returns the currency that
   carries most of that cloud's cost. The Overview figures of a scoped cloud are in the currency on its bill.
3. **Stable automatic currency.** Without a choice the currency is the one carrying most of the organization's cost over the last 90
   days in euro-equivalent (latest reference rate); the window's own dominant currency is only the fallback (migration 0079).
4. **Per-cloud arithmetic also in "all clouds".** Forecast = sum of per-cloud forecasts (each on its own weekday pattern and lag, the band
   adds the misses in quadrature); anomalies are judged per cloud; every cloud reports `days_with_data` and `last_day`, and a cloud more
   than two days behind is flagged wherever its figures are used.
5. **AWS open month.** Estimated days are ingested; the sync window re-reads the open month (and the previous one in the first five days
   of a month) until AWS finalizes it. Azure and GCP are unchanged.
6. **UI.** A cloud switch under the page title (All / each cloud with its billing currency). "All clouds" is the comparison: a card per
   cloud in its own currency with the converted figure beside it, and a chip row to choose the currency of the total.

## Consequences
- A policy predicate is evaluated for every read of those tables; the setting is read once per statement (uncorrelated sub-select) and
  the check is a plain comparison. Measured on the stand it is not visible; the load test (`scripts/loadtest`) should be run before a
  large tenant relies on it.
- A request that reads these tables for another purpose and carries `cloud` is scoped too: only the Overview sends the parameter.
- DigitalOcean has no connector yet; the UI (colour, mark, switch) is ready for it, a billing connector is the missing part.

## Extension (alpha.175)
- The scope value may be a list (`app.cloud` = `aws,azure`; migration 0081): a report is narrowed to the clouds its author ticked.
- "All clouds" is no longer a view of the Overview: there is no figure that adds clouds billed in different currencies. The switch has a
  tab per cloud and the pages that show cost or carbon of several clouds carry it (Overview, Carbon, Recommendations, Projects).
- Budgets may belong to a cloud (`finops.budgets.cloud`, migration 0082) and are then evaluated inside that cloud's scope.
