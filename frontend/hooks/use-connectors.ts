"use client";

import { useQueries } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import * as s from "@/lib/api/schemas";
import type { SyncRun } from "@/lib/api/schemas";
import { useConnectionCosts, useConnections, useK8sClusters, useProjects, useRegionFootprint, useRegions } from "./use-overview";
import { useDestinations } from "./use-admin";

/** Recent sync runs per connection (the same cache entries the connection page uses). */
export function useConnectionRuns(ids: string[]) {
  const results = useQueries({
    queries: ids.map((id) => ({ queryKey: ["sync-runs", id], queryFn: () => api(`cloud-accounts/${id}/sync-runs`, s.syncRuns), staleTime: 30_000 })),
  });
  const byId: Record<string, SyncRun[]> = {};
  ids.forEach((id, i) => { byId[id] = results[i]?.data ?? []; });
  return { byId, loading: results.some((r) => r.isLoading) };
}

/** Everything the Connectors page is built from, in one place (and the same facts the Overview tile uses). */
export function useConnectorData() {
  const connections = useConnections();
  const clusters = useK8sClusters();
  const destinations = useDestinations();
  const projects = useProjects();
  const footprint = useRegionFootprint();
  const costs = useConnectionCosts();
  const grid = useRegions();
  const runs = useConnectionRuns((connections.data ?? []).map((c) => c.id));
  return { connections, clusters, destinations, projects, footprint, costs, grid, runs };
}
