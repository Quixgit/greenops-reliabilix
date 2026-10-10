# Connecting external services

Everything the platform needs from outside the repository, in the order to do it.
After each step run `make doctor` (`go run ./cmd/admin doctor`): it checks only what is
configured and tells you what is still missing. Nothing below is required for local
development (`ENV=dev`, `docker compose up`), which uses a dev auth verifier, local
storage and no external APIs.

> Secrets never go into git. In production keep them in a secret manager (Vault, AWS
> Secrets Manager, Doppler) and inject them as environment variables.

## 1. PostgreSQL 18

1. Create the database and run the migrations as the owner role:
   `goose -dir backend/migrations postgres "$OWNER_DSN" up`
   (docker compose does this in the `migrate` service).
2. The migrations create the runtime roles `greenops_api` and `greenops_worker`
   (no BYPASSRLS, own nothing). Set a strong password for each:
   `ALTER ROLE greenops_api PASSWORD '...'; ALTER ROLE greenops_worker PASSWORD '...';`
3. `DATABASE_URL` for api/scheduler uses `greenops_api`, for the worker `greenops_worker`.
   Use `sslmode=verify-full` outside a private network. `doctor` rejects `sslmode=disable`
   and default passwords when `ENV` is not `dev`.
4. Back up daily and test a restore; usage tables are partitioned by month and the
   scheduler creates the next partitions (`admin partitions` does it manually).

## 2. Redis

Any Redis 7+ reachable from api, worker and scheduler (`REDIS_ADDR`). It carries the job
queue and rate limits, so enable persistence (AOF) and a password/TLS where it is not on
a private network.

## 3. Auth0

1. Create a tenant. **Applications → APIs → Create API**: identifier = `AUTH0_AUDIENCE`
   (for example `https://api.reliabilix.com`), signing algorithm RS256.
2. **Applications → Create Application → Regular Web App** for the Next.js frontend.
   Allowed callback `https://<web-host>/auth/callback`, logout URL `https://<web-host>`.
   Note domain, client id and client secret (frontend only).
3. **Actions → Library → Build custom → Post Login** and deploy it into the Login flow:
   ```js
   exports.onExecutePostLogin = async (event, api) => {
     const ns = "https://reliabilix.com/";
     api.accessToken.setCustomClaim(ns + "email", event.user.email);
     api.accessToken.setCustomClaim(ns + "email_verified", event.user.email_verified);
   };
   ```
   The token carries identity only. Tenant and role are read from the database, so do not
   put roles into Auth0.
4. Backend env: `AUTH0_DOMAIN=<tenant>.eu.auth0.com`, `AUTH0_AUDIENCE=<identifier>`,
   `AUTH_CLAIM_NS=https://reliabilix.com/`.
5. Optional hardening: enable MFA, disable public sign-ups if you onboard by invitation.

Check: `make doctor` fetches the JWKS and reports the key count.

## 4. AWS: platform identity

The platform assumes a role in each customer account, so it needs its own AWS identity.

1. Create an IAM role (ECS/EKS/EC2) or user for the worker in your platform account.
2. Attach `deploy/aws/platform-policy.json` (`sts:AssumeRole` on `arn:aws:iam::*:role/Reliabilix*`
   plus the storage permissions if you use S3 there).
3. Set `PLATFORM_AWS_ACCOUNT_ID=<12-digit id>`. Credentials come from the default AWS
   chain (task role, instance profile or `AWS_*` variables).

Check: `doctor` calls `sts:GetCallerIdentity` and compares the account with
`PLATFORM_AWS_ACCOUNT_ID`.

## 5. AWS: customer onboarding (per customer)

1. The customer creates a connection in the product (`POST /api/v1/cloud-accounts`). The
   response contains the `external_id` and the platform account id.
2. They deploy `deploy/aws/customer-role.yaml` (CloudFormation) or `customer-role.tf`
   (Terraform) with those two values. It creates **`ReliabilixReadOnly`** with
   `ce:GetCostAndUsage` and the optional `ce:GetRightsizingRecommendation` (rightsizing advice; without it everything else keeps working, and the customer must also opt in to rightsizing recommendations in Cost Explorer preferences). The role name must start with `Reliabilix`; the API rejects
   other names.
