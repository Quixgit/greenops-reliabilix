"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import * as s from "@/lib/api/schemas";

// ---- tenant settings ----
export const useNotificationCatalog = () => useQuery({ queryKey: ["notif-catalog"], queryFn: () => api("notifications/catalog", s.notificationCatalog) });
export const useDestinations = () => useQuery({ queryKey: ["notif-destinations"], queryFn: () => api("notifications/destinations", s.destinations) });
export const useDeliveries = () =>
  useQuery({
    queryKey: ["notif-deliveries"], queryFn: () => api("notifications/deliveries", s.deliveries, { limit: "30" }),
    refetchInterval: (q) => (q.state.data?.some((d) => d.status === "pending") ? 4000 : false), // follow a test message until it is delivered
  });
export const useMembers = () => useQuery({ queryKey: ["members"], queryFn: () => api("members", s.members) });
export const useInvitations = () => useQuery({ queryKey: ["invitations"], queryFn: () => api("invitations", s.invitations) });
export const useApiKeys = () => useQuery({ queryKey: ["api-keys"], queryFn: () => api("api-keys", s.apiKeys) });

// ---- operator console ----
export const useOperatorTenants = () => useQuery({ queryKey: ["op-tenants"], queryFn: () => api("operator/tenants", s.operatorTenants) });
export const useOperatorStats = (hours: number) =>
  useQuery({ queryKey: ["op-stats", hours], queryFn: () => api("operator/notifications/stats", s.operatorStats, { hours: String(hours) }) });
export const useOperatorDeliveries = (status: string) =>
  useQuery({
    queryKey: ["op-deliveries", status],
    queryFn: () => api("operator/notifications/deliveries", s.operatorDeliveries, { status: status === "all" ? undefined : status, limit: "100" }),
  });
export const useOperatorConnections = () => useQuery({ queryKey: ["op-connections"], queryFn: () => api("operator/connections/unhealthy", s.operatorConnections) });
export const useOperatorSystem = () => useQuery({ queryKey: ["op-system"], queryFn: () => api("operator/system", s.operatorSystem) });
export const useOperatorMe = () => useQuery({ queryKey: ["op-me"], queryFn: () => api("operator/me", s.operatorMe), staleTime: 5 * 60_000 });
export const useOperatorErrors = (status: string, service: string) =>
  useQuery({
    queryKey: ["op-errors", status, service],
    queryFn: () => api("operator/errors", s.operatorErrors, { status, service: service === "all" ? undefined : service, limit: "100" }),
    refetchInterval: 30_000,
  });
export const useOperatorIntegrations = () =>
  useQuery({ queryKey: ["op-integrations"], queryFn: () => api("operator/integrations", s.managedIntegrations) });
