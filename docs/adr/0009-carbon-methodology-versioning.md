# ADR-0009 Versioned carbon methodology; SCI is never guessed

Status: accepted

Decision: coefficients are immutable versioned data (platform/methodology/rlx-provisional-1.json); every calculation stores methodology_version; a change creates a new version and never rewrites history. Calculation is separate from collection. Energy comes from measured usage when available (usage_based) and otherwise from spend (cost_based, low confidence); the method is stored per row. SCI = ((E x I) + M) / R: M is 0 in the current version (operational only); R (functional units) is reported by the customer per project and period, and SCI stays NULL where R is unknown. Rows the engine cannot attribute (global services, regions without grid data, non-USD cost-based rows) are skipped and counted, not estimated. The current coefficient set is marked provisional and is exposed at GET /carbon/methodology and in every report.


## Addendum: provenance, and why the version is not called CCF

The first set was labelled `CCF-2026.1`. That was misleading: the values were chosen by the platform authors (several
recalled from Cloud Carbon Footprint's published constants, none imported or verified against the published datasets,
and the cost-based values have no CCF counterpart at all). A version label must never imply more provenance than
exists, so the set was renamed `RLX-PROVISIONAL-1` and now carries `provenance: author_estimate_unverified` and a list of
`caveats` that the API returns with every carbon summary. The rename also fixed a units error found while auditing the
file: the storage coefficient was a per-terabyte-month figure used as per gigabyte-month (1000 times too high).
A loader check refuses a provisional set without provenance and caveats.

A citable set (a verified import of the published CCF datasets, with its version and source recorded) must be added as a
new version before any figure is used for external reporting.
