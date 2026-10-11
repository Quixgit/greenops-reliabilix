# ADR-0044: AWS and GCP through the customer's own credential, like Azure

Status: accepted · 0.1.0-alpha.65 · extends ADR-0008, ADR-0018, ADR-0043

## Context
Azure became a one-step connection with ADR-0043: the customer gives a read-only service principal and nothing is
exported, uploaded or shared. AWS (a role the platform assumes, plus an ExternalId) and GCP (a billing export, a dataset
label and a dataset shared with the platform's service account) still asked the customer to build several things
before the first byte of cost could be read. The decision is that every provider has one usual way, the same in
spirit: **the customer's own read-only credential**, entered in the console.

## Decision
- **AWS: an IAM user access key** (`credential_ref = awskey://<access key id>`, the secret access key is the
  write-only `client_secret`). The key id is an identifier; the secret is sealed. Cost Explorer is read with it directly
  (`ce:GetCostAndUsage`, optionally `ce:GetRightsizingRecommendation`), the FOCUS S3 export and rightsizing work the
  same way. Only long-term keys (`AKIA…`) are accepted: a session key (`ASIA…`) expires and would silently stop the sync.
  The role mode stays as "advanced" (it stores no secret at all).
- **GCP: a service account key** (the whole JSON key file as the write-only `client_secret`; `credential_ref` stays
  `bq://<project>/<dataset>/<table>`). A connection that has a sealed secret is in this mode. Queries run with the
  customer's service account **in its own project** (BigQuery Job User there, Data Viewer on the dataset), so the
  platform's own Google identity is not needed at all: the GCP connector no longer depends on `PLATFORM_GCP_PROJECT`.
  The shared-dataset mode stays as "advanced" and is available where the platform has its Google identity.
- **No ownership proof in key mode.** Possession of the credential is the proof of control, exactly as for the
  Azure service principal; the dataset label exists only because the platform's own identity could otherwise be steered
  at another customer's dataset (confused deputy).
- **What is accepted as a secret is checked by shape and nothing else is stored:** an AWS secret access key (40
  characters), a Google key file of `type: service_account` with a project, an email on `iam.gserviceaccount.com`, a
  private key and Google's own token URI (a user credentials file, an OAuth client file or a foreign token URI is
  refused: they would be a broader grant than intended). Error texts never echo the key.
- **Sealing is the one mechanism of ADR-0043**: AES-256-GCM under the platform key, bound to the organization and the
  connection token, `json:"-"`, never in a response, a log or the data export, 503 when the platform has no sealing key.
  One more rule: the secret is accepted only for the three kinds that use it (Azure service principal, AWS access key,
  GCP key); on any other connection it is refused.
- `access` on a connection (`role`, `access_key`, `dataset_share`, `service_account_key`, `service_principal`,
  `export`) says which way is in use, computed, so the console explains the right steps.
- Failures say why (alpha.64): an unopenable stored secret names the sealing-key mismatch, a key Google rejects names
  the account, a refused AssumeRole or a missing permission names the step.

## Not done / risks
- A long-term access key is a standing secret the platform keeps. It is least-privilege (cost read only), sealed and
  revocable by the customer at once, but a role with an ExternalId stores nothing; that is why the role mode stays.
  Rotation is by reconnecting; automatic rotation reminders are not built.
- GCP bills the customer's project for the query (a few cents at most, the scan is capped at 20 GiB); the console says
  so in the steps.
- The key mode of GCP and AWS has been exercised against fakes and the admin check commands, not yet against a live
  customer account.
