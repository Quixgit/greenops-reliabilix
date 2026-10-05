# ADR-0008 External identity provider

Status: accepted

Decision: Zitadel or Keycloak (Auth0 acceptable for MVP) via OIDC; no home-grown auth. Services verify tokens themselves; tenant and role arrive as claims or are resolved from memberships.
