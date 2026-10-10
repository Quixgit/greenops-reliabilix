"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import * as s from "@/lib/api/schemas";
import { periodRange, useUi } from "@/lib/store";

/** Filters shared by every Overview query; changing project/period refetches all of them. */
function useFilters() {
  const { projectId, period } = useUi();
  const { from, to } = periodRange(period);
  return { from, to, project_id: projectId ?? undefined };
}

export const useCarbonSummary = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["carbon-summary", f], queryFn: () => api("carbon/summary", s.carbonSummary, f) });
};
export const useFinopsSummary = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["finops-summary", f], queryFn: () => api("finops/summary", s.finopsSummary, f) });
};
export const useTrend = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["trend", f], queryFn: () => api("dashboard/trend", s.trend, f) });
};
export const useProviders = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["providers", f], queryFn: () => api("dashboard/providers", s.providers, f) });
};
export const useServices = (category: string) => {
  const f = useFilters();
  return useQuery({
    queryKey: ["services", f, category],
    queryFn: () => api("dashboard/services", s.services, { ...f, category: category === "all" ? undefined : category }),
  });
};
export const useRegions = () => useQuery({ queryKey: ["regions"], queryFn: () => api("dashboard/regions", s.regions) });
export const useActivity = () => useQuery({ queryKey: ["activity"], queryFn: () => api("dashboard/activity", s.activity, { limit: "6" }) });
export const useProjects = () => useQuery({ queryKey: ["projects"], queryFn: () => api("projects", s.projects) });
export const useConnections = () => useQuery({ queryKey: ["connections"], queryFn: () => api("cloud-accounts", s.connections) });
export const useRecommendations = (status: string) =>
  useQuery({ queryKey: ["recommendations", status], queryFn: () => api("recommendations", s.recommendations, { status: status === "all" ? undefined : status, limit: "50" }) });
export const useReports = () =>
  useQuery({ queryKey: ["reports"], queryFn: () => api("reports", s.reports), refetchInterval: (q) => (q.state.data?.some((r) => r.status === "pending") ? 3000 : false) });
export const useAnomalies = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["anomalies", f], queryFn: () => api("finops/anomalies", s.anomalies, f) });
};
export const useCarbonTrend = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["carbon-trend", f], queryFn: () => api("carbon/trend", s.carbonTrend, f) });
};
export const useIntensityHistory = (region: string | undefined) => {
  const f = useFilters();
  return useQuery({ queryKey: ["intensity-history", f, region], queryFn: () => api("carbon/intensity", s.intensityHistory, { ...f, region }) });
};
export const useMethodologyVersions = () =>
  useQuery({ queryKey: ["methodology-versions"], queryFn: () => api("carbon/methodology/versions", s.methodologyVersions), staleTime: 10 * 60_000 });
export const useConnection = (id: string) => useQuery({ queryKey: ["connection", id], queryFn: () => api(`cloud-accounts/${id}`, s.connectionDetail) });
export const useSyncRuns = (id: string) =>
  useQuery({ queryKey: ["sync-runs", id], queryFn: () => api(`cloud-accounts/${id}/sync-runs`, s.syncRuns) });
export const useAlerts = (status: string) =>
  useQuery({ queryKey: ["alerts", status], queryFn: () => api("alerts", s.alerts, { status: status === "all" ? undefined : status, limit: "100" }) });
export const useK8sClusters = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["k8s-clusters", f], queryFn: () => api("kubernetes/clusters", s.k8sClusters, f) });
};
export const useK8sWorkloads = (cluster: string) => {
  const f = useFilters();
  return useQuery({ queryKey: ["k8s-workloads", f, cluster], queryFn: () => api("kubernetes/workloads", s.k8sWorkloads, { ...f, cluster: cluster || undefined, limit: "50" }) });
};
