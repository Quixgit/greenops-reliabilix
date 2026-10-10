# ADR-0040: Citable operational coefficients (RLX-CCF-1)

Status: accepted · 0.1.0-alpha.35 · supersedes the numbers of RLX-PROVISIONAL-1 as the active set

## Context
RLX-PROVISIONAL-1 was chosen by the platform authors; several values were recalled from Cloud Carbon Footprint (CCF) but
were not verified against it. That blocked using any figure for external reporting.

## Decision
`RLX-CCF-1` (new version, the old file is untouched and still available for comparison) takes its usage-based
coefficients from CCF's published AWS constants, read from the source file and the methodology page on 2026-10-10:

| Quantity | CCF constant | Value in the set |
|---|---|---|
| per vCPU-hour | MIN_WATTS_AVG 0.74 W, MAX_WATTS_AVG 3.5 W, default utilisation 50% | 0.74 + 0.5 x (3.5 - 0.74) = 2.12 W = 0.00212 kWh |
| per GB-hour of memory | MEMORY_COEFFICIENT | 0.000392 kWh |
| per GB-month of storage | HDDCOEFFICIENT 0.65 Wh per TB-hour | 0.65e-6 kWh x 730 h = 0.0004745 kWh (SSD 1.2 Wh per TB-hour would be 0.000876) |
| PUE | PUE_AVG | 1.135 |

The file's notes cite the file path, the constant names and the derivation; its checksum is pinned by a test.

## What is still not sourced
- **Cost-based factors** (kWh per USD) have no CCF counterpart: they stay the authors' estimates, labelled "not sourced".
  The set therefore stays `provisional` and the report footers keep saying so.
- **Embodied emissions (M)**: CCF derives them per instance type from Teads research; the methodology page gives no
  numbers and no dataset was imported. `embodied_g_per_kwh` stays 0 and every figure says it is operational only.
- Averages are used for every instance type and cloud (a per-instance-family CPU model is a later refinement).

## Consequences
- The active version changes; stored results of RLX-PROVISIONAL-1 stay (history is never rewritten) but dashboards read the
  active version, so a recalculation is needed: Data sources → "Recalculate carbon" (and it runs on deploy in the stand).
- The difference for usage-based compute is about +1% (0.00212 against 0.0021 kWh per vCPU-hour).
