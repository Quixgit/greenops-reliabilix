"use client";

import { Boxes } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useK8sClusters, useK8sWorkloads } from "@/hooks/use-overview";
import { carbonParts, nf } from "@/lib/formatters";

const carbon = (kg: number | null) => (kg === null ? <span className="text-muted" title="Some hours have no grid data yet">—</span> : `${nf(2).format(carbonParts(kg).n)} ${carbonParts(kg).unit === "t" ? "t" : "kg"}`);

/** Used / requested. Low use of a request is the first sign of an over-sized workload; no data stays a dash. */
function Use({ pct }: { pct: number | null | undefined }) {
  if (pct === null || pct === undefined) return <span className="text-muted" title="The agent has not reported usage and requests for this workload">—</span>;
  const low = pct < 25;
  return (
    <span className={`tabular-nums ${low ? "font-medium text-amber-700" : ""}`} title={low ? "Uses less than a quarter of what it requests: a candidate for smaller requests" : undefined}>
      {nf(0).format(pct)}%
    </span>
  );
}

export function KubernetesView() {
  const [cluster, setCluster] = useState("");
  const clusters = useK8sClusters();
  const workloads = useK8sWorkloads(cluster);
  return (
    <div className="space-y-5">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Kubernetes</h1>
        <p className="mt-1 text-sm text-muted">
          Energy of workloads as measured in your clusters. It is part of the cloud energy already counted on the Overview and is
          never added to it: this page tells you <i>where inside the nodes</i> the energy goes.
        </p>
      </div>
      <Card index={0}>
        <CardHeader title="Clusters" />
        {clusters.isLoading ? <Skeleton className="h-20" /> : clusters.isError ? <ErrorState /> : clusters.data!.length === 0 ? (
          <EmptyState icon={Boxes} className="py-10"
            text="No Kubernetes data yet. Create an API key with the access “Kubernetes agent” in Settings, then run the agent in your cluster (see docs/setup/kubernetes.md)."
            action={{ href: "/settings", label: "Open Settings" }} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs text-muted"><tr><th className="py-2 pr-3 font-medium">Cluster</th><th className="pr-3 font-medium">Namespaces</th><th className="pr-3 font-medium">Workloads</th><th className="pr-3 text-right font-medium">Energy</th><th className="pr-3 text-right font-medium">CO₂e</th><th className="font-medium">Last hour</th></tr></thead>
              <tbody className="divide-y divide-line/70">
                {clusters.data!.map((c) => (
                  <tr key={c.cluster} className={cluster === c.cluster ? "bg-slate-50" : ""}>
                    <td className="py-2 pr-3"><button type="button" onClick={() => setCluster(cluster === c.cluster ? "" : c.cluster)} className="font-medium text-primary hover:underline">{c.cluster}</button></td>
                    <td className="pr-3">{c.namespaces}</td><td className="pr-3">{c.workloads}</td>
                    <td className="pr-3 text-right tabular-nums">{nf(1).format(c.energy_kwh)} kWh</td>
                    <td className="pr-3 text-right tabular-nums">{carbon(c.carbon_kg_co2e)}</td>
                    <td className="whitespace-nowrap text-xs text-muted">{c.last_hour.replace("T", " ").replace(":00:00Z", ":00 UTC")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      {!!clusters.data?.length && (
        <Card index={1}>
          <CardHeader title={cluster ? `Workloads in ${cluster}` : "Workloads"} action={cluster ? <button className="text-sm text-primary" onClick={() => setCluster("")}>All clusters</button> : undefined} />
          {workloads.isLoading ? <Skeleton className="h-24" /> : workloads.isError ? <ErrorState /> : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="text-xs text-muted"><tr><th className="py-2 pr-3 font-medium">Workload</th><th className="pr-3 font-medium">Namespace</th><th className="pr-3 font-medium">Kind</th><th className="pr-3 text-right font-medium">Energy</th><th className="pr-3 text-right font-medium">CO₂e</th><th className="pr-3 text-right font-medium" title="CPU used / CPU requested">CPU use</th><th className="pr-3 text-right font-medium" title="Memory used / memory requested">Memory use</th><th className="text-right font-medium">Hours</th></tr></thead>
                <tbody className="divide-y divide-line/70">
                  {workloads.data!.map((w) => (
                    <tr key={`${w.cluster}/${w.namespace}/${w.workload}`}>
                      <td className="py-2 pr-3 font-medium">{w.workload}{!cluster && <span className="ml-2 text-xs font-normal text-muted">{w.cluster}</span>}</td>
                      <td className="pr-3">{w.namespace}</td><td className="pr-3 text-muted">{w.kind}</td>
                      <td className="pr-3 text-right tabular-nums">{nf(2).format(w.energy_kwh)} kWh</td>
                      <td className="pr-3 text-right tabular-nums">{carbon(w.carbon_kg_co2e)}</td>
                      <td className="pr-3 text-right"><Use pct={w.cpu_utilization_pct} /></td>
                      <td className="pr-3 text-right"><Use pct={w.memory_utilization_pct} /></td>
                      <td className="text-right tabular-nums text-muted">{w.hours}{w.hours_without_grid_data > 0 && <span title="hours without grid data"> ({w.hours_without_grid_data} ?)</span>}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="mt-3 text-xs text-muted">
            Measured energy (Kepler) × PUE × grid intensity of that hour. “Use” is what the workload consumed of what it requested over the hours the agent reported both; it is the base for right-sizing, which is not automated yet. A dash means some hours had no grid reading yet; the energy is
            kept and the carbon appears once a reading exists. <Link href="/carbon" className="text-primary hover:underline">How figures are calculated</Link>.
          </p>
        </Card>
      )}
    </div>
  );
}
