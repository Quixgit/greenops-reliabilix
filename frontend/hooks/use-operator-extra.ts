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

const aggSource = z.object({
  id: z.string(), label: z.string(), measure: z.string(), granularity: z.enum(["hourly", "monthly"]), computed: z.boolean(), licence: z.string(),
  position: z.number(), pinned_regions: z.number(), regions: z.number(), hourly_readings_30d: z.number(), monthly_figures: z.number(),
  newest: z.string().nullable(), max_age_hours: z.number(), freshness: z.enum(["fresh", "aging", "stale", "none"]),
});
const aggregation = z.object({
  policy: z.object({
    priority: z.array(z.string()), overrides: z.record(z.string(), z.string()).nullable().transform((v) => v ?? {}), min_day_readings: z.number(), max_age_hours: z.number(),
    source_max_age_hours: z.record(z.string(), z.number()).nullable().transform((v) => v ?? {}), daily_request_cap: z.number(),
  }),
  sources: z.array(aggSource),
  provenance: z.object({
    days: z.number(), total: z.number(), unattributed: z.number(), precise_share: z.number(),
    rows: z.array(z.object({ source: z.string(), basis: z.string(), rows: z.number() })).nullable().transform((v) => v ?? []),
  }),
  rules: z.array(z.object({ code: z.string(), title: z.string(), text: z.string() })),
  regions_in_use: z.number(),
  divergence: z.array(z.object({
    region: z.string(), in_use: z.boolean(), ratio: z.number(), level: z.enum(["ok", "differs", "disagrees"]), used: z.string(),
    sources: z.array(z.object({
      id: z.string(), label: z.string(), g_per_kwh: z.number(), readings: z.number(), newest: z.string().nullable(), computed: z.boolean(), fresh: z.boolean(),
      vs_anchor_pct: z.number().nullable(),
    })),
    anchor: z.object({ source: z.string(), label: z.string(), month: z.string(), g_per_kwh: z.number() }).nullable(),
    inventory: z.object({ source: z.string(), label: z.string(), month: z.string(), g_per_kwh: z.number() }).nullable().optional().transform((v) => v ?? null),
  })).nullable().transform((v) => v ?? []),
});
export type Aggregation = z.infer<typeof aggregation>;
export type AggSource = z.infer<typeof aggSource>;
export const useAggregation = () => useQuery({ queryKey: ["op-aggregation"], queryFn: () => api("operator/aggregation", aggregation) });

const nul = <T extends z.ZodTypeAny>(t: T) => t.nullable();
const explainSource = z.object({
  source: z.string(), position: z.number(), pinned: z.boolean(), day_readings: z.number(), day_mean: nul(z.number()), monthly: nul(z.number()),
  latest_before: nul(z.number()), latest_before_at: nul(z.string()), age_hours: nul(z.number()), max_age_hours: z.number(), within_max_age: z.boolean(),
  latest_overall: nul(z.number()), latest_overall_at: nul(z.string()),
  verdict: z.enum(["chosen", "outranked", "too_few", "backup", "stale", "no_data"]),
});
const explanation = z.object({
  region: z.string(), day: z.string(), min_day_readings: z.number(), max_age_hours: z.number(), order: z.array(z.string()),
  sources: z.array(explainSource), chosen: nul(z.object({ g_per_kwh: z.number(), source: z.string(), basis: z.string() })),
});
export type Explanation = z.infer<typeof explanation>;
export type ExplainSource = z.infer<typeof explainSource>;
export const useExplain = (region: string, day: string) =>
  useQuery({ queryKey: ["op-explain", region, day], enabled: region !== "" && day !== "", retry: false, queryFn: () => api("operator/aggregation/explain", explanation, { region, day }) });
