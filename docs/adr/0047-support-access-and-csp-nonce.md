# ADR-0047: Time-limited support access, and a Content-Security-Policy with a nonce

Status: accepted · 0.1.0-alpha.144

## Context
ADR-0020 keeps the operator console metadata-only: counts, never an organization's working detail. Support work needs more
for a short time (which project, which connection, what failed). The organization must decide that, and see it.

## Decision: support access
1. **The organization opens it**, not the operator: an owner or admin chooses 1 to 168 hours and may write why. A grant still open is
   replaced; it can be closed at any moment; it is never open-ended (a CHECK of one week).
2. **Without a grant the detail does not exist**: `platform.operator_support_overview` raises without an open grant (403 to the
   console), requires an admin operator, and records the look in `tenants.support_views` before answering.
3. **What it returns** is read only and bounded: project names, functional units and allowed regions; connections with provider,
   account reference, status and a truncated last error; counts of budgets and open recommendations; the last 30 audit entries
   (action and target, no metadata). No usage or cost rows, no secrets, no reports.
4. **The organization reads the record**: grants (and who opened them) and every look (operator, what, when) are in Settings → Security,
   and grant and revoke are in its own audit log.
5. Erasure of an organization removes both tables with it.

## Decision: CSP nonce
`proxy.ts` sets the CSP per request with a nonce and `'strict-dynamic'`; Next puts the nonce on its scripts (it reads it from the CSP
of the request). `script-src` no longer allows `'unsafe-inline'`. The Auth0 path keeps the old policy: the SDK builds its own
response and does not forward a changed request, so the nonce cannot reach the page. It is to be verified against a real tenant
before it is switched; the sign-in used on the stand and in development is covered (16 routes, no violation).

## Consequences
- Support access is a decision per incident, visible to the customer; nothing is standing.
- Moving the Auth0 path to the nonce policy is the remaining step of the CSP work.
