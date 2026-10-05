# ADR-0010 Secrets management

Status: accepted

Decision: no credentials in DB/frontend. Prefer federated short-lived identity (IAM role + External ID, workload identity); otherwise Vault or the cloud secret manager, referenced by path.
