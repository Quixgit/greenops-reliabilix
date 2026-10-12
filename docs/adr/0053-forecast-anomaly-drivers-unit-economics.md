# ADR-0053: Seasonal forecast, anomaly drivers, unit economics

Status: accepted

## Context
Three answers a finance lead asks of any cost tool were weak. The month-end forecast scaled the spend so far to the whole period, which is wrong
twice: spend has a weekly rhythm, and the last day or two are still being delivered by the cloud and look like a drop. An anomaly was a date and
a number ("3 standard deviations") with no cause. The cost of one unit of work existed only as one figure over one window.
(Showback by team, environment and shared-cost rules was already built: teams, environments, allocation rules and the allocation report.)

## Decision
- **Forecast** (`domain.BuildForecast`, `GET /finops/forecast`, budgets): the expected cost of a day is the median of the same weekday over the last
  six weeks, scaled by the trend of the last 14 days over the 14 before (kept within 0.8-1.25). Trailing days under 70% of what their weekday usually
  costs (at most two) are treated as unfinished and forecast, so a late delivery never reads as a saving. The range is 80%, from how far the same rule
  missed on the last 28 trusted days (errors add as independent: sd x sqrt(days left) x 1.28). With fewer than 14 trusted days the method is
  reported as `linear` and the range collapses; the API says which method it used. Budgets keep their state rules (exceeded / at risk / ok) on the
  expected figure and show the range; the Overview says when the high end could cross the budget.
- **Anomaly drivers**: for each of the latest ten anomalous days, the three services-in-region with the biggest increase over their daily average of
  the previous 14 days; a pair with nothing before it is marked new. Shown on the Overview tile with all three in the tooltip.
- **Unit economics** (`GET /dashboard/unit-economics`, Projects > Unit economics): per project and month, cost and CO2e per functional unit over the
  days units were reported (never extrapolated), and the change between the two latest months with at least seven reported days. The unit is the
  project's own (requests, orders, customers).

## Not done
- **Showback by cloud tags and labels.** Usage lines carry no tags: every connector sums the provider's rows into daily lines by service and region
  before they are stored, so tags would have to become part of each connector's aggregation key (AWS and Azure FOCUS exports have a Tags column;
  Google's export has labels) and a setting would have to name the tag keys. That is a connector change with its own tests per cloud, not a report;
  teams, environments and allocation rules cover showback meanwhile.
- A unit that is not per project (cost per customer across projects) and several units per project.
- Alerts that carry the drivers (the alert text is unchanged).
