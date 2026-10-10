# ADR-0019: Notifications: a port for producers, channels as plug-ins, a database outbox

Status: accepted

## Context
The platform has to tell people things: a report is ready, a cloud connection needs attention, later a budget was
exceeded or a recommendation is waiting. Customers want those messages where they work: e-mail, Slack, their own
systems (webhooks), later Teams, PagerDuty, SMS. Before this ADR nothing could be delivered (invitations even return
a raw token because there is no e-mail).

Requirements: producers must not know about channels; a new channel must not touch producers, tables or the API;
a lost message is worse than a duplicate; tenants must not be able to point the platform at its own network or use
it to mail strangers; no secret may sit in clear text in PostgreSQL (ADR-0010).

## Decision

**A port, not a module import.** `platform/notify` defines `Event` (channel-neutral: type, severity, title, body,
app-relative path, small attrs, optional dedupe key) and `Notifier.Publish(ctx, tenantID, event)`. Domains depend on
that interface only (`reports` and `cloudaccounts` already publish through it). The `notifications` module
implements it and is wired in `internal/app`; lint forbids every domain from importing `notifications` and
`notifications` from importing any domain. Publishing is best effort for the producer: a notification problem never
fails the business operation.

**Destinations are tenant configuration.** A destination is `kind` + sealed address + event filter (`event_types`,
`*` allowed) + `min_severity`. At most 20 per tenant. The API never returns an address, only a masked
`target_hint`. Owners and admins write, engineers read (`notification:read|write`); machine keys have neither.

**Channels are plug-ins.** `domain.Channel` is `Kind()`, `Prepare(address)` (validate, normalise, build a hint) and
`Send(ctx, target, message)`. Errors are classified with `domain.Permanent(err)`: retrying cannot help (HTTP 4xx,
rejected mailbox) versus transient. Optional capability interfaces keep the application channel-agnostic:
`PersonalChannel` (the address identifies a person) and `SigningChannel` (the channel signs its requests). Shipped:
`email` (SMTP with required TLS), `webhook` (signed JSON), `slack` (incoming webhook). Adding Teams or PagerDuty =
one package implementing `Channel` + one line in `internal/app/notifications.go`. The registry is built per process,
so a deployment offers only the channels it is configured for (`GET /notifications/catalog`).

**A transactional outbox in PostgreSQL.** `Publish` writes one `notifications.deliveries` row per matching
destination (idempotent per `(destination, dedupe_key)`), then enqueues `notifications:deliver` as a fast path.
The database is the source of truth: a worker takes a 2-minute lease (`UPDATE ... RETURNING`), sends, and records
`sent`, a retry (`next_attempt_at` = 30 s * 2^(n-1), capped at 1 h) or `failed` after 8 attempts or a permanent
error. A scheduler job (`notifications:sweep`, every minute) re-queues anything due through
`notifications.list_due_deliveries` (a SECURITY DEFINER function returning ids only), so a Redis outage, a lost task
or a crashed worker loses nothing. Delivery is at-least-once; every webhook carries a stable delivery id
(`X-Reliabilix-Delivery`) for receiver-side deduplication. Finished rows are purged after 90 days
(`notifications:purge`). A removed destination keeps its history (`destination_id` becomes NULL; pending rows become
`skipped`).

**Security.**
- *Secrets at rest:* the address is sealed with AES-256-GCM (`platform/secretbox`) under `NOTIFICATIONS_SECRET_KEY`;
  the additional data is `<tenant>:<destination id>`, so a ciphertext copied to another row does not open. This
  refines ADR-0010: the database holds ciphertext only, the key lives in the environment. Without the key the API
  answers 503 for destination creation instead of storing anything unprotected.
- *No stored signing secrets:* a webhook's HMAC secret is derived from the same key (`HMAC(key, "destination-secret:"+id)`),
  shown once on creation and recomputed at send time. Signature: `X-Reliabilix-Signature: t=<unix>,v1=<hex HMAC-SHA256(secret, t + "." + body)>`.
- *SSRF:* webhook/Slack URLs must be https and public. The check runs when the connection is made, after DNS
  resolution (so a host name that resolves or rebinds to a private, loopback, link-local, metadata or CGNAT address is
  refused), redirects are never followed, environment proxies are ignored, response size and time are bounded, and
  error texts never contain the URL. Slack is pinned to `hooks.slack.com`. Development (`ENV=dev`) may allow a local
  receiver.
- *Mail abuse:* e-mail goes only to addresses of current tenant members (checked through a `Directory` port wired to
  the tenants module), header values are stripped of control characters, mail is quoted-printable UTF-8.
- *Isolation:* both tables have forced RLS (the existing integration test covers the new schema). The API role can
  manage destinations and insert deliveries, the worker can only read destinations and update deliveries; the
  cross-tenant helpers are SECURITY DEFINER functions granted to the worker only.
- *Audit:* destination changes and terminal delivery failures are audited in the same transaction, with the masked
  hint, never the address.

## Consequences
- The first events are `report.ready`, `report.failed` and `connection.error`. A producer adds an event type to
  `notify.Catalog()` and calls `Publish`; nothing else.
- E-mail now exists as a channel, but invitations still return their token: wiring them is a separate change (it
  needs a transactional template, not a tenant destination).
- Losing or rotating `NOTIFICATIONS_SECRET_KEY` makes stored destinations unreadable (deliveries retry, then fail).
  The `v1:` prefix of sealed values is the hook for supporting two keys during a rotation; not built yet.
- `notifications` depends on the queue and the database only, so it is a natural first candidate for extraction
  (ADR-0005): the producer port becomes an HTTP/queue client, the outbox stays where it is.
- Not built, deliberately: per-user preferences and in-app inbox, message templates and localisation, per-tenant send
  rate limits (a tenant is bounded by 20 destinations and by what its producers publish), digest/batching.
