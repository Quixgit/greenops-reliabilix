# ADR-0002 One PostgreSQL, one schema per domain

Status: accepted

Decision: PostgreSQL 18, one database, schemas tenants, projects, cloudaccounts, usage, carbon, finops, recommendations, reports, audit (+ platform helpers). Keeps transactions and joins where natural; no distributed consistency before it is needed. usage.usage_records and carbon.calculations are range-partitioned by month from day one (partitions ensured by the scheduler and before every write: a partition cannot be created once DEFAULT holds rows for its range).

Access is SQL-first: queries live in repository/queries/*.sql and sqlc generates typed pgx code (parameterized by construction). CI fails if generated code is stale. Migrations: goose. Three runtime roles: greenops_api, greenops_worker (no BYPASSRLS, own nothing) and the NOLOGIN greenops_fanout that owns a few narrow SECURITY DEFINER functions (membership resolution, API-key lookup, job fan-out). The API role can read but never write usage and carbon.
