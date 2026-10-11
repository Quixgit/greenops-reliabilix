"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { api } from "@/lib/api/client";

const day = z.object({
  day: z.string(), organizations: z.number(), users: z.number(), projects: z.number(), connections: z.number(), reports: z.number(),
  gate_checks: z.number(), recommendations: z.number(), deliveries: z.number(), deliveries_failed: z.number(), errors: z.number(), active_organizations: z.number(),
});
export type MetricDay = z.infer<typeof day>;
const metrics = z.object({
  days: z.number(), series: z.array(day),
  totals: z.object({
    organizations: z.number(), users: z.number(), projects: z.number(), connections: z.number(), reports: z.number(), recommendations: z.number(),
    open_errors: z.number(), announcements: z.number(), pending_erasures: z.number(),
  }),
  plans: z.array(z.object({ plan: z.string(), organizations: z.number() })),
  subscriptions: z.array(z.object({ status: z.string(), organizations: z.number() })),
});
export type OperatorMetrics = z.infer<typeof metrics>;
export const useOperatorMetrics = (days: number) => useQuery({ queryKey: ["op-metrics", days], queryFn: () => api("operator/metrics", metrics, { days: String(days) }), staleTime: 60_000 });

const announcement = z.object({
  id: z.string(), kind: z.enum(["update", "maintenance", "notice", "incident"]), severity: z.enum(["info", "warning", "critical"]), title: z.string(), body: z.string(),
  path: z.string(), audience_plans: z.array(z.string()).default([]), email_copy: z.boolean(), created_by: z.string(), created_at: z.string(),
  recipients: z.number(), withdrawn_at: z.string().nullable(), read_by: z.number(),
});
export type Announcement = z.infer<typeof announcement>;
export const useAnnouncements = () => useQuery({ queryKey: ["op-announcements"], queryFn: () => api("operator/announcements", z.object({ items: z.array(announcement) })) });

export type AnnouncementInput = { kind: string; severity: string; title: string; body: string; path: string; audience_plans: string[]; email_copy: boolean };
export function useSendAnnouncement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (v: AnnouncementInput) => api("operator/announcements", z.object({ id: z.string(), recipients: z.number(), mailed_organizations: z.number() }), undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["op-announcements"] }); void qc.invalidateQueries({ queryKey: ["op-metrics"] }); },
  });
}
export function useWithdrawAnnouncement() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`operator/announcements/${id}/withdraw`, z.object({ removed: z.number() }), undefined, { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["op-announcements"] }),
  });
}

const overview = z.object({
  overview: z.object({
    id: z.string(), name: z.string(), plan: z.string(), created_at: z.string(), members: z.number(), invitations_open: z.number(), api_keys: z.number(),
    projects: z.number(), connections: z.number(), connections_failing: z.number(), reports: z.number(), destinations: z.number(), gate_checks: z.number(),
    last_activity: z.string().nullable(),
    erasure_requested: z.object({ at: z.string(), execute_after: z.string(), by: z.string() }).nullable(),
  }),
  timeline: z.array(z.object({ at: z.string(), actor: z.string(), action: z.string(), target: z.string() })),
});
export const useOrgOverview = (id: string) => useQuery({ queryKey: ["op-org-overview", id], queryFn: () => api(`operator/tenants/${id}/overview`, overview) });
export function useErasure(id: string) {
  const qc = useQueryClient();
  const done = () => { void qc.invalidateQueries({ queryKey: ["op-org-overview", id] }); void qc.invalidateQueries({ queryKey: ["op-metrics"] }); };
  return {
    request: useMutation({ mutationFn: (v: { grace_days: number; reason: string }) => api(`operator/tenants/${id}/erasure`, z.unknown(), undefined, { method: "POST", body: JSON.stringify(v) }), onSuccess: done }),
    cancel: useMutation({ mutationFn: (reason: string) => api(`operator/tenants/${id}/erasure/cancel`, z.unknown(), undefined, { method: "POST", body: JSON.stringify({ reason }) }), onSuccess: done }),
  };
}
