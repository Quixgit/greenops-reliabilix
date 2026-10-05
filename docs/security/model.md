# Security model (baseline: OWASP ASVS 5.0)

- **AuthN**: external OIDC provider (MFA, rotation). Services verify JWT (iss/aud/exp, tenant + role claims). Dev verifier only with `ENV=dev`; production fails closed.
- **AuthZ**: RBAC in `pkg/auth` (owner/admin/engineer/viewer/billing), enforced per route with `auth.Require(perm)`; server-side only.
- **Tenant isolation**: `tenant_id` from token; repositories take tenant explicitly; PostgreSQL RLS (`FORCE ROW LEVEL SECURITY`) bound via `app.tenant_id`. The app DB role must not be a superuser/owner (owners bypass RLS unless FORCE; superusers always bypass).
- **HTTP**: strict timeouts, 1 MiB body cap, strict JSON decoding, RFC 9457 errors without internals, security headers, request ids, no header/body logging.
- **Secrets**: never in DB, repo or frontend. Cloud access via AWS IAM role + per-tenant External ID (Azure/GCP workload identity), least privilege read-only billing scopes. Secret manager references only.
- **Audit**: append-only `audit_logs` for logins, role changes, cloud connections, approvals, automation runs.
- **Automation**: never changes infrastructure without policy check + explicit human approval + rollback plan.
- **Supply chain / CI**: govulncheck, golangci-lint (gosec), gitleaks, trivy, distroless nonroot images.
- **TODO before production**: rate limiting at gateway/WAF, mTLS between services, CSP nonces, SBOM + image signing, pen test.
