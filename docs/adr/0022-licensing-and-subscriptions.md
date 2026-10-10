# ADR-0022: Licensing: entitlements inside the monolith now, extractable later

Status: accepted (design; implementation follows)

## Context
Customers will buy subscriptions. The product must know, per organization, which plan it is on, which limits apply
(projects, cloud connections, members, notification destinations) and which features are on (automation plans, PDF
reports ...), and it must be able to take payment through a provider (Stripe) once the account and keys exist.

Question: does licensing need its own server?

## Decision: no separate server now; a module with a hard boundary
Reasons: Reliabilix is multi-tenant SaaS, so every check is a lookup of one row for a tenant that already is in the
request. A remote licensing server would put a network hop and a new single point of failure into every create call,
for no isolation benefit: we run the code that enforces the limits. It would also need its own deployment, auth and
monitoring before there is a second customer.

What we build instead is the boundary that makes extraction cheap:
- **A port, `platform/entitlements`:** `Allow(ctx, tenant, resource)` (is there room under the plan's limit?) and
  `HasFeature(ctx, tenant, feature)`. Domains depend only on this interface (optional field, default "no limit" so tests
  and self-contained modules stay simple). They call it before creating a project, a cloud connection, a notification
  destination or an automation plan, and before rendering a PDF. A refusal is `ErrLimitReached` -> HTTP 402 with the limit
  in the message.
- **A `licensing` module with its own schema:** plan catalog in code (versioned, reviewed like code: free, team,
  business, enterprise; limits and features per plan), `licensing.subscriptions` per tenant (plan, status
  `active|trialing|past_due|canceled`, source `manual|stripe`, period end, provider reference), tenant-readable under RLS,
  changed only through operator functions (audited in the tenant's log) or, later, the billing provider's webhook.
  No subscription row = free plan. `canceled` or an elapsed period = free limits; `past_due` keeps the plan for a grace period.
- **Billing provider behind an interface** (`CheckoutURL`, `HandleWebhook`) with a stub implementation until Stripe keys exist.
  Until then plans are assigned by an operator in the console; the UI shows that self-service upgrade is not set up (a stub,
  labelled as such in the operator system view).
- **Downgrades never delete anything.** Existing resources stay; only new ones are refused until usage is under the limit.
- **Visible to the customer:** `GET /licensing/plan` returns plan, status, limits, current usage and features; Settings gets a
  Plan tab with usage against limits.

**Update (ADR-0023):** limits and module switches are now the `limits.*` / `modules.*` settings, enforced through the
`entitlements.Gate` port and request gates that already exist. A plan is therefore a *preset of those settings* written for
an organization when its subscription changes, so the licensing module adds the plan catalog, subscription state and the billing
provider, but no new enforcement code.

## When a separate licensing service is justified (ADR-0005 triggers)
1. **Self-hosted / on-prem installations:** there the customer's copy cannot ask our database, so it verifies a signed license file
   (offline, public-key signature, expiry, seat/feature claims) and optionally phones home to a licensing service we then run
   separately and publicly. The `entitlements` port is the same; only its implementation changes.
2. A dedicated team or billing system owns subscriptions and releases independently.
3. Measured load or a different security profile (it holds payment-adjacent data) requires isolation.
Until one of these fires, extraction would be cost without benefit.

## Consequences
- The plan catalog (numbers, features) is a product decision to confirm before the first paying customer.
- Stripe integration, invoices, taxes and dunning are deliberately out of scope here; the provider stays the system of record
  for money, we keep only the entitlement-relevant state.
