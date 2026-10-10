# Kubernetes energy

Measured energy of your workloads, from [Kepler](https://sustainable-computing.io/) through Prometheus, pushed to
Reliabilix by a small agent. See [ADR-0030](../adr/0030-kubernetes-energy-agent.md).

## 1. In the cluster: Kepler and Prometheus

Install Kepler and make sure Prometheus scrapes it. The query the agent runs is

```
sum by (container_namespace, pod_name) (increase(kepler_container_joules_total[1h])) / 3.6e6
```

(joules to kWh). Label names differ between Kepler versions; adjust the script if yours differ.

## 2. In Reliabilix

1. Settings → API keys → create a key with the access **Kubernetes agent**. Copy it (it is shown once).
2. Note the **project id** the cluster belongs to.

## 3. Run the agent

`scripts/kepler-push.sh` is a reference agent (curl + jq). Environment: `PROM_URL`, `API_URL`, `API_KEY`, `PROJECT_ID`,
`CLUSTER` (a stable name), `REGION` (the cloud region of the nodes, for the grid intensity). Run it every hour:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata: {name: reliabilix-energy, namespace: monitoring}
spec:
  schedule: "7 * * * *"            # a few minutes past the hour, so the last hour is complete
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: push
              image: ghcr.io/quixgit/reliabilix/agent:0.1.0-alpha.56   # pin the exact version
              envFrom: [{secretRef: {name: reliabilix-agent}}]   # API_KEY, API_URL, PROJECT_ID, CLUSTER, REGION, PROM_URL
```

To back-fill a past hour: `HOUR=2026-10-10T09:00:00Z bash kepler-push.sh`. `DRY_RUN=1` prints what would be sent.

## What you see

Kubernetes page: clusters and the workloads that use the most energy, with CO₂e where a grid reading exists for that
hour. The figures are part of your cloud energy already counted on the Overview; they are not added to it.

The script is a reference and has been checked against sample Prometheus output, not against a live cluster.
