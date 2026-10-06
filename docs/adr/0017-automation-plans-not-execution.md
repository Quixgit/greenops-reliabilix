# ADR-0017: Automation produces reviewed plans; people execute them

Status: accepted

## Context
Phase 3 of the roadmap is "automate". Executing changes in a customer's cloud needs write permissions, which
breaks the read-only access model every customer was promised, and a mistake changes production.

## Decision
- An *automation job* is created only for a recommendation that a human has already approved, and only while the
  project's data-residency policy still allows it (re-checked at planning and again at approval).
- The platform builds a plan: steps, commands, Terraform hint, rollback plan and a risk assessment. Plans are
  built only from identifiers that were validated against strict patterns (instance id, instance type, region),
  because people copy the commands into a shell. A value that does not match is an error, never escaped.
- A second, separate human decision approves the plan (`automation:approve`: owner and engineer). Admins can read
  plans; viewers, billing users and machine keys have no access to automation at all.
- The platform does not execute anything. The person who applies the plan reports the outcome
  (`completed`, `failed`, `rolled_back`; failures need a note). `completed` marks the recommendation as applied
  (idempotently, so a retry after a partial failure succeeds).
- At most one live job exists per recommendation; every step is audited in the same transaction.
- The runtime database roles have no DELETE on jobs, and the worker role has no access to the automation schema.

## Consequences
Customers keep read-only roles. If an executor is added later (Terraform or Kubernetes), it plugs in behind
`Job.CanExecute` (approved by a human, with a rollback plan) and its own, separately granted credentials.
