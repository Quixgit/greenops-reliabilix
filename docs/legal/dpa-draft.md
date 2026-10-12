# Data processing agreement: working draft

**Status: draft for review by a lawyer. Not a contract and not legal advice.** It records what the platform actually does, so that the
agreement can be written against facts. Facts below come from [the security model](../security/model.md) and
[ADR-0027](../adr/0027-data-lifecycle.md).

## Roles
The customer is the controller of the personal data in its organization; Reliabilix is the processor. Personal data is limited to the
names and e-mail addresses of members, sign-in identifiers, and audit records of what members did. Billing and usage records of the
customer's cloud are business data and may contain resource names a customer chose.

## What is processed, and what is not
Processed: members' names and e-mails, billing and usage records, resource metadata, the customer's settings, audit records.
Not read: the content of workloads and storage. Not kept: card numbers (payment providers hold them), cloud passwords (access is a
read-only role the customer creates; the few secrets that must be kept are sealed and write-only).

## Security measures
As in the security model: forced row-level isolation per organization, roles read from the database, hashed API keys, sealed secrets
(AES-256-GCM), append-only audit log, read-only cloud access with a per-connection identifier, TLS in transit.

## Return and deletion
Export of all organization data by the owner at any time (NDJSON). Deletion by the owner after a grace period (default 14 days), in one
transaction with the organization's files; a record of row counts remains. **Open:** the age-out period of backups must be decided and
written here; ADR-0027 names it as an operations decision.

## Sub-processors
Per deployment: hosting provider, identity provider, payment providers in use. The list must be filled in with names, countries and
purposes before signing. Public grid-intensity sources receive a region or zone only.

## Open points for the lawyer
Hosting location and transfers outside the EU/EEA; breach notification time; audit rights; liability; the instruction that the platform
never acts in the customer's cloud (it only plans); retention of audit records (kept as the record of who did what).
