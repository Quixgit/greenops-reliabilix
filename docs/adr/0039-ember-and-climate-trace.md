# ADR-0039: A free second grid source (Ember), Climate TRACE evaluated, and monthly resolution

Status: accepted · 0.1.0-alpha.35

## Context
The platform asked for a free source of grid carbon intensity as an alternative to, or an aggregate with, Electricity Maps.
Climate TRACE was the candidate named.

## What was checked
- **Climate TRACE** is an emissions *inventory* (tonnes by sector, country and asset). Its public API is a beta with no
  availability guarantee; the aggregated country endpoint returned **annual totals in tonnes with no electricity generation
  and no intensity field**. Without generation (MWh) no gCO2e/kWh can be derived. It was **not** connected as an intensity
  source and is listed in the registry as evaluated. It may later supply power-sector totals as context.
- **Ember** publishes monthly national carbon intensity (`/v1/carbon-intensity/monthly`, gCO2/kWh per country) free with
  a key, CC-BY 4.0. It is coarse (country, month, about two months' delay) but real and free.

## Decision
1. Ember is connected as a source of **monthly national averages**. A reading has a `resolution` (`hourly` | `monthly`);
   monthly figures are never counted as "readings of the day".
2. Resolution order for a day and region (never mixing sources, ADR-0035): enough hourly readings of the first trusted
   source (`day_mean`) → a published monthly figure (`month_mean`) → fewer hourly readings (`partial_day_mean`) → last
   reading before (`latest_before`) → latest of all (`latest_overall`) → live. A month figure ranks above a thin day because it
   describes the right period.
3. The default priority is Electricity Maps then Ember; with no Ember key nothing changes.
4. The monthly figures are refreshed daily for the regions in use (36 months, revised values replace stored ones), through
   the same request cap and metering as other paid APIs.
5. The regions table in the console shows, for every region in use, what each source has, which source is used now, and a
   selector to pin one for that region (replacing the free-text list).
6. Attribution: where Ember figures appear, "Ember (CC-BY 4.0)" is shown.

## Consequences
- A region whose country Ember does not cover simply has no monthly figure. Country matching uses the zone map.
- Monthly national averages are not zone-specific: they are a fallback, and are labelled so in every view and report row.

## Update (0.1.0-alpha.35): UK Carbon Intensity
The UK Carbon Intensity API (National Grid ESO) is connected: free, no key, half-hourly measured values for Great Britain
(only the `actual` value is stored; a forecast never is). It covers the cloud regions whose zone is GB, is refreshed with the
hourly grid refresh and loads its past through the same backfill, and sits second in the default order (Electricity Maps,
UK Carbon Intensity, Ember). A region it does not cover is simply not asked.

## WattTime (not connected as an intensity source)
WattTime's free plan gives all signals for one region (CAISO_NORTH) plus percentiles for the rest. Its main signal, MOER, is
*marginal* intensity: the emissions of the generator that would respond to a change in load. That is a different
quantity from the average intensity used for inventory figures and must not be mixed with it (ADR-0035). It becomes useful
for carbon-aware scheduling ("when is the best hour to run this job"), which is a separate feature; the connector comes
with it, stored with `measure = marginal`.
