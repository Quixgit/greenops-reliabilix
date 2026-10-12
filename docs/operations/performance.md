# Performance: what the API sustains, how it was measured, what was fixed

Figures for the read side of the API (the Overview, the cost and carbon pages) against a realistic volume of data. Measured on 2026-10-12
with `0.1.0-alpha.165`. They are a baseline to compare against after a change, not a promise to a customer: see "What these numbers are not".

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

## Results after the change (large organization)

| Endpoint | 1 client: p50 / p95 | 10 clients: p50 / p95 | 50 clients: p50 / p95 | Best throughput |
|---|---|---|---|---|
| `projects`, `recommendations` (small reads) | 1.0 ms / 1.8 ms | 3 ms / 5 ms | 15 ms / 24 ms | about 3,400 req/s |
| `carbon/trend` | 140 ms / 177 ms | 300 ms / 395 ms | 1.5 s / 2.2 s | 33 req/s |
| `finops/forecast` | 242 ms / 279 ms | 503 ms / 727 ms | 2.4 s / 2.6 s | 20 req/s |
| `dashboard/services` | 737 ms / 812 ms | 1.3 s / 1.6 s | 6.7 s / 7.3 s | 8 req/s |
| `carbon/summary` | 646 ms / 725 ms | 1.4 s / 1.9 s | 6.0 s / 6.3 s | 8 req/s |
| `finops/summary` | 1.05 s / 1.2 s | 2.2 s / 2.5 s | 9.6 s / 11.2 s | 5 req/s |
| mix of all | 225 ms / 1.1 s | 532 ms / 1.7 s | 3.7 s / 9.0 s | 12 req/s |

No request failed at any level. Small reads are limited by the machine and keep their latency; the analytical endpoints are limited by the database and do not scale
past a few requests a second on this host: more clients only queue.

## What this means

- One large organization (a million line items a year) opening the Overview needs about a second of database time per widget. Ten such people at once are served in 2 to 3 seconds
  each. This is fine for the product's usage (a few people looking at a page now and then); it is not fine for an integration that polls the Overview every second.
- The limit is the database CPU reading and summing rows. The next steps, in order of effect: a daily roll-up table kept by the ingestion (a read of hundreds of rows instead of
  hundreds of thousands), then a read replica for the dashboards. Neither is built. `carbon/summary` (0.65 s) and `carbon/trend` were not optimized in this round.
- A dashboard's several widgets run as separate requests, so a page costs the sum of them in database time.

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
go run scripts/loadtest/main.go -base http://127.0.0.1:38460 -token dev:load:00000000-0000-4000-8000-000000000001:owner -duration 8s -conc 1,10,50
```
