# Company sign-in (SSO) with SAML or OIDC

Reliabilix signs people in through Auth0 ([ADR-0004](../adr/0004-auth0-managed-oidc.md)); SAML is a feature of that identity provider, not of the platform. What the platform adds is the link between your
**e-mail domain** and your **Auth0 enterprise connection**, so that people of your company reach the right sign-in without choosing, and (optionally) join on their own.

## 1. In Auth0

1. Authentication > Enterprise: create a **SAML** (or OpenID Connect, Azure AD, Okta ...) connection to your company's identity provider. Note its **name** (for example `acme-saml`).
2. Enable the connection for the Reliabilix application.
3. Make sure the connection returns the person's e-mail and that Auth0 marks it verified (an enterprise connection normally does); the post-login Action in [docs/api/README.md](../api/README.md) passes it to the API.

## 2. In Reliabilix (owner or administrator)

Settings > Security > Company sign-in:

1. Enter your **e-mail domain** (`acme.com`) and the **connection name**. Public mail domains (gmail.com and the like) are refused.
2. Choose what anyone with a verified address of the domain may do on first sign-in: **must be invited** (the default), or join as viewer, engineer or billing. Owner and administrator can never be given this way.
3. Publish the **TXT record** shown (`_reliabilix-sso.acme.com` with a value `reliabilix-sso=...`) in your DNS and press **Check the record**. DNS changes can take a few minutes. Until the record is verified the domain does nothing.
   Only one organization can hold a verified domain.

## What happens then

- On the sign-in page, a person types their work e-mail; an address of the verified domain is sent straight to the connection (`/auth/login?connection=...&login_hint=...`). Any other address signs in as before.
- A person who signs in with a **verified** address and has no membership yet joins with the domain's role, if you chose one. Otherwise they need an invitation or a SCIM push ([scim.md](scim.md)).
- Nothing here removes the ordinary sign-in (password, Google, GitHub) for the same people: to require SSO, disable the other connections for your application in Auth0 or use an Auth0 Action. The platform does not enforce it.
- Removing the domain stops the routing and the automatic join; existing members stay.

Decision and limits: [ADR-0064](../adr/0064-sso-domains.md).