3. They paste the role ARN, then call verify. The result is audited
   (`cloud_connection.verified` / `verification_failed`).
4. Each sync makes three Cost Explorer queries (cost, EC2 hours, S3 storage; $0.01 per result page). No extra permission is needed.
5. Enable Cost Explorer in the customer's account (first activation takes up to 24 h).

### 5b. Recommended: FOCUS data export (invoice-level data)

Cost Explorer works out of the box. For invoice-level data and exact instance hours, the customer creates an
**AWS Data Export** (Billing and Cost Management → Data Exports → *Standard data export*, table
**FOCUS 1.0 with AWS columns**, format **Text or CSV, gzip**, daily refresh, overwrite) into an S3 bucket.

1. Deploy the role template with `ExportBucketName` and `ExportPrefix` (read access to that prefix only,
   `s3:GetObject`, nothing else).
2. `PUT /api/v1/cloud-accounts/{id}/export` with `{"bucket","prefix","name","region"}` (locations only).
3. Verify again. The connection then reads the export instead of Cost Explorer (switch back with `DELETE`).

The platform reads the month's manifest, accepts only files inside the export's own folder, streams them with
size limits and stores daily aggregates. Delivery lags by up to a day, and the first delivery can take 24 h.

## 5c. GCP: platform identity and customer onboarding

The GCP connector is opt-in. It reads the customer's **Cloud Billing export in BigQuery** with the platform's own
Google identity; no customer credential is stored.

Platform side (once):
1. Create a project for the platform's BigQuery jobs and enable the BigQuery API in it
   (`gcloud services enable bigquery.googleapis.com --project <platform-project>`).
2. Create a service account in it, grant it `roles/bigquery.jobUser` on that project, and give the api/worker its
   credentials (a key file via `GOOGLE_APPLICATION_CREDENTIALS`, or workload identity on your runtime).
3. Set `PLATFORM_GCP_PROJECT=<platform-project>` and `PLATFORM_GCP_SERVICE_ACCOUNT=<service account email>`.
   `make doctor` runs a dry-run query to prove the identity, the API and the role.

Per customer (shown step by step by `POST /cloud-accounts` with `provider: gcp`, `account_ref` = their GCP project id
and `credential_ref` = `bq://<project>/<dataset>/<table>`):
1. The BigQuery API must be enabled in their project.
2. Cloud Billing export to BigQuery (Standard usage cost) must write into the dataset. It needs the Billing Account
   Administrator role, only covers data from the day it is enabled (no history) and the first delivery can take up
   to 48 hours. Until then verification answers "billing export not found": verify again later.
3. They add the connection's label to the dataset (`bq update --set_label <external_id>:1 <project>:<dataset>`):
   proof that they control it.
4. They share the dataset with the platform service account as BigQuery Data Viewer (read-only);
   `deploy/gcp/customer-access.tf` covers steps 3 and 4.
5. They verify. Failures are specific: BigQuery not enabled, export not found, no access, ownership not proven.

Queries are capped at 20 GiB scanned each and 500k result lines; they only read the billing export table.

## 5d. Azure: platform application and customer onboarding

The Azure connector is opt-in. It reads the customer's **Cost Management export (FOCUS cost and usage, CSV) from Blob
Storage** with the platform's own Microsoft Entra application; no customer credential, SAS token or key is stored
(ADR-0041).

Platform side (once):
1. Register a **multi-tenant** application in Microsoft Entra ID (accounts in any organizational directory). No API
   permissions are needed: access comes from the role the customer assigns on their container.
2. Create a client secret for it and note its expiry (rotate before it lapses).
3. Enter the application (client) id and the secret in the operator console (Integrations, Platform Microsoft Azure
   application) or set `PLATFORM_AZURE_CLIENT_ID` and `PLATFORM_AZURE_CLIENT_SECRET`; restart api, worker and
   scheduler (the application is read at start, like the GCP project).

