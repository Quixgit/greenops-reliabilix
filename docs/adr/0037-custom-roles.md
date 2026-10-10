# ADR-0037: Custom roles per organization

Status: accepted · 0.1.0-alpha.33

## Context
The five built-in roles (owner, admin, engineer, billing, viewer) cannot express "may read carbon and cost but not
reports" or "may approve recommendations but not manage people". The product owner asked for flexible roles that can be
assigned and withdrawn when needed.

## Decision
- An organization defines **custom roles**: a name, a description and a set of permissions from the catalog
  (`auth.Permissions()`). Built-in roles stay and are read-only.
- A member holds **either** a built-in role **or** one custom role. The membership keeps its built-in role as the fallback;
  deleting a custom role returns its members to it (they are never locked out by a deletion).
- Resolution: `tenants.resolve_membership` answers `custom` for such a member and the API loads the role's set
  (`tenants.membership_permissions`) into the request claims. `Claims.Can` is used by every check; a custom member whose
  set cannot be loaded holds nothing (fails closed).
- **Guard rails**, enforced in the domain and covered by tests:
  1. `tenant:admin`, `tenant:data` and `kubernetes:ingest` (organization administration, export/erasure, machine access)
     can never be in a custom role, so a custom role cannot be a path to managing people or exporting everything;
  2. a role can only contain permissions the **creator holds**, and only someone who holds all of a role's permissions
     can edit it or assign it (no granting what you do not have);
  3. owners are never given a custom role (that is a demotion that stays under the role-change and last-owner rules);
  4. names are unique per organization, case-insensitively.
- Operators are unaffected: platform operator roles are separate (ADR-0020, ADR-0034).
- Every create, change, delete and assignment is in the organization's audit log.

## API
`GET /roles` (catalog, built-in roles, custom roles), `POST /roles`, `PUT /roles/{id}`, `DELETE /roles/{id}`;
`PATCH /members/{id}` takes `role` or `custom_role_id`.

## Not done
Invitations carry a built-in role only (assign the custom role after acceptance); per-project role scoping; roles shared
across organizations; the client UI gates by the built-in role (the server decides every request).
