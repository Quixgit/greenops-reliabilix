# ADR-0048: A dry run for automation plans

Status: accepted

## Context
[ADR-0017](0017-automation-plans-not-execution.md) keeps the platform from changing a customer's cloud: it builds a plan,
a person approves it and applies it with their own tooling. The roadmap also asks for "executors with a dry run".
A real executor needs write credentials, which breaks the read-only access model, so it stays out. What a person (or a
pipeline that applies the plan) still lacks is a way to ask, just before applying, "is this plan still valid?". Between
planning and applying, the recommendation can be withdrawn, applied elsewhere or changed, and the project's data-residency
policy can change.

## Decision
- `POST /automation/jobs/{id}/dry-run` (permission `automation:read`) runs the checks and returns a report. It is a
  read: no job, recommendation, audit row or cloud resource changes, and nothing is executed. The report says
  `executed: false` for a machine reading it.
- Checks: the job is still open (planned or approved); a person approved it (`Job.CanExecute`); a rollback plan exists;
  the recommendation is still approved; the data-residency policy still allows it; the plan equals what the planner builds
  from the recommendation today (drift means plan again); every command has the shape the planner produces
  (`aws ec2 <verb>` with plain-token options), so a value changed in the database cannot smuggle shell syntax into a
  command shown as runnable.
- `ready` is true only for an approved plan with every check passed. A plan that is only planned shows its commands for
  review with `approval` failing.
- A command the plan keeps commented out on purpose (the final terminate after an observation period) is listed in
  `held`, never in `would_run`.
- The pure part is `domain.DryRun`; the service reads the current recommendation and policy verdict through the existing
  port. There is no new table and no migration.

## Consequences
- The checks that guard a real executor already exist in one place: a future Terraform or Kubernetes executor can call
  `domain.DryRun` before acting and run only when `ready` is true, behind its own separately granted credentials.
- The report is a point-in-time answer: it does not reserve anything, so the person still applies at their own risk and
  reports the result as before.
