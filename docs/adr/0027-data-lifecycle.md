# ADR-0027: Data lifecycle: export, scheduled erasure and retention

Status: accepted (implemented, migration 0019)

## Context
Before real customers, an organization has to be able to leave: to take its data with it and to have it removed, and
the platform must not keep high-volume history forever. Doing this by hand per request does not scale and is easy to
get wrong (a forgotten table, a forgotten file).

## Decision
**Export.** `GET /tenant/export`, owners only, people only (not API keys). One NDJSON stream from a single read-only
repeatable-read snapshot, run under the organization's own tenant transaction. Tables are an allow-list with explicit
columns (`lifecycle/domain.ExportTables`); a test fails if a secret-looking column appears. Not exported: API key and
invitation token hashes, sealed notification targets, AWS ExternalIds, generated report files.

**Erasure.** `POST /tenant/deletion` with the exact organization name starts a grace period
(`lifecycle.deletion_grace_days`, default 14, 1-90); `DELETE /tenant/deletion` cancels. Nothing is removed during the
grace period. An hourly job then, per organization: deletes the object-store prefix `tenants/<id>/`, then runs
`platform.erase_tenant(uuid)`, one transaction that empties every row-level-secured table with a `tenant_id` or
`organization_id` column (retrying around foreign keys), deletes the organization and the users that belonged only to
it, and writes `platform.erasure_log` (tenant id, time, row counts: no names or addresses). If anything remains the
transaction is rolled back and the job retries; a half-erased organization cannot exist. The files step is idempotent.

**Retention.** `retention.data_months` (platform, default 0 = keep) drops whole monthly partitions of
`usage.usage_records` and `carbon.calculations` older than the window (never less than 13 months, never the current
month). Dropping a partition needs no row scan and no per-tenant context. Audit logs are not trimmed: they are the
record of who did what.

## Security notes
- `erase_tenant` is a SECURITY DEFINER function owned by `greenops_fanout` (BYPASSRLS) with explicit table grants and
  names the organization in every statement. Granting is deliberate: a new tenant table without a grant makes the
  integration test fail, which is the intended reminder.
- Only the worker role may execute the erase and partition functions; the API role may only create and cancel a
  request for its own organization (RLS).
- Erasure and cancellation are audited in the organization's own log until the moment it is erased.

## Consequences
- Erasure is irreversible after the grace period; backups still hold the data until they age out. The retention of
  backups is an operations decision that must be written next to the privacy policy.
- Per-organization retention (a customer choosing its own window) is not offered; it is a platform setting.
- Exports of very large organizations stream for as long as they take; a background job writing to object storage is
  the next step if that becomes a problem.
