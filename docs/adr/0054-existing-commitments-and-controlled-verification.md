# ADR-0054: Reading existing commitments, purchase advice, and savings verified against a control group

Status: accepted

## Context
The commitments report (ADR-0051) assumed nothing was committed yet, so a customer who already held Savings Plans was told to buy what they
had. And "verified savings" compared a service's cost before and after a change, so a workload that grew or shrank by itself was credited or
blamed to the recommendation.

## Decision
**Existing commitments, AWS.** The daily rightsizing job also reads, with Cost Explorer, the Savings Plans (utilization, coverage, unused
commitment, net savings) and reservations (utilization, coverage hours, unused cost) of each connection over the last 30 days and keeps the
latest reading per connection and kind (`cloudaccounts.commitment_snapshots`, migration 0071). It needs four optional permissions
(`ce:GetSavingsPlansUtilization`, `ce:GetSavingsPlansCoverage`, `ce:GetReservationUtilization`, `ce:GetReservationCoverage`), now in the setup
policy and the Terraform/CLI snippets; without them the reading is skipped, not failed, and the connection stays healthy. A figure the cloud
did not give is null, never zero; an account without commitments has no snapshot. The platform only reads; it never buys or changes anything.

**Azure and Google Cloud are not read.** Their commitment APIs need other scopes (reservation readers, Compute) that cannot be checked without real
accounts here, so for them, and to correct a reading, the report has input fields for coverage and utilization ("entered by you" wins over
"read from your cloud").

**Advice** (per cloud, in the commitments report): utilization under 90% is `reduce`: no purchase is advised, the unused commitment is shown, and
the page says to use what is paid for first. Otherwise only the part of the target coverage not yet held is counted: the existing coverage
(a share of eligible spend) is restated as a share of the steady baseline, and the difference to the target is `buy` (with the saving and the
hourly commitment for the addition only) or `hold` when nothing is left to add. With nothing known the figures are unchanged and the page says to
check the console first. Several accounts are merged by commitment size (utilization) and mean (coverage); Savings Plans win over reservations.

**Verification with a control group.** The control is the rest of the project (its total less the recommendation's service) over the same two
14-day windows (each holds two of every weekday, so the weekly rhythm cancels). The service's expected cost after the change is its own before-mean
times how the control's daily mean moved; the saving is that less what was measured, in cost and in carbon, per 30 days. The control is used only
when it has 10 or more days in both windows, is at least 30% of the service's daily cost, and moved by less than a factor of two; otherwise the plain
before/after result stands and the note says why. The record keeps both figures (`measured_*` raw, `adjusted_*` corrected), the control's growth and
its days (migration 0070); the Overview's verified saving uses the corrected one when there is one. Changes made to other services at the same
time still blur it; the note says so.

## Not done
Reading commitments from Azure and Google Cloud; per-family and per-region purchase advice; a control from a different project; verification of
recommendations other than a change of one service. Existing recommendations verified before this change keep their plain result.
