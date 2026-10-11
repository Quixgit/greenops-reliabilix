"use client";

import { useQueries } from "@tanstack/react-query";
import { IconCheck as Check, IconBell as BellRing, IconCloud as Cloud, IconFolder as FolderKanban, IconGauge as Gauge, IconPin as MapPinned, IconPlay as Rocket, IconWallet as PiggyBank } from "@/components/ui/icons";
import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, type CSSProperties } from "react";
import { z } from "zod";
import { Card, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useDestinations } from "@/hooks/use-admin";
import { useBudgets, useConnections, useEfficiency, useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { cn } from "@/lib/utils";

const policy = z.object({ allowed_regions: z.array(z.string()) });

/**
 * Why a number may be missing and what unlocks it. Each step is checked against what the organization actually has,
 * so the list shrinks as the platform is set up and tells the truth about every empty block above.
 */
export function SetupGuide() {
  const t = useT();
  const conns = useConnections();
  const projects = useProjects();
  const budgets = useBudgets();
  const eff = useEfficiency();
  const dests = useDestinations();
  const pols = useQueries({ queries: (projects.data ?? []).map((p) => ({ queryKey: ["policy", p.id], queryFn: () => api(`projects/${p.id}/policy`, policy) })) });

  const steps = useMemo(() => {
    const synced = (conns.data ?? []).some((c) => !!c.last_sync_at);
    const regions = pols.length > 0 && pols.every((p) => p.data) && pols.some((p) => (p.data?.allowed_regions.length ?? 0) > 0);
    const list: { id: string; icon: LucideIcon; done: boolean; href: string }[] = [
      { id: "connect", icon: Cloud, done: (conns.data?.length ?? 0) > 0, href: "/connectors" },
      { id: "sync", icon: Rocket, done: synced, href: "/connectors" },
      { id: "regions", icon: MapPinned, done: regions, href: "/settings#policies" },
      { id: "units", icon: Gauge, done: !!eff.data?.has_units, href: "/projects?units=1" },
      { id: "budget", icon: PiggyBank, done: (budgets.data?.length ?? 0) > 0, href: "/settings" },
      { id: "notify", icon: BellRing, done: (dests.data ?? []).some((d) => d.enabled), href: "/settings#notifications" },
    ];
    return list;
  }, [conns.data, pols, eff.data, budgets.data, dests.data]);

  const loading = conns.isLoading || projects.isLoading || eff.isLoading;
  const done = steps.filter((s) => s.done).length;
  const pct = Math.round((done / steps.length) * 100);
  if (!loading && done === steps.length) return null; // everything is set up: nothing to explain

  return (
    <Card index={1} interactive={false}>
      <CardHeader icon={<FolderKanban className="size-[18px] text-leaf-deep" />} title={t("ov2.setup.title")} info={t("info.setup")}
        action={<span className="text-xs font-semibold tabular-nums text-muted">{done}/{steps.length}</span>} />
      <div className="mb-3 h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="bar-x h-full rounded-full bg-leaf-deep/80" style={{ width: `${Math.max(4, pct)}%` }} /></div>
      {loading ? <Skeleton className="h-48" /> : (
        <ul className="space-y-1">
          {steps.map((s, i) => (
            <li key={s.id} className="rise" style={{ "--i": i } as CSSProperties}>
              <Link href={s.href} className={cn("group flex items-start gap-3 rounded-xl p-2 transition-colors hover:bg-slate-50", s.done && "opacity-60")}>
                <span className={cn("mt-0.5 flex size-8 shrink-0 items-center justify-center transition-transform duration-300 group-hover:scale-110", s.done ? "text-leaf-deep" : "text-slate-400")}>
                  {s.done ? <Check className="size-5" strokeWidth={2.6} /> : <s.icon className="size-8" strokeWidth={1.4} />}
                </span>
                <span className="min-w-0">
                  <span className="block text-[13px] font-medium text-ink">{t(`ov2.setup.${s.id}`)}</span>
                  <span className="block text-xs leading-snug text-muted">{t(`ov2.setup.${s.id}_why`)}</span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
