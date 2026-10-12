# ADR-0063: Kubernetes cost allocation

Status: accepted

## Context
The Kubernetes page showed energy and rightsizing, but not the money: what a cluster costs and which namespace or workload it is spent on. That is the first thing Kubecost and the FinOps teams that use it
look for, and the data to answer it was already collected: per workload and hour, what it requested and what it used.

## Decision
**Where the cost comes from.** Nodes are billed as ordinary cloud services (virtual machines, node pools), so the cost of a cluster is read from the bill, not guessed from list prices. The services that pay for a cluster are
chosen per cluster (`kubernetes.cluster_pools`, `PUT /kubernetes/clusters/pool`; the screen lists the project's compute and container services with their cost). Until one is chosen **every compute cost of the project is
assumed to be the cluster, and the screen says so**. The cost is the billed cost in the reporting currency, converted like every other total.

**How it is split** (`kubernetes/domain/cost.go`, a pure function): for each day, the cost of the hours the agent reported (the day's cost times hours reported over 24) is divided into CPU (65%) and memory (35%); each part is shared among the workloads
in proportion to what they **hold**: per hour the larger of what they request and what they use (a requested core is not available to anybody else whether or not it is busy). The weights are core-hours and GiB-hours.
- If the agent also reports the nodes' **capacity** (allocatable CPU and memory per hour: `capacity` in `POST /kubernetes/energy`, `kube_node_status_allocatable` in the reference agent), the price of a core-hour and a GiB-hour is the cost over the capacity, and what
  no workload holds is **idle**. If the capacity is not reported, the whole cost is shared out and idle is reported as **null (unknown), never zero**. Holds above capacity are scaled down so that nothing is allocated that was not paid.
- **Unused requests** (waste) are the part of a workload's cost that pays for requests above its use, in CPU and memory; a workload that reports no requests has none. It is the money size of what rightsizing recommends.
- A workload that reported neither requests nor use holds nothing and gets no cost; the screen counts them. A day with cost and no workload holding anything is shown as unallocated.

**What it is not.** Only the hours the agent reported are costed (a cluster whose agent runs a few hours a day shows that share). The 65/35 split is a stated convention, not a measurement of the node price; for a bill with GPUs or very
memory-heavy nodes it is rough. Shared costs outside the nodes (control plane, load balancers, storage, networking) are not in the pool unless their services are chosen. Carbon and energy are separate and unchanged. Verified on the stand with a synthetic cluster (the pool equals the
compute cost in the database to the cent, and allocated plus idle equals the pool); no live Kepler cluster was available.

## Not done
Per-label (team, product) cost of Kubernetes workloads through pod labels; persistent-volume and network cost per workload; spot and reserved node pricing per node; cost per pod; a workload without requests allocated by energy as a fallback.
