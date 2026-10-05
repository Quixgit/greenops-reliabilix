# Security model (baseline: OWASP ASVS 5.0)

- **AuthN**: Auth0 (OIDC, MFA there). The API verifies RS256 JWTs itself: JWKS cache with rotation handling, `iss`, `aud`, `exp`, `nbf`; `none`/HS256 rejected. Outside `ENV=dev` the dev verifier cannot be constructed.
- **Browser never holds tokens**: the Next.js session cookie is httpOnly; calls go browser -> `/api/proxy/*` -> API with the access token attached server-side. Only GET/POST under `/api/v1` are reachable.
- **AuthZ**: RBAC (owner/admin/engineer/viewer/billing) enforced per route (`auth.Require`), server-side. UI role checks are hints only.
- **Tenant isolation**: token tenant -> repository argument -> `app.tenant_id` -> PostgreSQL RLS (`FORCE ROW LEVEL SECURITY` on every tenant table; integration test fails if one is missing). Missing context returns zero rows (fail closed).
- **DB roles**: `greenops_api` / `greenops_worker` own nothing and have no BYPASSRLS; audit is INSERT/SELECT only. Worker fan-out uses one narrow SECURITY DEFINER function owned by a NOLOGIN BYPASSRLS role.
- **HTTP**: timeouts, 1 MiB body cap, strict JSON, RFC 9457 errors without internals, security headers, per-IP rate limit, exact-match CORS, request ids, no header/body logging.
- **Secrets**: never in DB/repo/frontend. Cloud access by IAM role + External ID (Azure/GCP workload identity); `credential_ref` is a reference and secret-looking values are rejected.
- **Audit**: written in the same transaction as the change; Recent Activity shows an allowlist only.
- **Automation**: `Plan.CanExecute` refuses without human approval and rollback plan.
- **TODO before production**: CSP nonces (currently `unsafe-inline` for Next bootstrap), Redis-backed rate limits for multi-replica, SBOM/image signing, live Auth0 end-to-end test, pen test.
