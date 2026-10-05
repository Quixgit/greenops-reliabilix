# ADR-0004 Auth0 as managed OIDC

Status: accepted

Decision: no home-grown authentication. Auth0 issues tokens carrying tenant_id and role claims (namespaced). The API verifies tokens itself; the Next.js app keeps the session server-side and proxies API calls (ADR-0008). Self-hosted Keycloak/Zitadel only if a vendor-independence or compliance trigger appears.
