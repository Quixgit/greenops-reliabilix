# ADR-0055: GHG Protocol Scope 3 / ESRS E1 inventory report

Status: accepted

## Context
Phase 4 of [the plan](../product/08-phases.md) is "I have to pass an audit and file a report". A company that buys cloud services counts their
emissions as Scope 3, category 3.1 (Purchased goods and services) in the GHG Protocol, and discloses the gross figure under ESRS E1-6 (CSRD).
The existing reports (carbon, SCI, sustainability summary) hold the numbers but not in that frame, and say nothing about the boundary.

## Decision
A new report kind `ghg` (migration 0072 widens the kind checks of `reports.reports` and `reports.schedules`). It reuses the report pipeline
(CSV, JSON, PDF, schedules, formats allowed by settings) and the four-column layout (section, item, value, unit) the PDF already draws as a list
of findings. Content, in tCO2e: the inventory frame (standard, category, disclosure, approach), the gross total and energy, the share from
measured usage against estimated from spend, then by provider, by month and by service (top 25). A "Not included" block states embodied
emissions (category 3.2) as 0 and Scope 1 and 2 of the organization as not covered. The notes carry the methodology status; with provisional
coefficients the report says it is not suitable for a published statement. One new query (`ReportGhgBreakdown`), the same calculation rows as
every other report, so the totals agree with the Overview.

## Not done
Embodied emissions (needs per-instance hardware data), market-based accounting, the organization's own Scope 1 and 2, the full ESRS E1 data
point list, assurance or sign-off. Still open in phase 4: embodied emissions, the trust page and security policies, DPA, SSO (SAML/OIDC) and SCIM.
