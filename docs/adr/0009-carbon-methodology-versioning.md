# ADR-0009 Versioned carbon methodology; SCI is never guessed

Status: accepted

Decision: coefficients are immutable versioned data (platform/methodology/ccf-2026.1.json); every calculation stores methodology_version; a change creates a new version and never rewrites history. Calculation is separate from collection. Energy comes from measured usage when available (usage_based) and otherwise from spend (cost_based, low confidence); the method is stored per row. SCI = ((E x I) + M) / R: M is 0 in 2026.1 (operational only); R (functional units) is reported by the customer per project and period, and SCI stays NULL where R is unknown. Rows the engine cannot attribute (global services, regions without grid data, non-USD cost-based rows) are skipped and counted, not estimated. The current coefficient set is marked provisional and is exposed at GET /carbon/methodology and in every report.
