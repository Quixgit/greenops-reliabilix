# ADR-0031: Kubernetes right-sizing from measured usage

Status: accepted (implemented, migration 0026)

## Context
Workloads usually ask for far more CPU and memory than they use. With the agent reporting usage and requests per hour
(ADR-0030) the platform can say where requests are oversized.

## Decision
- **A new recommendation type `k8s_rightsizing`** in the existing workflow (open, approved, applied, dismissed), created by
  the recommendations job from two weeks of measurements per workload.
- **Conservative rule.** The recommended request is the *peak* hourly usage of the window plus 25 % headroom (hourly
  averages hide spikes, so the mean is not used), never below 50 millicores or 64 MiB, and a change is proposed only when
  it is at most 60 % of the current request. A workload needs at least 72 hours that have both usage and a request, and
  only deployments and statefulsets are sized. CPU and memory are judged separately; only the oversized one is proposed.
- **No saving is claimed.** Smaller requests free reserved capacity; fewer nodes follow only if the scheduler can pack
  better. Energy and cost impact stay "not estimated" and the recommendation says why. Because it claims no saving, it is
  not measured afterwards by the impact verification (ADR-0025).
- **The platform changes nothing in the cluster.** A person applies it (for example in the manifest) and marks it applied.
  Automation plans are not offered for this type.
- **Confidence** grows with the hours of evidence (2 weeks = 0.85, never higher).

## Consequences
- A spike shorter than an hour can still exceed the recommended request; the screen says to check spikes first.
- A later step can use the Vertical Pod Autoscaler recommender or percentile data from the agent (p95/p99) for a sharper
  rule; the stored hourly means are the limit of this version.
