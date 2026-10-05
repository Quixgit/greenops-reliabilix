# ADR-0006 Multi-tenancy: tenant_id + RLS

Status: accepted

Decision: every tenant table has tenant_id and FORCE RLS keyed on the transaction-local app.tenant_id, set by database.WithTenantTx. Application code also passes tenantID explicitly. An integration test asserts RLS on all tenant tables.
