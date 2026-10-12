# ADR-0056: SCIM 2.0 user provisioning

Status: accepted (implemented, migration 0073)

## Context
Phase 4 asks for SSO and SCIM. Sign-in is OpenID Connect at the identity provider (ADR-0004), which already carries SAML federation on its side. What a
company cannot do today is have people appear and disappear in the platform from its own directory: members are invited by hand and removed by hand.

## Decision
**Provisioning, not sign-in.** The platform serves SCIM 2.0 (`/api/v1/scim/v2`: ServiceProviderConfig, ResourceTypes, Users with list, filter
`userName eq`, create, read, replace, patch, delete). It authenticates with an API key of the new role `scim`, which holds one permission
(`scim:provision`) and nothing else; the organization is the key's own, so a key can only touch its own people. Keys are made in Settings as
any API key, by whoever manages members.

**A pushed person is not yet a member.** Creating a user records e-mail, name, external id and role (`roles[].value`: viewer, engineer, admin,
billing; default viewer; never owner) in `tenants.scim_users`. The person becomes a member when they first sign in with the same **verified** e-mail:
the authentication middleware, finding no membership, asks `tenants.claim_provisioned` (a SECURITY DEFINER function, like invitation acceptance),
which creates the membership with the pushed role. An unverified e-mail is never used, so no one can take a seat by typing an address.
Somebody who already is a member is linked, not changed. The rules of an invitation hold: valid e-mail, the organization's allowed domains, the member limit.

**Deprovisioning.** `active: false` (Okta's patch without a path or Azure's `"False"` string) or DELETE removes the membership and, for DELETE,
the record; the person's next request finds no membership. Reactivating lets the next sign-in claim again. Role changes flow to the membership
(and clear a custom role). **Owners are never removed or demoted by provisioning**: a deactivation answers 409 `mutability`, a role push is ignored.
Everything is audited (`scim.user_provisioned`, `scim.user_updated`, `scim.user_deprovisioned`, plus `member.joined`, `member.role_changed`,
`member.removed` with `by: scim`).

## Not done
Groups (SCIM `/Groups`), bulk, sort, ETag, filters other than `userName eq`, attribute-level `remove`. The member limit is checked when a person is pushed, not
again at their first sign-in. No plan gating yet (any plan can use it). Not tested against a real Okta or Entra tenant, only against the
protocol (requests of both styles on the stand); SAML sign-in is the identity provider's feature, not built here.
