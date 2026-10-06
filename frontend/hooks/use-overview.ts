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
