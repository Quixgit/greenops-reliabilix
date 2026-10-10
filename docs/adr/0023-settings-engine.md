# ADR-0023: One settings engine for the platform, organizations and operators

Status: accepted

## Context
A serious multi-tenant product needs many knobs: for customers (their policies, formats, notifications), and for the
people who run the platform (limits, enabled modules, maintenance, retention, sign-up rules). Hand-building a table,
endpoint and form for each would be slow and would drift. Settings must also be safe: validated, bounded, audited, and
with a clear rule about who may change what.

## Decision
**A catalog in code is the single source of truth** (`platform/settings`). One `Def` per setting: key, group, title,
description, scope, type (`bool`, `int`, `float`, `string`, `enum`, `list`), default, bounds (min/max, enum values,
allowed list items, pattern, length), unit and a note on how it is applied. A broken catalog fails at startup and in tests
(duplicate keys, defaults that do not validate). Storage, validation, the API and the admin screens are all generated from
it: adding a setting is one `Def` plus the code that honours it.

**Three scopes decide who may change a value.**
- *platform*: the whole deployment; operators only (maintenance mode, sign-up policy, delivery retries, retention).
- *tenant*: an organization's own choices; its owners and admins change them (invitation lifetime and domains, API key
  limit, report formats, notifications on/off, minimum interval between data pulls).
- *operator*: per organization, operator only; the organization sees it read-only (limits, enabled modules, allowed cloud
  providers).

**Inheritance:** an organization's effective value is its own, else the platform value, else the catalog default. For
tenant-scope keys the platform value is therefore *the default every organization inherits*. An operator can **override a
value for one organization, which locks it**: the organization cannot change or reset it (enforced in the service and again by
the database: restrictive row-level policies reject any organization-session write to an operator-sourced row). An override
replaces the organization's own value; unlocking returns it to inheriting.

**Storage** (`settings` schema): `platform_values` (readable by api and worker, written only by operator functions),
`tenant_values` (RLS; `source` is `tenant` or `operator`), `history` (append-only; organization rows visible to the
organization, platform rows only to operators). Operator writes are SECURITY DEFINER functions that check the operator is an
`admin`, record the history, and, for organizations, also write the organization's audit log (`operator.setting_changed`).
Values are validated against the catalog before they are stored; a stored value that no longer validates is ignored and the
default applies. Secrets are never settings (they stay in the environment, ADR-0010).

**Reading** is by the `settings.Reader` port: `Bool/Int/String/Strings(ctx, tenant, key)`, cached for 10 s per process and
invalidated on writes from the same process (so a change from another process shows up within 10 s). Reading never fails a
request: any error yields the catalog default.

**Enforcement is real or the setting does not exist.** Every setting in the catalog is honoured by code today:
- request gates (`app/gates.go`): maintenance mode (writes answer 503, reads and the operator console keep working, CI
  evaluation is exempt), module switches (403 for that organization), sign-up on/off and e-mail domain allow-list;
- `entitlements.Gate` port with the `limits.*` settings (projects, cloud connections, members including open invitations,
  notification destinations; 0 = unlimited; refusal is HTTP 402; existing resources are never removed);
- allowed cloud providers, minimum hours between data pulls, invitation lifetime and domains, active API key limit, allowed
  report formats, notification on/off, delivery attempts / retry delay / send timeout, retention of deliveries, handled errors
  and the operator access log (never below 90 days, enforced in the database).

**API and UI.** Organizations: `GET/PATCH /settings`, `POST /settings/reset`, `GET /settings/history` (permissions
`settings:read` for owner, admin, engineer; `settings:write` for owner, admin). Operators: `/operator/settings` (platform),
`/operator/tenants/{id}/settings` (per organization), history. A rejected change is a 422 naming each bad key; one bad value
rejects the whole request. One generic editor renders any of them from the catalog description: grouped, searchable, typed
controls, source badges, locks, dirty tracking, reset/unlock, history.

## Consequences
- Plans (ADR-0022) become presets of `limits.*` and `modules.*` written per organization; the licensing module needs no new
  enforcement code.
- Not built yet: role/permission matrix editing per organization (the RBAC matrix is still code), per-user preferences,
  branding, scheduled changes, import/export of settings, approval workflow for sensitive platform changes.
- The platform's own process settings that need a restart (HTTP rate limit, worker concurrency) are still environment
  variables; they are deliberately not in the catalog until they can take effect without a restart.
