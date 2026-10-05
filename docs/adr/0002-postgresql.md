# ADR-0002 PostgreSQL, database per service

Status: accepted

Decision: PostgreSQL 18, logical DB per service on a shared cluster initially, pgx + sqlc, goose migrations, time-partitioned usage and carbon tables. Redis is never a source of truth.
