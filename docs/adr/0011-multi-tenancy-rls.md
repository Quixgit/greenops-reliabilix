# ADR-0011 Multi-tenancy: explicit tenant argument + Row Level Security

Status: accepted

Decision: every tenant table carries tenant_id (organizations.id is the tenant) and has ENABLE + FORCE ROW LEVEL SECURITY keyed on the transaction-local setting app.tenant_id, which database.WithTenantTx sets for every query. Repositories also take tenantID explicitly, and the tenant always comes from the verified identity, never from request input. A missing setting yields zero rows. An integration test fails if any tenant table lacks RLS, and the end-to-end test proves cross-tenant reads and writes fail.
