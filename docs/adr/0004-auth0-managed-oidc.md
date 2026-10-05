# ADR-0004 Auth0 as managed OIDC; the database owns tenancy

Status: accepted

Decision: no home-grown authentication. Auth0 issues RS256 access tokens; the API verifies signature (cached JWKS), iss, aud, exp, nbf and rejects HS256/none. The token proves who the user is (sub, and the namespaced email / email_verified claims set by an Auth0 Action); it does NOT carry tenant or role. Tenant and role come from tenants.memberships via a SECURITY DEFINER lookup, so a role change or removal takes effect immediately and a stale token cannot keep privileges. New identities onboard themselves (POST /onboarding) or join through a one-time, email-bound invitation. Machine access uses hashed API keys (grk_...) with the minimal role ci. Self-hosted Keycloak/Zitadel only if a vendor-independence or compliance trigger appears.
