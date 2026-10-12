# Backup and recovery: what is kept, how much may be lost, how long it takes

This is the contract for an operator of the platform. The platform does not run backups itself: PostgreSQL and object
storage are operated by whoever deploys it, and this page says what they must keep, the targets to design for, and how
to prove a restore works. The rules for erasing a customer's data are in [ADR-0027](../adr/0027-data-lifecycle.md).

## 1. What holds state

| Store | Holds | Source of truth | Back up |
|---|---|---|---|
| PostgreSQL 18 | everything a customer or operator entered or the platform computed: organizations, members, cloud accounts (sealed secrets), usage and cost, carbon, recommendations, plans, audit, settings, integrations | **yes** | **yes, first priority** |
| Object storage (S3 / R2 / MinIO) | generated report files, exports | no: reports are regenerated from the database | versioning on; a lost file is regenerated |
| Redis 7 | the job queue and rate-limit counters | **no** | AOF as persistence; losing it loses queued jobs only, the scheduler re-issues the periodic ones |
| Secrets in the environment | database passwords, `NOTIFICATIONS_SECRET_KEY` (the key that seals the secrets stored in the database), Auth0 client secret | no | **in the operator's secret manager, separately from the database backup** |

Two things are easy to miss:

- **The sealing key is part of the backup.** Notification destinations and the credentials of cloud accounts are stored sealed with
  `NOTIFICATIONS_SECRET_KEY` ([setup](../setup/README.md)). A restored database without the key it was sealed with has them
  unreadable; customers would have to enter them again. Keep the key where the database backup
  can never be read together with it, and keep a copy that survives the loss of the host.
- **The runtime roles.** Row-level security depends on the roles `greenops_api` and `greenops_worker`
  ([ADR-0011](../adr/0011-multi-tenancy-rls.md)). A restore must create them first (`pg_dumpall --roles-only`), or the policies
  do not load.

## 2. Targets

The platform is designed for these targets. They are what the operator commits to when deploying with the recommended
set-up; a deployment that cannot meet them must say so to its customers.

| | Target | How it is met |
|---|---|---|
| **RPO** (data lost at worst) | **15 minutes** | continuous WAL archiving to storage in another failure domain (point-in-time recovery) |
| RPO, minimum acceptable | 24 hours | a daily logical or physical backup, when WAL archiving is not available (a small or demonstration deployment) |
| **RTO** (time to serve again) | **4 hours** | restore to a standby host from the latest base backup plus WAL, re-point the three services, run `admin doctor` |
| RTO of a lost application host (database intact) | 30 minutes | redeploy the images with the same environment |

Why these numbers: what is lost with the database is mostly re-derivable. Cost, usage and grid intensity are re-read from
the customer's cloud and the grid sources (the connectors re-sync and the daily job back-fills gaps), so the real exposure
of a 15-minute RPO is the human input of that window: approvals, settings, invitations, a plan reported as applied.
That is the reason for a short RPO on the database and a long one on everything else.

**What a restore does not bring back:** nothing entered after the last archived WAL segment; sessions at the identity
provider (Auth0 owns them); anything in Redis.

## 3. Retention and erasure

- Keep daily base backups for 14 days and WAL for the same window; keep one monthly backup for 12 months if the contracts
  need it. Retention is an operations decision and is written next to the privacy policy.
- A customer's erased organization (ADR-0027) is gone from the live database at once but **stays in backups until they age
  out**. When restoring after an erasure, run the erasures recorded since the backup again (the operator console lists
  the requests with their dates; compare them with the backup time); do not hand a restored database to users before that.
- Backups hold personal data and sealed secrets: encrypt them at rest, restrict who can read them, log access.

## 4. Recovery procedure

1. Stop the three services (`api`, `worker`, `scheduler`) so nothing writes to a half-restored database.
2. Create the database host (the same major version of PostgreSQL), restore the roles, restore the base backup, replay WAL
   to the chosen moment (before the incident, not after).
3. Check the shape: `select count(*) from pg_policies` equals the number in the release's schema (40 at alpha.146), and the
   newest migration of the release is applied. Apply missing migrations as the owner role (`goose ... up`).
4. Set `DATABASE_URL` for each service, restore `NOTIFICATIONS_SECRET_KEY` (the same on all three) and the other environment, start `worker` first, then
   `api`, then `scheduler`.
5. Run `admin doctor`; open the operator console; confirm in *Diagnostics* that connections report their last sync, and let
   the scheduler catch up (it back-fills cost, usage and intensity gaps).
6. Tell customers the window of lost input.

## 5. Proving it works

A backup nobody restored is a hope. Run the check at least monthly, and after any change of PostgreSQL version, of the
backup tool or of the schema layout:

```
SRC_CONTAINER=<postgres container> SRC_DB=<database> scripts/backup-restore-check.sh
```

It dumps the database, restores it into a throwaway container of the same major version (roles first), and compares the
row count of every table and the number of RLS policies, indexes and constraints. It changes nothing in the source and
prints `RESTORE CHECK PASSED` or the mismatches, and the time the dump and the restore took. That time is the measured part
of the RTO; the rest of the RTO is the host and the people.

It checks a logical dump: it proves the data and the schema (including the policies) come back whole. It does not test WAL
archiving or the point-in-time replay; do that on the production backup tool with its own restore test and write the date
below.

### Record of checks

| Date | Environment | Result | Size | Dump + restore | Tables / rows |
|---|---|---|---|---|---|
| 2026-10-11 | demonstration stand, PostgreSQL 16 | passed: counts, 40 policies, 133 indexes, 330 constraints equal | 1.8 MB | 7 s | 59 / 252,522 |

The stand is small. Scale the time by size: restore time grows roughly with the database, and the usage tables
(partitioned by month) dominate it; measure on a copy of the real size before promising an RTO.
