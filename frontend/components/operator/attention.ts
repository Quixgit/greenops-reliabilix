"use client";

import { IconBell as Bell, IconCloud as Cloud, IconDatabase as Database, IconPlug as Plug, IconWarning as AlertTriangle } from "@/components/ui/icons";
import { useMemo } from "react";
import { useOperatorConnections, useOperatorDataSources, useOperatorErrors, useOperatorStats, useOperatorSystem } from "@/hooks/use-admin";

export type AttentionItem = { key: string; text: string; href: string; icon: typeof Bell };

/**
 * What needs an operator now: open errors, failing connections, failed deliveries, missing integrations, failing sources.
 * One definition for the Overview list and the header bell, so the two never disagree. Counts and statuses only.
 */
export function useOperatorAttention(): { items: AttentionItem[]; loading: boolean; failedNotifications: number | undefined } {
  const errors = useOperatorErrors("open", "all");
  const stats = useOperatorStats(24);
  const conns = useOperatorConnections();
  const system = useOperatorSystem();
  const sources = useOperatorDataSources();
  const failed24 = stats.data?.filter((s) => s.status === "failed").reduce((n, s) => n + s.count, 0);
  const items = useMemo(() => {
    const out: AttentionItem[] = [];
    if (errors.data?.length) out.push({ key: "err", text: `${errors.data.length} open error group${errors.data.length === 1 ? "" : "s"}`, href: "/operator/errors", icon: AlertTriangle });
    if (conns.data?.length) out.push({ key: "conn", text: `${conns.data.length} cloud connection${conns.data.length === 1 ? "" : "s"} failing to sync`, href: "/operator/connections", icon: Cloud });
    if (failed24) out.push({ key: "notif", text: `${failed24} failed notification${failed24 === 1 ? "" : "s"} in 24 hours`, href: "/operator/notifications", icon: Bell });
    const missing = system.data?.integrations.filter((i) => i.status === "missing" && !/^(planned|optional)/.test(i.note)) ?? [];
    if (missing.length) out.push({ key: "int", text: `${missing.length} integration${missing.length === 1 ? "" : "s"} not set up: ${missing.map((m) => m.name).join(", ")}`, href: "/operator/data-sources", icon: Plug });
    const bad = sources.data?.filter((s) => s.state === "attention") ?? [];
    if (bad.length) out.push({ key: "src", text: `${bad.length} data source${bad.length === 1 ? "" : "s"} with failing connections`, href: "/operator/data-sources", icon: Database });
    return out;
  }, [errors.data, conns.data, failed24, system.data, sources.data]);
  return { items, loading: [errors, stats, conns, system, sources].some((q) => q.isLoading), failedNotifications: failed24 };
}
