# ADR-0030: Kubernetes energy comes from a customer-side agent

Status: accepted (ingestion, storage and screens implemented; the reference agent is a script)

## Context
The carbon engine estimates energy from usage and cost, which is the weakest part of the figures. Kubernetes can give
measured energy per container (Kepler exports it to Prometheus). The platform cannot reach into a customer's cluster, and
should not ask for cluster credentials.

## Decision
- **Push, not pull.** A small agent inside the customer's cluster reads Kepler from Prometheus once an hour and posts the
  energy of the last completed UTC hour per workload to `POST /api/v1/kubernetes/energy`. The customer gives the platform
  nothing but an API key of the new role `agent`, which may do that and nothing else (no reading, no other endpoint).
- **A completed hour per workload is the unit.** Samples are idempotent (the same workload and hour replaces the earlier
  value), batches are all-or-nothing, up to 500 samples. A value above 5000 kWh in an hour is refused as a unit mistake.
- **Attribution, not an addition.** The nodes' energy is already in the organization's totals through the cloud bill.
  Workload energy is stored in its own table and shown on the Kubernetes page; it is never summed into the Overview, the
  dashboard or the reports. The page says so.
- **Carbon at ingestion, never guessed.** Carbon = measured kWh × PUE (the active methodology) × the stored grid intensity
  of that hour. With no reading the energy is kept and carbon stays null; an aggregate over hours with any unknown shows
  no carbon total instead of a partial one.
- **Region comes from the agent** (the cloud region of the nodes), because a cluster name says nothing about the grid.

- **Usage and requests are optional columns of the same row**: CPU core-seconds and average working set per hour, and the
  average requested cores and memory, summed over the workload's pods. Utilization is used / requested over the hours that
  have both; with none it is unknown, never 0. Recommendations (smaller requests, scheduling) are built on this later.

## Consequences
- The workload name is derived by the agent from the pod name (ReplicaSet hash and suffix removed); it is a heuristic and
  labelled as such. A label-based mapping is a later improvement.
- The table is not partitioned yet: hourly rows per workload grow slowly compared with usage records. The trigger to
  partition is the same as for usage (ADR-0005).
- Using measured workload energy to replace the cost-based estimate of the same nodes in the Overview needs a way to tell
  which cloud usage rows are those nodes; that is future work and its own decision.
