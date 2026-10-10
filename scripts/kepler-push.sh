#!/usr/bin/env bash
# Reference agent: reads the energy of the last COMPLETED UTC hour per workload from Prometheus (Kepler metrics) and
# pushes it to Reliabilix. Run it hourly (a Kubernetes CronJob, see docs/setup/kubernetes.md). Needs curl and jq.
#
#   PROM_URL    Prometheus base URL, e.g. http://prometheus.monitoring.svc:9090
#   API_URL     Reliabilix API, e.g. https://api.example.com
#   API_KEY     an API key with the access "Kubernetes agent"
#   PROJECT_ID  the Reliabilix project the cluster belongs to
#   CLUSTER     a stable name for this cluster
#   REGION      the cloud region of the nodes, e.g. eu-central-1
#   HOUR        optional, an ISO hour (2026-10-10T09:00:00Z) to back-fill; default: the last completed hour
#   DRY_RUN     optional, "1" prints the payload instead of sending it
set -euo pipefail

: "${PROM_URL:?}" "${API_URL:?}" "${API_KEY:?}" "${PROJECT_ID:?}" "${CLUSTER:?}" "${REGION:?}"

end_epoch="$(date -u -d "${HOUR:-$(date -u +%Y-%m-%dT%H:00:00Z)}" +%s)"   # the end of the hour to report
if [ -n "${HOUR:-}" ]; then end_epoch=$((end_epoch + 3600)); fi
start_epoch=$((end_epoch - 3600))
period_start="$(date -u -d "@${start_epoch}" +%Y-%m-%dT%H:00:00Z)"

# Joules consumed by each container in the hour, summed per pod, converted to kWh (1 kWh = 3.6e6 J).
query='sum by (container_namespace, pod_name) (increase(kepler_container_joules_total[1h])) / 3.6e6'
resp="$(curl -fsS --max-time 60 -G "${PROM_URL%/}/api/v1/query" --data-urlencode "query=${query}" --data-urlencode "time=${end_epoch}")"

# pod -> workload: strip the ReplicaSet hash and the pod suffix (a real hash contains a digit). The name is a heuristic;
# anything it does not recognise is reported as the pod itself.
payload="$(jq -c --arg project "$PROJECT_ID" --arg cluster "$CLUSTER" --arg region "$REGION" --arg start "$period_start" '
  def workload($p):
    if   ($p | test("-(?=[a-z0-9]*[0-9])[a-z0-9]{8,10}-[a-z0-9]{5}$")) then {name: ($p | sub("-(?=[a-z0-9]*[0-9])[a-z0-9]{8,10}-[a-z0-9]{5}$"; "")), kind: "deployment"}
    elif ($p | test("-[0-9]+$"))                    then {name: ($p | sub("-[0-9]+$"; "")), kind: "statefulset"}
    elif ($p | test("-[a-z0-9]{5}$"))               then {name: ($p | sub("-[a-z0-9]{5}$"; "")), kind: "daemonset"}
    else {name: $p, kind: "pod"} end;
  [.data.result[]
    | select(.metric.container_namespace != null and .metric.pod_name != null)
    | (workload(.metric.pod_name)) as $w
    | {cluster: $cluster, namespace: .metric.container_namespace, workload: $w.name, kind: $w.kind,
       period_start: $start, energy_kwh: (.value[1] | tonumber), region: $region}]
  | group_by([.namespace, .workload, .kind])
  | map(.[0] + {energy_kwh: (map(.energy_kwh) | add)})
  | {project_id: $project, samples: .}' <<<"$resp")"

n="$(jq '.samples | length' <<<"$payload")"
if [ "$n" -eq 0 ]; then echo "no Kepler samples for ${period_start}"; exit 0; fi
if [ "${DRY_RUN:-0}" = "1" ]; then echo "$payload"; exit 0; fi

# at most 500 samples per request
jq -c --argjson size 500 '. as $p | [range(0; ($p.samples | length); $size)] | map({project_id: $p.project_id, samples: $p.samples[.:(. + $size)]}) | .[]' <<<"$payload" |
while read -r chunk; do
  curl -fsS --max-time 60 -X POST "${API_URL%/}/api/v1/kubernetes/energy" \
    -H "Authorization: Bearer ${API_KEY}" -H "Content-Type: application/json" --data "$chunk"
  echo
done
