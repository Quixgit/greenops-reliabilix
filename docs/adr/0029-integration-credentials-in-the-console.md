# ADR-0029: External-service credentials are managed in the operator console

Status: accepted (implemented, migration 0022)

## Context
Keys and addresses of external services (Electricity Maps, SMTP) lived only in the deployment environment: changing one
meant a redeploy, and nobody could see from the product whether a service was configured. Operators asked that every
external dependency have its settings in the admin from the start.

## Decision
- **A catalog** (`platform/integrations`) lists each integration and its fields: label, whether it is a secret,
  whether it is required, its validation and the environment variable it falls back to.
- **Value order:** the console, then the environment, then nothing. The effective value and its source are shown per field.
- **Secrets are sealed** with `platform/secretbox` (AES-256-GCM, bound to `integration:<id>:<field>`). The database holds
  ciphertext only. The API never returns a secret: a field reports `configured` and a masked hint (`••••` plus the last
  four characters of a long secret), and a new value is typed to replace it.
- **One write path.** The application role has no write grant on `platform.integration_values`; operators change values
  only through `platform.operator_set_integration`, behind the operator console's admin check, which also logs the access.
- **Applied at run time.** Services read through `integrations.Store` (15 s cache): the carbon provider
  (`electricitymaps.Dynamic`) and the e-mail channel (`email.Channel.Config`) look the value up on every use, so a change
  needs no restart. E-mail is hidden from the notification catalog and refused for new destinations until it is
  configured (development logs instead).
- **A connection test** per integration returns a short, fixed-text result and never echoes a provider's error.
- **Applied at restart.** Settings of clients that are built once (object storage, error tracking, tracing, the
  platform AWS account and Google project) live in the same catalog, marked `startup`: services read them from the
  database right after connecting to it (`ApplyIntegrations`) and use them instead of the environment's values. A change
  needs a restart, which the console says.
- **Fixed by the deployment** (the identity provider and the other variables of the services) stays in the
  environment and is listed read-only on the same page with its state.

## Rule for new integrations
Any new external service that needs a key or an address gets a catalog entry in the same change as the code that uses it,
read through the store. A value that cannot be applied at run time is listed as "set by the deployment" until it can.

## Consequences
- The sealing key (`NOTIFICATIONS_SECRET_KEY`) is now the platform's secret key; without it secrets cannot be stored
  (503), non-secret fields still can. Rotating it needs a re-entry of the secrets.
- A process reads changes within the cache time; changes by another process show after at most 15 seconds.
