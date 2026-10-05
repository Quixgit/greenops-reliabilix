# ADR-0002 One PostgreSQL, schema per domain

Status: accepted

Decision: PostgreSQL 18, one database, one schema per domain (tenants, projects, cloudaccounts, usage, carbon, finops, recommendations, reports, audit). Keeps transactions and joins where natural. usage_records and carbon.calculations are range-partitioned by month from day one (partitions ensured by a scheduled job). Migrations: goose. pgx for access.
