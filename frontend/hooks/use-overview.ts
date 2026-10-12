"use client";

import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "@/lib/api/client";
import * as s from "@/lib/api/schemas";
import { setDefaultCurrency } from "@/lib/formatters";
import { periodRange, useUi } from "@/lib/store";

/** Filters shared by every Overview query; changing project/period refetches all of them. */
function useFilters() {
  const { projectId, period, custom } = useUi();
  void custom; // a new custom range re-renders the queries that depend on the period
  const { from, to } = periodRange(period);
  return { from, to, project_id: projectId ?? undefined };
}

export const useCarbonSummary = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["carbon-summary", f], queryFn: () => api("carbon/summary", s.carbonSummary, f) });
};
export const useFinopsSummary = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["finops-summary", f], queryFn: async () => { const d = await api("finops/summary", s.finopsSummary, f); setDefaultCurrency(d.currency); return d; } });
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
export const useGridSources = () => useQuery({ queryKey: ["grid-sources"], queryFn: () => api("dashboard/grid-sources", s.gridSources), staleTime: 5 * 60_000 });
export const useConnectionCosts = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["connection-costs", f], queryFn: () => api("dashboard/connections", s.connectionCosts, f) });
};
export const useRegions = () => useQuery({ queryKey: ["regions"], queryFn: () => api("dashboard/regions", s.regions) });
const countries = z.object({ items: z.array(z.object({ code: z.string(), g_per_kwh: z.number(), month: z.string(), source: z.enum(["ember", "inventory"]).default("ember") })) }).transform((o) => o.items);
/** The newest national monthly average of every country: colours the map where no cloud region is. Reference data, slow to change. */
export const useCountries = () => useQuery({ queryKey: ["countries"], queryFn: () => api("dashboard/countries", countries), staleTime: 60 * 60_000 });
export const useRegionFootprint = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["region-footprint", f], queryFn: () => api("dashboard/regions/footprint", s.regionFootprint, f) });
};
export const useOptimization = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["optimization", f], queryFn: () => api("dashboard/optimization", s.optimization, f) });
};
export const useEfficiency = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["efficiency", f], queryFn: () => api("dashboard/efficiency", s.efficiency, f) });
};
export const useActivity = () => useQuery({ queryKey: ["activity"], queryFn: () => api("dashboard/activity", s.activity, { limit: "20" }) });
export const useProjects = () => useQuery({ queryKey: ["projects"], queryFn: () => api("projects", s.projects) });
export const useConnections = () => useQuery({ queryKey: ["connections"], queryFn: () => api("cloud-accounts", s.connections) });
export const useRecommendations = (status: string) =>
  useQuery({ queryKey: ["recommendations", status], queryFn: () => api("recommendations", s.recommendations, { status: status === "all" ? undefined : status, limit: "50" }) });
export const useReports = () =>
  useQuery({ queryKey: ["reports"], queryFn: () => api("reports", s.reports), refetchInterval: (q) => (q.state.data?.some((r) => r.status === "pending") ? 3000 : false) });
const forecastSchema = z.object({ currency: z.string(), forecast: z.object({
  expected: z.number(), low: z.number(), high: z.number(), method: z.enum(["seasonal", "linear"]), spent: z.number(), lag_days: z.number(), days_left: z.number(),
}) });
/** Where the month will end: the weekday pattern and the trend, with an 80% range; the billing lag of the last days is allowed for. */
export const useForecast = () => {
  const { projectId } = useUi();
  return useQuery({ queryKey: ["forecast", projectId], queryFn: () => api("finops/forecast", forecastSchema, projectId ? { project_id: projectId } : undefined) });
};
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
export const useScheduleAdvice = (region: string | undefined, duration: number, within: number) =>
  useQuery({
    queryKey: ["schedule", region, duration, within], enabled: !!region, retry: false,
    queryFn: () => api("carbon/schedule", s.scheduleAdvice, { region, duration_hours: String(duration), within_hours: String(within) }),
  });
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
  return useQuery({ queryKey: ["k8s-workloads", f, cluster], queryFn: () => api("kubernetes/workloads", s.k8sWorkloads, { ...f, cluster: cluster || undefined, limit: "200" }) });
};
export const useTeams = () => useQuery({ queryKey: ["teams"], queryFn: () => api("teams", s.teams) });
export const useEnvironments = () => useQuery({ queryKey: ["environments"], queryFn: () => api("environments", s.environments) });
export const useAllocationRules = () => useQuery({ queryKey: ["allocation-rules"], queryFn: () => api("allocation-rules", s.allocationRules) });
export const useAllocation = () => {
  const f = useFilters();
  return useQuery({ queryKey: ["allocation", f.from, f.to], queryFn: () => api("allocation", s.allocationReport, { from: f.from, to: f.to }) });
};

/** Budgets with the consumption of the current period; a role without billing access gets an error and sees nothing. */
export const useBudgets = () =>
  useQuery({ queryKey: ["budgets"], retry: false, queryFn: () => api("finops/budgets", s.budgetList) });

/** The automation page: full jobs with plan, risk and rollback. */
export const useAutomationPlans = () =>
  useQuery({ queryKey: ["automation-jobs"], queryFn: () => api("automation/jobs", s.automationJobList, { limit: "100" }) });

/** Automation jobs, for the Overview counters only; a user without the permission simply sees none. */
export const useAutomationJobs = () =>
  useQuery({ queryKey: ["automation-jobs-lite"], retry: false, queryFn: () => api("automation/jobs", s.automationJobs, { limit: "100" }) });
