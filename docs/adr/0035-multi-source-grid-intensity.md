# ADR-0035: Grid intensity from several sources

Status: accepted · 0.1.0-alpha.32

## Context
The platform stores grid carbon intensity from external providers (Electricity Maps today; UK Carbon Intensity and
WattTime are prepared). Readers took "the latest reading of the region" regardless of provider, so a second source would
have been mixed in silently; a day of usage was calculated with one reading, and the result did not say where its
intensity came from. Average and marginal intensity are different quantities.

## Decision
1. **A reading has a measure** (`average` or `marginal`). Only `average` feeds carbon figures. Marginal readings are stored
   and never mixed with them.
2. **One source per region and day, in policy order, never averaged.** `domain.ResolveDay`: (1) the first trusted source
   with at least `carbon.min_day_readings` readings that day, using their mean; (2) the first source with any reading
   (partial); (3) the last reading before the day not older than `carbon.max_reading_age_hours`; (4) the latest reading of
   all, marked as an estimate; (5) a live provider call. Sources outside `carbon.provider_priority` are not used.
   `carbon.region_provider_overrides` puts one source first for a region.
3. **The policy is platform settings** (group "Grid data sources", editable in the console next to the sources), so it is
   audited and versioned by the settings engine. SQL readers that cannot call the Go resolver use the one function
   `carbon.provider_priority()` (priority only); a test pins its default to the catalog default.
4. **Every calculation row records `intensity_source` and `intensity_basis`.** Rows calculated earlier stay NULL: their origin
   is not guessed. The summary reports the carbon by source and basis and the share that is a same-day mean.
5. **The client sees the history**: `GET /carbon/intensity` returns, for a region, one chosen source per day with its
   min and max and the other sources beside it; days without readings are absent. Carbon & Energy draws it with gaps as
   gaps, and shows where the figures came from.

## Consequences
- Adding a source is a connector that saves readings with its provider id plus an entry in the priority list's enum.
- A priority change applies to the next calculation; history is re-run with a recalculation.
- Mixed measures or licences can never be blended by accident; combining sources, if ever wanted, is a new, explicit
  methodology version.
