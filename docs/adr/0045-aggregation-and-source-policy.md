# ADR-0045: Aggregation and source policy in the operator console

Status: accepted · 0.1.0-alpha.100

## Context
ADR-0035 made the choice of a grid-intensity source a policy (priority, a pin per region, readings a source needs per day,
the age a reading may have) and recorded the source and basis on every calculation row. The console had the pieces in
three places (Platform settings, a tab of Data sources, the customer's Carbon page) and a menu item "Aggregation &
policies" that said it was planned. Three questions could not be answered without reading the database: why did this
region and day get this figure, how fresh is each source, and how much of what the platform calculated rests on the day's
own readings rather than an estimate.

## Decision
1. **One page** for the policy: priority (an ordered list with an on/off switch per source), thresholds, a pin per region,
   the recalculation and the loading of the past, the provenance of the figures and the rules, on `/operator/aggregation`.
   It edits the same platform settings as before (audited and versioned by the settings engine); nothing is stored twice.
2. **Explanation is asked of the engine.** `carbon/domain.Explain` collects what each trusted source has for a region and day
   through the same `IntensityReader` the engine uses and takes the decision by calling `ResolveDay`, so the page can
   never disagree with a calculation. The operator module reaches it through a function port wired in `internal/app`
   (domains do not import each other). It reads stored readings only and never calls a provider.
3. **A limit per source** (`carbon.source_max_age_hours`, entries `source=hours`): an hourly feed and a monthly one do not
   go stale on the same schedule. A source not listed uses `carbon.max_reading_age_hours`.
4. **Provenance is counted, not shown per organization.** `carbon.operator_intensity_provenance(days)` (migration 0058)
   returns rows by source and basis; rows from before the origin was recorded are reported with an empty source.
5. **Freshness is a grade of the newest reading**: an hourly source is fresh for six hours, a monthly one for 75 days;
   beyond the accepted age it is stale; with no reading, "no data".
6. The rules (one source per day, never averaged; the order of preference; average and marginal kept apart; units and time;
   money; origin recorded; changes apply forward) are stated on the page from the server, so what is written is what
   the engine does.

## Not built
Marginal intensity is not stored yet. There is one policy for grid intensity because it is the only quantity with more than
one source. Spend on paid APIs is not metered (the price depends on the provider's plan); only requests per day are capped.
ENTSO-E as a measured European source.

## Consequences
- A new source is a connector, an entry in the priority enum and a line in the operator's source catalog.
- The explanation shows the policy in force now, applied to stored readings; a row calculated earlier keeps the origin that
  was recorded with it, which may differ if the policy or the data changed since.
