# ADR-0033: Plan catalog and plan assignment

Status: accepted · 0.1.0-alpha.33 · implements the first step of [ADR-0022](0022-licensing-and-subscriptions.md)

## Decision
- The catalog is code (`internal/platform/plans`): `free`, `team`, `business`, `enterprise`. A plan is a preset of the
  `limits.*` and `modules.*` settings of the settings engine (ADR-0023); it adds no enforcement of its own.
- Assigning a plan (`PUT /operator/tenants/{id}/plan`, operator admin, reason of 3-300 characters) writes the preset as
  locked operator values for that organization (settings history) and sets `tenants.organizations.plan`
  (`platform.operator_set_plan`, which writes the reason into the organization's audit log).
- A test ties the catalog to the settings catalog: every key must exist, be operator-scope and pass validation.
- Lowering a plan never deletes data; only new resources are refused.
- Individual terms: after assigning, any single limit can still be overridden on the organization's page.

## Not decided here
Subscriptions (period, trial, status), payment provider, usage-based billing, per-organization cost of paid APIs,
customer-facing plan page. The numbers of the catalog are a proposal to confirm before the first paying customer.

## Consequences
- Organizations created before this change show `free` but keep their previous values until a plan is assigned.
- Two writes (settings, then the plan label) are not one transaction; the settings go first and a repeat completes it.
