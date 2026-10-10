# ADR-0020: Platform operator console: a separate role, metadata only, every access recorded

Status: accepted

## Context
The people who run the platform need to see what is wrong across organizations: failing deliveries, connections in
error, the state of every integration. Tenant roles (owner, admin ...) are about one organization and must not grow
cross-tenant powers; and RLS exists precisely so that nothing can read across tenants by accident.

## Decision
- **Operators are a database fact.** `platform.operators(auth_sub, role)` with roles `viewer` (read) and `admin`
  (also retry/disable/resolve). Only the migration owner can write it (`make operator-add SUB=... ROLE=...`); the api and
  worker roles have no grant on it, so a compromised process cannot appoint anyone. As with tenant roles, the token
  proves identity only. API keys never reach the console (identity authentication rejects them).
- **Cross-tenant access is a handful of SECURITY DEFINER functions** (owner `greenops_fanout`, BYPASSRLS) that return
  metadata only: counts, statuses, error texts, masked names. Never payloads, destination addresses, tokens, usage or
  carbon data, report files. EXECUTE is granted to the api role only; the worker cannot call them.
- **Everything is recorded.** Every successful console request (reads included) is written to the append-only
  `platform.operator_access_log` before data is returned; if the record cannot be written, nothing is shown. Actions
  (retry a delivery, pause a destination) also land in the affected organization's own audit log as
  `operator.*` attributed to `operator:<sub>`, so customers can see them.
- **API** under `/api/v1/operator/*` (organizations, notification stats and deliveries, unhealthy connections, system
  and integrations, error log, plus the actions above). Non-operators get 403. **UI** at `/operator`: the layout asks the
  API who the caller is and answers 404 to everyone else; the link in the sidebar appears only for operators.
- **Stubs are labelled.** `GET /operator/system` lists every external integration as `ready`, `stub` (a development
  stand-in runs) or `missing`, so a missing key or a paid API not yet connected is visible, never silently faked.

## Consequences
- Operators should sign in with MFA (enforce it in Auth0); this ADR does not replace that.
- Not built: viewing the access log in the UI, impersonation (deliberately never), per-operator permissions finer than
  viewer/admin, suspending an organization.


> Update (alpha.25): operator roles can be assigned and withdrawn from the console through two guarded database functions; see [ADR-0034](0034-operator-roles-from-the-console.md).
