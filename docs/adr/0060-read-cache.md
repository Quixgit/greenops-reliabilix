# ADR-0060: A short answer cache for the heavy dashboard reads

Status: accepted

## Context
The second load measurement ([performance.md](../operations/performance.md)) added a person opening the Overview: seven widgets at once. For one large organization it took 1.0 s alone, 6.2 s with ten people and
did not finish in 30 s with fifty (all failed). The widgets answer questions over a year of line items that every person of the organization and every refresh asks, nearly identically, so the same
database work was repeated per person.

## Decision
`platform/readcache`: a middleware on the heavy read endpoints that depend only on ingested data (dashboard trend, providers, services, regions, countries and footprint; finops summary, anomalies and forecast;
carbon summary and trend). It keeps a successful answer for `READ_CACHE_SECONDS` (default 30, 0 turns it off) and makes concurrent identical requests share one run (`singleflight`).
- **Isolation**: the key is the organization of the verified identity plus the full request address; the middleware sits behind authorization, never serves an unauthenticated call, and keeps only GET answers
  with status 200 up to 2 MB, at most 512 entries (expired ones first). A unit test asks for two organizations and the same address; the stand check did too.
- **Freshness**: an answer may lag the data by up to the ttl. Anything a person changes and then looks at (connections and their sync state, recommendations, functional units, budgets, commitments) is not
  cached; a response header `X-Cache: hit|miss|shared` says what happened.
- The cache is per process (the API runs as one process today; with several replicas each keeps its own, which only costs an extra run per replica).

## Consequences
Overview page, 50 people on one large organization: 30 s and failures to 29 ms; 10 people: 6.2 s to 5.6 ms. The cold cost (about a second per widget for this size) is unchanged and is paid once per
organization and query per ttl: many different organizations opening large dashboards together still pay it, which is what a daily roll-up table would fix and is not built.
