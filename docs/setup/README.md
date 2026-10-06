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
| `ELECTRICITYMAPS_API_KEY`, `ELECTRICITYMAPS_ZONE_OVERRIDES` | for carbon | grid intensity |
| `S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `S3_PATH_STYLE`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` | prod | reports |
| `SENTRY_DSN`, `OTEL_EXPORTER_OTLP_ENDPOINT` | no | error tracking, traces |
| `CORS_ORIGINS`, `HTTP_ADDR`, `METRICS_ADDR` | no | edge |
| `SYNC_BACKFILL_DAYS`, `RATE_PER_SEC`, `RATE_BURST` | no | tuning |

## Final check

```
make doctor     # every configured integration
make smoke      # end-to-end against a running stack
```
