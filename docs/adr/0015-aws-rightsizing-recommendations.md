# ADR-0015: Rightsizing recommendations from AWS Cost Explorer

Status: accepted

## Context
Rightsizing is the first recommendation type that needs facts about single instances. AWS already analyses
utilization and exposes the result through Cost Explorer (`GetRightsizingRecommendation`), so the platform does
not need CloudWatch access or its own utilization model.

## Decision
- A daily job per healthy connection (`cloudaccounts:sync_rightsizing`) asks AWS for findings and passes them
  through a port (`RightsizingSink`) to the recommendations module; the composition root adapts one to the
  other, so the domains still do not import each other.
- The carbon effect is modelled with the carbon methodology's coefficients from the instance shapes (vCPU and
  memory reported by AWS, falling back to `platform/ec2spec`) and the stored grid intensity of the instance's
  region. Terminate removes all modelled energy of the instance; modify removes the difference.
- A finding is skipped, never guessed, when the shape or the region's intensity is unknown, when the modelled
  energy does not drop, or when the effect is below 0.5 kg CO2e per month. Skips are logged with their reason.
- The cost effect is AWS's own estimate and only shown in USD (`cost_basis = provider_estimate`).
- The permission `ce:GetRightsizingRecommendation` is optional. Without it the job quietly does nothing and the
  connection stays healthy; cost and usage syncs never depend on it.
- Findings use the normal approval workflow. Region does not change, so data residency is trivially satisfied.

## Consequences
Customers must opt in to rightsizing recommendations in Cost Explorer preferences. Findings AWS stops
reporting are not closed automatically yet; open ones stay until a human decides.
