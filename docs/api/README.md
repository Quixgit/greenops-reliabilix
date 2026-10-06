# API guide

Contract: `backend/api/openapi.yaml` (OpenAPI 3, validated and kept in sync with the routes by tests).
TypeScript types: `npm run gen:api` in `frontend/`.

## Authentication

`Authorization: Bearer <Auth0 access token>` or an API key `grk_...`. Optional `X-Tenant-ID` selects among several
memberships. A signed-in user without a tenant gets `403` with `detail: onboarding_required`.

Auth0 setup (post-login Action) - the API reads only the user's identity from the token:

```js
exports.onExecutePostLogin = async (event, api) => {
  const ns = "https://reliabilix.com/";
  api.accessToken.setCustomClaim(ns + "email", event.user.email);
  api.accessToken.setCustomClaim(ns + "email_verified", event.user.email_verified);
};
```

## First steps

1. `POST /api/v1/onboarding {"organization_name": "Acme"}` - creates the organization, you become `owner`.
2. `POST /api/v1/projects {"name": "prod", "functional_unit": "api_request"}`.
3. `POST /api/v1/projects/{id}/functional-units` - how many functional units the project served in a period (SCI denominator).
4. `PUT /api/v1/projects/{id}/policy {"allowed_regions": ["eu-central-1", "eu-west-1"]}` - data-residency allow-list. Without it no region_shift recommendation is ever produced.
5. `POST /api/v1/cloud-accounts` - registers a pending connection and returns `setup.external_id`, a trust policy and the (read-only) permissions policy. Create the IAM role in AWS, then
   `POST /api/v1/cloud-accounts/{id}/verify`: on success the first sync is queued (sync -> FOCUS usage -> carbon -> recommendations).
6. Read results: `/carbon/summary`, `/carbon/trend`, `/finops/summary`, `/dashboard/*`, `/recommendations`.

The platform account that customers trust is `PLATFORM_AWS_ACCOUNT_ID`; that identity needs only `sts:AssumeRole` on customer roles.

## Recommendations

`GET /recommendations` -> `POST /recommendations/{id}/approve` -> `POST /recommendations/{id}/apply` (or `/dismiss`).
Only people may decide (not API keys); compliance is re-checked against the current policy on every step (`409` if it changed).

## CI gate

Create a key (`POST /api-keys {"name": "github-actions", "role": "ci"}`, shown once), then in a pipeline:

```bash
curl -sS -X POST https://api.reliabilix.green/api/v1/ci/evaluate \
  -H "Authorization: Bearer $GREENOPS_API_KEY" -H "Content-Type: application/json" \
  -d '{"project_id": "<uuid>",
       "changes": [{"kind": "vcpu_hours", "amount": 7200, "region": "eu-central-1", "monthly_cost_delta": 38}],
       "thresholds": {"carbon_warn_kg": 10, "carbon_block_kg": 20, "cost_block": 100}}'
# {"verdict":"PASS|WARN|BLOCK","carbon_kg_month":...,"cost_delta_month":...,"reasons":[...]}
```

Thresholds omitted in the request fall back to the project policy. Only increases count; reductions never block.
A reusable GitHub Action wrapping this call (`reliabilix/greenops-ci-gate`) is a separate repository.

## Reports

`POST /reports {"kind": "carbon|sci|finops", "format": "csv|json|pdf", "period_start": "...", "period_end": "..."}` -> `202`;
poll `GET /reports/{id}` until `ready`, then `GET /reports/{id}/download`. Files live in S3-compatible storage under
`tenants/{tenant_id}/reports/`. CSV cells are protected against formula injection.

## Cloud connection role names

The IAM role ARN must match `arn:aws:iam::<12 digits>:role/Reliabilix*`; other names are rejected (limits the blast radius of the platform's `sts:AssumeRole`). Verification results are audited.