Per customer (shown step by step by `POST /cloud-accounts` with `provider: azure`, `account_ref` = the subscription id
and `credential_ref` = `azblob://<tenant id>@<storage account>/<container>/<export directory>`):
1. Create a Cost Management export of type "Cost and usage details (FOCUS)", version 1.0, daily, CSV (gzip is fine),
   into that storage account, container and directory. Use "Export all data" for history; the first delivery can take
   up to 24 hours. Until then verification answers "billing export not found": verify again later.
2. Mark the container with the connection's token as metadata (`az storage container metadata update ... --metadata
   <external_id with - replaced by _>=1`): proof that they control it.
3. Create the platform application's service principal in their tenant (`az ad sp create --id <application id>`) and
   assign it Storage Blob Data Reader on the container only. `deploy/azure/customer-access.tf` covers steps 2 and 3.
4. They verify. Failures are specific: application refused in the tenant or no role (access denied), export not found,
   ownership not proven, no rows for the subscription.

A billing-scope export may contain many subscriptions; only the connection's subscription is read. Reads are bounded
(500 files, 24 GiB per sync).

## 6. Electricity Maps (grid carbon intensity)

1. Get an API key at <https://www.electricitymaps.com/> (Free tier is enough for testing).
2. `ELECTRICITYMAPS_API_KEY=...`
3. Regions are mapped to zones by a built-in table (about 26 AWS regions). Override or add
   with `ELECTRICITYMAPS_ZONE_OVERRIDES=eu-central-1=DE,eu-north-1=SE`. An invalid spec
   stops startup.

The client caches the latest reading for 15 minutes and forecasts for 1 hour; `refresh-grid` also stores the forecast (optional, a missing forecast is only logged).

Without a key the carbon job has no grid data and `refresh-grid` reports it; cost data
still flows. Check: `doctor` makes a live call and verifies zone availability.

## 7. Object storage (reports)

Any S3-compatible store: AWS S3, Cloudflare R2 or MinIO.

1. Create a **private** bucket, block public access, enable versioning/encryption.
2. A key limited to that bucket with `GetObject`, `PutObject`, `DeleteObject`.
3. Env: `S3_BUCKET`, `S3_REGION`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`; for R2/MinIO also
   `S3_ENDPOINT=https://<account>.r2.cloudflarestorage.com` and `S3_PATH_STYLE=true` for MinIO.

Check: `doctor` writes, reads and deletes a probe object.

## 7b. Notifications (e-mail, webhook, Slack)

Events (`report.ready`, `report.failed`, `connection.error`) are delivered to destinations that owners and admins
configure through `POST /notifications/destinations` (ADR-0019).

1. `NOTIFICATIONS_SECRET_KEY=$(openssl rand -base64 32)`. Keep it in the secret manager next to the DB password: it
   seals destination addresses, and webhook signing secrets are derived from it. Without it the API answers 503
   when someone adds a destination. Changing it makes existing destinations unreadable: delete and re-add them.
2. E-mail (optional): `SMTP_HOST`, `SMTP_PORT` (587 = STARTTLS, 465 = implicit TLS), `SMTP_USERNAME`,
   `SMTP_PASSWORD`, `SMTP_FROM` (`Reliabilix <notifications@your-domain>`). TLS is mandatory. Mail goes only to
   addresses of members of the organization. Without `SMTP_HOST` the e-mail channel is not offered.
3. `PUBLIC_BASE_URL=https://app.your-domain` so messages link back to the app.
4. Webhook and Slack need nothing else. Webhooks must be public https endpoints; deliveries are signed
   (`X-Reliabilix-Signature: t=<unix>,v1=<hex HMAC-SHA256(secret, "<t>.<body>")>`, secret shown once on creation).
   Receivers should reject old timestamps and deduplicate on `X-Reliabilix-Delivery`.
5. `make doctor` checks the key and that the SMTP server is reachable. Use `POST /notifications/destinations/{id}/test`
   to send a test message; `GET /notifications/deliveries` shows what happened.

