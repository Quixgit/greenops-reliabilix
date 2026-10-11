# ADR-0046: Inter-region latency: measured where published, estimated with a stated error elsewhere

Status: accepted · 0.1.0-alpha.119

## Context
Region-move recommendations, the map and the Move check showed a round trip computed from distance alone (light in fibre,
1.5x route factor). It was labelled an estimate, but it was off in both directions, and it decided which moves are
proposed (setting `recommendations.max_latency_ms`). Customers will not install an agent to measure, so the platform has
to be honest with what is published.

## Decision
1. Source order for a pair of regions: **Microsoft's published inter-region round-trip table** (Azure pairs, 51 regions),
   then **CloudPing's** public matrix (AWS pairs, 26 regions, median over a year), then **an estimate**: a line fitted to
   the 3,200 measured pairs (6 ms + 0.0156 ms per km). When two directions are published the larger is used.
2. Every figure carries its basis (`measured_azure`, `measured_aws`, `estimated`) and an estimate its typical error
   (`latency_error_pct`, the 80th percentile of the fit's error: 24%). The API, the recommendation `details`, the map and
   the Move check show it; a measured figure is never written with "≈".
3. Data lives in `backend/internal/platform/geo/latency.json`, copied to `frontend/lib/latency.json` by
   `scripts/gen-latency.py`; the frontend repeats the lookup of `geo.Between` (kept equal by tests on both sides).
4. No measurement is invented for Google Cloud or between clouds: they are estimates, and say so.

## Consequences
- Some moves disappear at the default 60 ms limit (us-east-1 to us-west-2 is measured at 68 ms, the estimate was about 60).
- CloudPing's matrix is the site's own endpoint, not a documented API: the generator may break if they change it. The
  committed data keep working; refreshing is a manual step.
- Platform to region probes were considered and rejected as a basis: they measure the platform, not region to region.
- Latency is region to region, not from the customer's users; the console says to check it for user-facing traffic.
