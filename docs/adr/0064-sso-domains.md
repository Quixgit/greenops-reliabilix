# ADR-0064: Company sign-in by e-mail domain

Status: accepted (implemented, migration 0078)

## Context
SAML single sign-on was listed as missing. Sign-in is OpenID Connect at Auth0 (ADR-0004), and Auth0 already speaks SAML to a company's identity provider through an enterprise connection; building a SAML service provider
into the platform would mean issuing our own sessions beside Auth0's and could not be tested without a real identity provider. What a customer still cannot do is tell the platform which sign-in its people use, nor have them
land in the right organization.

## Decision
**Domain claim with proof.** An organization claims an e-mail domain with the name of its Auth0 connection (`tenants.sso_domains`). The claim is inert until the domain's DNS publishes `_reliabilix-sso.<domain>` TXT
`reliabilix-sso=<token>` (a random 128-bit token, checked with a bounded lookup). Only one organization can hold a verified domain (a partial unique index; the second to prove it is refused), because routing someone's sign-in to
another organization's identity provider would let that provider vouch for their address. Public mail domains, IP addresses and single labels are refused; the connection name has the shape Auth0 allows.

**Routing.** `GET /sso/discover?email=` is public (the sign-in page asks before anyone is signed in) and returns the connection of a verified domain, nothing else; the frontend asks it through its own rate-limited route and sends the
address to `/auth/login?connection=...&login_hint=...`, or takes the ordinary sign-in when there is none or the question fails.

**Joining.** A claim may set a `jit_role` (viewer, engineer, billing: never owner or admin). A person with **no membership** who signs in with a **verified** address of a verified domain joins with that role (`claim_sso_domain`, a SECURITY DEFINER
function like invitation acceptance, tried after SCIM provisioning in the authentication middleware). An existing membership is never changed; with no `jit_role`, invitations and SCIM remain the only ways in.

**Not enforced.** The platform does not require SSO for a domain: it cannot see which connection a token came through, and an Auth0 Action or disabling the other connections is the place for that. Removing a domain stops routing and joining; members stay.

## Not done
SAML built into the platform; enforcement of SSO per domain; periodic re-check of the DNS proof; the member limit at an automatic join (as for SCIM); not tested against a real Auth0 enterprise connection or a published DNS record, only the domain, the verification
decision (with a fake resolver; a real lookup of a missing record was refused), the routing and the joining against the stand's database.
