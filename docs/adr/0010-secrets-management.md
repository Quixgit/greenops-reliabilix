# ADR-0010 Secrets management

Status: accepted

Decision: no secret ever lives in PostgreSQL, the repository or the frontend. Customer cloud access is federated: AWS IAM role + ExternalId (Azure/GCP workload identity later), read-only billing scope. connections.credential_ref is a reference (role ARN) and values that look like keys are rejected. The platform's own secrets (DB passwords, Auth0 client secret, Electricity Maps key, Sentry DSN) come from the environment, injected from Vault or a cloud secret manager. API keys and invitation tokens are stored only as SHA-256 hashes and shown once.