The worker delivers, the scheduler runs `notifications:sweep` every minute (retries and lost queue messages) and
`notifications:purge` daily. Watch `notification_deliveries_total{channel,outcome}`.

## 7c. Platform operators

Operators see metadata across organizations (ADR-0020). They are rows in `platform.operators`, written with the **owner**
database role only:

```
make operator-add SUB='auth0|abc123' EMAIL=you@company.com ROLE=admin   # viewer = read only
make operator-list
make operator-remove SUB='auth0|abc123'
```

`SUB` is the Auth0 `sub` of the person (visible in their token or the Auth0 user page). Enforce MFA for these accounts in
Auth0. The console is at `/operator`; the API checks the database on every request and records each access
(`platform.operator_access_log`). In development the seed makes the dev user an operator.

## 8. Observability (optional)

- Local: `docker compose --profile observability up` starts Prometheus, Tempo, Loki,
  Promtail and Grafana (<http://localhost:3001>). Metrics listen on `METRICS_ADDR`
  (loopback by default; do not expose it publicly).
- Tracing: set `OTEL_EXPORTER_OTLP_ENDPOINT` (for example `tempo:4317`).
- Sentry: create a Go project, set `SENTRY_DSN`.
- Alert rules: `deploy/observability/alerts.yml`.

## 9. Edge: Caddy / Cloudflare

Terminate TLS at Caddy or Cloudflare; proxy `/api` to `greenops-api:8080` and the rest to
the web container. Set `CORS_ORIGINS` to the exact web origin. Keep `METRICS_ADDR` and the
database/Redis ports off the public network.

## 10. GitHub Actions

Repository secrets for CI gating by customers: none are needed by this repo's CI itself.
Customers create an API key (role `ci`) in the product and store it as their own
secret, then call the gate endpoint (see `docs/api/README.md`).

## Environment variables

| Variable | Required | Purpose |
|---|---|---|
| `ENV` | yes | `dev` enables the dev auth verifier and local storage; anything else is strict |
| `DATABASE_URL` | prod | api/scheduler: `greenops_api`; worker: `greenops_worker` |
| `REDIS_ADDR` | yes | job queue, rate limit |
| `AUTH0_DOMAIN`, `AUTH0_AUDIENCE`, `AUTH_CLAIM_NS` | prod | JWT verification |
| `PLATFORM_AWS_ACCOUNT_ID` | for AWS sync | trust principal given to customers |
| `PLATFORM_GCP_PROJECT`, `PLATFORM_GCP_SERVICE_ACCOUNT` | for GCP sync | project where BigQuery jobs run; principal customers share their dataset with (credentials via `GOOGLE_APPLICATION_CREDENTIALS` or workload identity) |
| `PLATFORM_AZURE_CLIENT_ID`, `PLATFORM_AZURE_CLIENT_SECRET` | for Azure sync | the platform's multi-tenant Entra application: customers create its service principal and grant it read access to their export container |
| `ELECTRICITYMAPS_API_KEY`, `ELECTRICITYMAPS_ZONE_OVERRIDES` | for carbon | grid intensity |
| `NOTIFICATIONS_SECRET_KEY` | prod, for notifications | 32 random bytes, base64 (`openssl rand -base64 32`); seals destination addresses and derives webhook signing secrets. Losing or changing it makes stored destinations unreadable |
| `PUBLIC_BASE_URL` | no | https address of the web app, used for links in notifications |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` | for e-mail | e-mail channel (STARTTLS on 587, implicit TLS on 465; TLS is required). Webhook and Slack need no extra setup |
| `S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `S3_PATH_STYLE`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | prod | reports |
| `SENTRY_DSN`, `OTEL_EXPORTER_OTLP_ENDPOINT` | no | error tracking, traces |
| `CORS_ORIGINS`, `HTTP_ADDR`, `METRICS_ADDR` | no | edge |
| `SYNC_BACKFILL_DAYS`, `RATE_PER_SEC`, `RATE_BURST` | no | tuning |

## Final check

```
make doctor     # every configured integration
make smoke      # end-to-end against a running stack
```
