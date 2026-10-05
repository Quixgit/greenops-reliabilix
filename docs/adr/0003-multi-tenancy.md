# ADR-0003 Multi-tenancy via tenant_id + RLS

Status: accepted

Decision: every tenant table carries tenant_id; defense in depth with application-level scoping and PostgreSQL RLS via transaction-local app.tenant_id (pkg/db.WithTenantTx).
