# ADR-0050: A plan limit on tracked spend, and scheduled reports

Status: accepted

## Context
The market sells by the cloud spend under management (Vantage: $2,500 free, then steps; Cloudability a percentage), while our plans
separated only by the number of connected cloud accounts. A customer with $100,000 a month paid like one with $20,000
([pricing](../product/07-pricing.md)). Separately, every buyer of a reporting product expects the report to arrive without a request.

## Decision
**Tracked spend.** `limits.tracked_spend` (operator scope, US dollars of the last 30 days, 0 = no limit) is part of every plan:
Free $2,500, Team $10,000, Business $40,000, Enterprise none. The spend is the sum of `billed_cost` of the last 30 days converted to USD at
each day's reference rate (an amount in a currency without a rate is left out, as in every other total).
- The plan page and the plan card show it in dollars; above 100% a warning says what happens.
- Above **120%** a new cloud connection is refused (402, as every plan limit). The margin is there so a customer who is a little over
  is not stopped by a price spike. Spend already tracked is never removed, and nothing else is blocked.
- The count of connections stays: it is what costs the platform money; the spend is what the customer pays for.

**Scheduled reports.** `reports.schedules` (migration 0068): kind, format, project, cadence (`weekly`: Monday 06:00 UTC for the seven
days before; `monthly`: the 1st for the month before) and up to 20 recipients. An hourly job (`reports:run_schedules`) moves each due
schedule to its next run *before* it asks for the report (a retried job never makes the report twice; a failed request skips that run).
When the report is ready the recipients get the usual "report ready" e-mail with a link; **the file is not attached**, it stays behind
the sign-in. A recipient must be a member of the organization, checked when the schedule is made and again at every send
(`platform.member_emails`, because users are not readable by the sending roles). A paused schedule resumes at the next regular run, not
with the runs it missed.

## Consequences
- The worker role gets INSERT on `reports.reports` (it asks for the scheduled report itself).
- E-mail goes only to members. A person outside the organization cannot be added, which keeps the feature from being a mail relay.
- Without SMTP the stand logs the message instead of sending it; with it, nothing else changes.
- The retention of files by plan is still not a plan setting.
