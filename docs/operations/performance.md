# Performance: what the API sustains, how it was measured, what was fixed

Figures for the read side of the API (the Overview, the cost and carbon pages) against a realistic volume of data. Measured on 2026-10-12
with `0.1.0-alpha.165` (the cache measurement with `0.1.0-alpha.168`). They are a baseline to compare against after a change, not a promise to a customer: see "What these numbers are not".

## Method

- **Data** (`scripts/loadtest/seed.sql`, throwaway PostgreSQL 18 database with every migration applied): 21 organizations, **3.0 million usage rows**
  and 2.4 million carbon rows over 365 days. The **large organization** has 2,740 resources billed daily (**1.0 million usage rows a year**), 20 projects,
  12 regions, 8 services; the other twenty have a tenth of that. Random values, realistic sizes. Database 2.3 GB.
- **Load** (`scripts/loadtest/main.go`): closed loop, N concurrent clients each asking the next question when the last answer arrived, 8 seconds per
  endpoint and level, against the three-month window July to September. Read only. Levels 1, 10 and 50 clients; the last row of a level mixes all endpoints.
- **Machine**: one host with 6 cores and 15 GB, shared with the stand's other services, Redis and the load generator itself. One API process, one PostgreSQL.
  Development sign-in, rate limit raised so it does not interfere.
- **Check of correctness**: before and after the change below, 16 answers of the large and a small organization were compared byte for byte; and 11
  answers of an organization with an EUR reporting currency, USD and GBP rows and days without an exchange rate (the conversion path) were compared
  between the old and the new build. All identical.

## What was found and fixed

The first measurement on this data found that cost queries were slow in proportion to the rows read, not the rows returned: the three-month summary of the
large organization took **14.4 s**. The cause was twofold: the reporting currency (two settings lookups) was computed for every row, and every row went through
the currency conversion function even when it was already in the reporting currency. Now the reporting currency is computed once per query (a CTE) and
the conversion function is called only for rows in another currency. Nothing else changed: no migration, same results.

| Call (large organization, 3 months, one client) | Before | After |
|---|---:|---:|
| `GET /finops/summary` (Overview totals) | 14.4 s | 1.0 s |
| `GET /dashboard/services` | 6.8 s | 0.4 s |
| `GET /dashboard/trend` | 6.6 s | 0.5 s |
| `GET /finops/forecast` | 2.1 s | 0.2 s |
| `GET /finops/anomalies` | 2.0 s | 0.12 s |
| same, a small organization (100,000 rows a year) | 0.2 to 1.5 s | 0.03 to 0.13 s |

## Results (large organization)

Two measurements of the same build and data: **without** the answer cache (every request reaches the database) and **with** it (the default, 30 s, ADR-0060).
"Overview page" is a person opening the Overview: its seven widgets asked at once; the time is the slowest of them.

### Every request reaches the database (cold work)

| Endpoint | 1 client: p50 / p95 | 10 clients: p50 / p95 | 50 clients: p50 / p95 | Best throughput |
|---|---|---|---|---|
| `projects`, `recommendations` (small reads) | 1.1 ms / 2.4 ms | 3 ms / 5 ms | 15 ms / 20 ms | about 3,000 req/s |
| `carbon/trend` | 142 ms / 167 ms | 273 ms / 364 ms | 1.5 s / 2.3 s | 36 req/s |
| `finops/forecast` | 233 ms / 256 ms | 453 ms / 547 ms | 2.3 s / 2.5 s | 22 req/s |
| `dashboard/services` | 655 ms / 826 ms | 1.4 s / 2.0 s | 6.3 s / 6.9 s | 7 req/s |
| `carbon/summary` | 656 ms / 703 ms | 1.2 s / 1.8 s | 6.0 s / 6.5 s | 8 req/s |
| `finops/summary` | 960 ms / 1.1 s | 2.0 s / 2.3 s | 9.4 s / 10.6 s | 5 req/s |
| **Overview page** | **1.0 s** / 1.2 s | **6.2 s** / 7.6 s | **30 s timeout, all 50 failed** | 2 pages/s |

(Single requests made by hand on a quiet machine gave 0.27 s for `carbon/summary`; the table shows what the test measured.)

### With the answer cache (the default)

| Endpoint | 1 client: p50 / p95 | 10 clients: p50 / p95 | 50 clients: p50 / p95 | Best throughput |
|---|---|---|---|---|
| the cached analytic endpoints | 0.2 ms / 0.4 ms | 0.4 ms / 1.4 ms | 1.6 to 3.1 ms / 6 to 11 ms | 12,000 to 20,000 req/s |
| mix of all | 0.3 ms / 1.6 ms | 0.7 ms / 4.2 ms | 0.9 ms / 24 ms | 6,700 req/s |
| **Overview page** | **1.5 ms** / 2.9 ms | **5.6 ms** / 9.8 ms | **29 ms** / 42 ms, none failed | 1,500 pages/s |

The cold work did not get cheaper: the slowest single request of each cached run (0.4 to 0.9 s) is the first one after the 30 seconds, and it is the same database work as in the first table.
What changed is that the work is done once per organization and query instead of once per person. No request failed in either measurement except the 50-person page load without the cache.
No request is answered with another organization's data: the key includes the organization of the verified identity (a unit test and a check on the stand).

## What this means

- The first time an organization of this size opens the Overview, it needs about a second of database time per widget; with the cache, everyone else who asks the same within 30 seconds, and every
  refresh, costs a millisecond. Many people looking at one organization is no longer a problem; **many different organizations each opening a large dashboard at the same moment is**: each one
  pays the cold cost once. That is the case the cache does not help, and the first table is what to plan for.
- The limit of the cold work is the database CPU reading and summing rows. The next steps, in order of effect: a daily roll-up table kept by the ingestion (a read of hundreds of rows instead of
  hundreds of thousands), then a read replica for the dashboards. Neither is built.
- The cache keeps an answer for up to 30 seconds, so a number on a dashboard can lag a sync by that long. What a person changes and then looks at (connections, recommendations, functional units,
  budgets, commitments) is not cached.

## What these numbers are not

- Not a capacity plan: one host, a shared CPU, a generator on the same machine, random data. A production database on its own hardware will be faster; a larger customer will be slower.
- Not the write side: ingestion, carbon calculation, sync and report generation were not measured.
- Not a test of many organizations at once: the load was on one large organization (the other twenty are only there for the size of the tables and the index).
- Not a test over time: 8 seconds per cell, no soak, no failover. The backup and recovery targets are in [backup-and-recovery.md](backup-and-recovery.md), checked by `scripts/backup-restore-check.sh`.

## Repeat it

```bash
docker run -d --name rlx-load-pg -e POSTGRES_PASSWORD=load -p 127.0.0.1:55433:5432 postgres:18 -c max_connections=200 -c shared_buffers=512MB
# apply every migration (the Up part of backend/migrations/*.sql), set passwords for greenops_api and greenops_worker, then:
psql ... -d greenops_load -f scripts/loadtest/seed.sql                  # about 10 minutes
ENV=dev HTTP_ADDR=127.0.0.1:38460 DATABASE_URL=postgres://greenops_api:...@127.0.0.1:55433/greenops_load?sslmode=disable \
  RATE_PER_SEC=1000000 RATE_BURST=1000000 ... backend/bin/api
go run scripts/loadtest/main.go -base http://127.0.0.1:38460 -token dev:load:00000000-0000-4000-8000-000000000001:owner -duration 8s -conc 1,10,50 -page   # READ_CACHE_SECONDS=0 for the cold table
```
