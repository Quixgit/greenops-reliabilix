# ADR-0038: Payment providers and subscriptions

Status: accepted · 0.1.0-alpha.32 · implements the billing part of [ADR-0022](0022-licensing-and-subscriptions.md)

## Decision
1. **Providers hold the money; the platform holds a state.** No card or bank data is stored. The platform keeps a
   subscription row (plan, status, source, trial end, period end) and an event id per provider notification.
2. **A provider is an adapter behind one port** (`platform/billing`): `Checkout` (a hosted page to send the person to) and
   `ParseWebhook` (verify, then reduce to plan, status and period). Credentials live in the console (`platform/integrations`),
   read through the store, so a key applies without a restart.
3. **Providers shipped**, chosen for different customers:
   | Provider | For | How it sells | Notification check |
   |---|---|---|---|
   | Stripe | cards worldwide, Apple Pay and Google Pay | Checkout, subscription | HMAC-SHA256 header, 5-minute window |
   | Paddle | selling abroad: it is the merchant of record and handles VAT and sales tax | transaction, subscription | HMAC-SHA256 header, 5-minute window |
   | LiqPay | Ukraine, UAH: cards, Apple Pay, Google Pay, Privat24 | signed checkout link, monthly subscription | signature (sha1 over key, data, key) |
   | WayForPay | Ukraine, UAH: cards, Apple Pay, Google Pay, instalments | hosted invoice page, one payment buys a period | HMAC-MD5 over fixed fields, signed accept reply |
   | Payment link | Payoneer payment requests, Wise, PayPal.me, any hosted link | a link per plan | none: the operator confirms |
   | Invoice | bank transfer | request recorded | none: the operator confirms |
   Apple Pay and Google Pay are payment *methods* offered by these processors on their payment pages; they are not
   separate providers. A direct Payoneer Checkout adapter needs a merchant account and its API documentation and is not
   included; Payoneer works today through payment links.
4. **A notification is applied once.** `platform.billing_apply` records `(provider, event id)`; a repeated delivery changes
   nothing. A bad signature is a 400 and nothing is applied; a provider that is not set up answers 503.
5. **A plan applies its preset** (limits and modules, ADR-0033) through one function, `platform.apply_plan`, used by the
   webhooks and by the expiry job; operators go through the settings engine and `operator_set_subscription`.
6. **The period is a clock.** Hourly, `licensing:expire` returns organizations whose paid period or trial ended to the Free
   plan (limits and modules of Free, `subscription.expired` in the audit log). Lowering a plan never deletes data.
7. **Manual providers** (invoice, payment link) record an "asked to pay" event the operator sees on Plans & billing; the
   operator assigns the plan with its period when the money arrived.

## Consequences
- Prices live at the providers (price ids, amounts in the console), not in the plan catalog.
- A subscription with automatic renewal (Stripe, Paddle, LiqPay) keeps itself current through the provider's events; a
  provider that sells periods (WayForPay) needs the customer to pay again at the end, and the plan returns to Free if not.
- Taxes and invoices are the providers' job (Paddle as merchant of record).
