# ADR-0021: An operator error log in the database, next to logs and Sentry

Status: accepted

## Context
Errors of the api, the worker and the integrations they call were visible only in container logs and Sentry. Several
handlers answered 500 without recording the cause at all.

## Decision
- `platform.error_groups` holds one row per kind of error: fingerprint (service + component + message with ids and
  numbers masked), severity, sanitized message, occurrence counter, first/last seen, the tenant and request id of the last
  occurrence, status `open | resolved | ignored`. A job that retries a thousand times is one row. A resolved error that
  recurs is reopened; an ignored one stays ignored.
- **Sources:** every 5xx and panic of the api (`httpx.ErrorHook`, grouped by route pattern, with the cause that handlers now
  report through `httpx.Internal`), every failed background job attempt (worker error handler; jobs that gave up for good are
  warnings), notification deliveries that fail for good. Add a source by calling `errlog.Recorder.Record`.
- **Sanitized before storage:** credentials in URLs, bearer/secret values, query strings, cloud access keys, e-mail
  addresses and token-like strings are removed; control characters stripped; 500 characters at most.
- **Never harms the caller:** events are counted in memory and flushed every 5 s; the buffer is bounded (extra new kinds are
  dropped and counted in `platform_errors_total`); a failed write is logged and dropped; a malformed tenant id does not lose
  the error.
- Written through `platform.record_error` (api and worker roles); read and handled only through the operator functions;
  resolved/ignored groups are purged after 90 days (`platform:purge_errors`).

## Consequences
- Slog lines and Sentry stack traces remain the detailed record; this is the grouped, queryable picture.
- The scheduler and the web frontend are not sources yet (the scheduler has no database connection; browser errors would
  need an authenticated intake endpoint, a flood risk to design first).
