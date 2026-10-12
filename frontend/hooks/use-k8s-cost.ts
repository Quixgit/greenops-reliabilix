"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { api, apiVoid } from "@/lib/api/client";
import { periodRange, useUi } from "@/lib/store";

const num = z.number().nullable();
const cost = z.object({
  currency: z.string(), cpu_share: z.number(),
  clusters: z.array(z.object({
    project_id: z.string(), cluster: z.string(), pool_services: z.array(z.string()).default([]), pool_assumed: z.boolean(), pool_cost: z.number(), allocated_cost: z.number(),
    idle_cost: num, idle_days: z.number(), days: z.number(), unallocated_cost: z.number().default(0), workloads_without_usage: z.number().default(0), waste_cost: z.number().default(0),
    namespaces: z.array(z.object({ namespace: z.string(), cost: z.number(), share_pct: z.number(), waste_cost: z.number(), workloads: z.number() })).default([]),
    workloads: z.array(z.object({ namespace: z.string(), workload: z.string(), cost: z.number(), cpu_cost: z.number(), memory_cost: z.number(), waste_cost: z.number(), cpu_utilization_pct: num, memory_utilization_pct: num, hours: z.number() })).default([]),
  })),
});
export type K8sCost = z.infer<typeof cost>;
export type K8sClusterCost = K8sCost["clusters"][number];
const pool = z.object({ services: z.array(z.string()).default([]), currency: z.string(), candidates: z.array(z.object({ name: z.string(), cost: z.number() })).default([]) });

function useFilters() {
  const { projectId, period, custom } = useUi();
  void custom;
  const { from, to } = periodRange(period);
  return { from, to, project_id: projectId ?? undefined };
}

/** What the clusters that reported in the selected period cost, split over their namespaces and workloads. */
export const useK8sCost = (cluster: string) => {
  const f = useFilters();
  return useQuery({ queryKey: ["k8s-cost", f, cluster], queryFn: () => api("kubernetes/cost", cost, { ...f, cluster: cluster || undefined }) });
};

/** The billed services that pay for a cluster, and the ones that could. */
export const usePoolChoice = (projectId: string, cluster: string, enabled: boolean) => {
  const f = useFilters();
  return useQuery({ queryKey: ["k8s-pool", projectId, cluster, f.from, f.to], enabled, queryFn: () => api("kubernetes/clusters/pool", pool, { from: f.from, to: f.to, project_id: projectId, cluster }) });
};

export function useSetPool() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: { projectId: string; cluster: string; services: string[] }) =>
      apiVoid("kubernetes/clusters/pool", { method: "PUT", body: JSON.stringify({ project_id: v.projectId, cluster: v.cluster, services: v.services }) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["k8s-pool"] }); void qc.invalidateQueries({ queryKey: ["k8s-cost"] }); },
  });
}
