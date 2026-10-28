# Audit: what `internal/automation` can do today (input for the migration planner)

Date: 2026-10-28. Scope: read-only review of `backend/internal/automation` and the code around it.

## What exists

| Area | State |
|---|---|
| Job lifecycle | `planned -> approved -> completed / failed / rolled_back / cancelled`, one live job per recommendation, every step audited, row lock during a transition |
| Plan content | Free text: steps, a Terraform hint, shell commands, a rollback paragraph, a risk level with factors |
| Supported kinds | `ec2_resize`, `ec2_terminate`, `region_shift` (only these three) |
| Input validation | Strict patterns for instance id, instance type and region before they enter commands; injection tests |
| Approval | Separate human approval (`automation:approve`), compliance re-check at planning and approval, machines have no access |
| Execution | **None.** The platform plans; a person applies the plan and reports the outcome (ADR-0017) |

## Resource coverage

| Resource | Today |
|---|---|
| EC2 | Resize and retire plans as text. No discovery, no execution, no verification |
| EBS | Mentioned in a step ("take snapshots"); not modelled |
| RDS | Nothing |
| VPC, subnets, security groups | Nothing |
| Load balancers (ALB/NLB) | Nothing |
| DNS (Route 53) | Nothing |
| S3 | Nothing (only read for billing exports) |
| Data replication (MGN/DRS) | Nothing |
| Unknown resource type | The whole recommendation fails with `ErrUnsupported`. There is no per-resource "blocked, because ..." |

## Gaps against the migration requirements

1. **No inventory.** The platform's access is Cost Explorer, billing exports and rightsizing findings. It cannot
   list resources or their relationships (no `ec2:Describe*`, `rds:Describe*`, ...). A planner cannot recognise
   resources it cannot see.
2. **No dependency graph.** A plan is one recommendation about one resource, not a set of resources with edges.
3. **No steps.** A job has one status; there are no step records, idempotency keys, leases, resume or per-step
   audit. A crash mid-migration would leave nothing to resume from.
4. **No execution identity.** Executing needs write access (EC2, RDS, Route 53, MGN/DRS ...). Customer roles are
   read-only by design, and the security model promises that. A write path is a new trust boundary, not a feature.
5. **No test double for AWS.** Tests use fakes per provider call. Failure scenarios (lost connection, interrupted
   replication, repeated start, rollback) need a fault-injecting AWS layer that does not exist yet.
6. **Unknown types are not first-class.** The requirement "stop that part of the migration and explain why" needs
   a per-node state (`supported`, `blocked` + reason) computed before anything runs.

## Reusable as is

The job table with row locking and audit, the approval and compliance re-check pattern, strict input validation
for anything that reaches a command, tenant isolation (RLS) and the Asynq job chain.
