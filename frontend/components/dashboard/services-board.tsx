"use client";

import { ArrowDown, ArrowUp, HardDrive, Network, Server, Table2 } from "lucide-react";
import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import { IconCluster as Boxes, IconDatabase as Database } from "@/components/ui/icons";
import { useEffect, useMemo, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { Sparkline } from "@/components/charts/sparkline";
import { useFinopsSummary, useServices } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCarbon, formatCost, formatPct, halfTrend } from "@/lib/formatters";
import { share } from "@/lib/overview";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

const ICON: Record<string, { icon: LucideIcon; tile: string }> = {
  compute: { icon: Server, tile: "from-orange-300 to-orange-500" },
  storage: { icon: HardDrive, tile: "from-teal-300 to-teal-500" },
  database: { icon: Database, tile: "from-indigo-300 to-violet-500" },
  networking: { icon: Network, tile: "from-sky-300 to-blue-500" },
  other: { icon: Boxes, tile: "from-amber-300 to-orange-400" },
};
const CATS = ["all", "compute", "storage", "database", "networking", "other"] as const;

/** Every service ranked by cost with its share of cost and of emissions side by side: the heavy emitters stand out. */
export function ServicesBoard() {
  const t = useT();
  const [tab, setTab] = useState<string>("all");
  const q = useServices(tab);
  const cur = useFinopsSummary().data?.currency || "USD";
  const focus = useUi((s) => s.focusService);
  const setFocus = useUi((s) => s.setFocusService);
  useEffect(() => { if (!focus) return; const id = setTimeout(() => setFocus(null), 4000); return () => clearTimeout(id); }, [focus, setFocus]);
  const rows = useMemo(() => [...(q.data ?? [])].sort((a, b) => b.cost - a.cost), [q.data]);
  const totalCost = rows.reduce((n, r) => n + r.cost, 0), totalCarbon = rows.reduce((n, r) => n + r.carbon_kg_co2e, 0);
  const pg = usePaged(rows, 10);

  return (
    <Card index={10} interactive={false} id="services-table" className="min-w-0">
      <CardHeader title={t("ov2.svc.title")} info={t("info.services")} />
      <div className="mb-3"><PillTabs value={tab} onChange={(v) => { setTab(v); pg.setPage(1); }} tabs={CATS.map((c) => ({ value: c, label: t(`ov2.cat.${c}`) }))} /></div>
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError ? <ErrorState /> : rows.length === 0 ? <EmptyState icon={Table2} text={t("ov2.svc.empty")} /> : (
        <div key={tab}>
          <div className="hidden grid-cols-[minmax(0,1.6fr)_minmax(0,1.4fr)_96px_96px_132px] gap-4 px-2 pb-2 text-xs font-medium text-muted lg:grid">
            <span>{t("ov2.svc.service")}</span><span>{t("ov2.svc.shares")}</span><span className="text-right">{t("ov2.svc.cost")}</span><span className="text-right">CO₂e</span><span>{t("ov2.svc.trend")}</span>
          </div>
          <ul>
            {pg.slice.map((r, i) => {
              const meta = ICON[r.category] ?? ICON.other;
              const sc = share(r.cost, totalCost), sk = share(r.carbon_kg_co2e, totalCarbon);
              const heavy = sk > 4 && sk > sc * 1.6; // emits much more than it costs
              const trend = halfTrend(r.trend), good = trend !== null && trend <= 0;
              return (
                <li key={r.service} className={cn("rise group grid items-center gap-x-4 gap-y-2 rounded-xl border-t border-line/70 px-2 py-2.5 transition-colors first:border-t-0 hover:bg-slate-50/80 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1.4fr)_96px_96px_132px]",
                  focus === r.service && "bg-sky-50 ring-2 ring-inset ring-sky-200")} style={{ "--i": i } as CSSProperties}>
                  <span className="flex min-w-0 items-center gap-3">
                    <span className={cn("flex size-9 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br text-white shadow-sm transition-transform duration-300 group-hover:scale-110", meta.tile)}><meta.icon className="size-[18px]" strokeWidth={1.8} /></span>
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-medium" title={r.service}>{r.service.replace(/^Amazon |^AWS /, "")}</span>
                      {heavy && <span className="mt-0.5 inline-block rounded-md bg-orange-50 px-1.5 py-px text-[10px] font-semibold text-orange-700 ring-1 ring-orange-200" title={t("ov2.svc.heavy_tip")}>{t("ov2.svc.heavy")}</span>}
                    </span>
                  </span>
                  <span className="space-y-1" aria-label={t("ov2.svc.shares")}>
                    <span className="flex items-center gap-2"><span className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-blue-400" style={{ width: `${Math.max(2, sc)}%`, "--d": `${i * 50}ms` } as CSSProperties} /></span><span className="w-10 text-right text-[11px] tabular-nums text-muted">{sc.toFixed(sc < 10 ? 1 : 0)}%</span></span>
                    <span className="flex items-center gap-2"><span className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-emerald-400" style={{ width: `${Math.max(2, sk)}%`, "--d": `${i * 50 + 60}ms` } as CSSProperties} /></span><span className="w-10 text-right text-[11px] tabular-nums text-muted">{sk.toFixed(sk < 10 ? 1 : 0)}%</span></span>
                  </span>
                  <span className="whitespace-nowrap text-sm tabular-nums lg:text-right">{formatCost(r.cost, cur)}</span>
                  <span className="whitespace-nowrap text-sm tabular-nums lg:text-right">{formatCarbon(r.carbon_kg_co2e)}</span>
                  <span className="flex items-center gap-2">
                    {trend !== null && (
                      <span title={t("ov2.svc.trend_tip")} className={cn("inline-flex w-14 shrink-0 items-center text-xs font-semibold", good ? "text-emerald-600" : "text-bad")}>
                        {trend <= 0 ? <ArrowDown className="size-3" strokeWidth={2.5} /> : <ArrowUp className="size-3" strokeWidth={2.5} />}{formatPct(trend)}
                      </span>
                    )}
                    <Sparkline values={r.trend} width={56} delay={i * 50 + 200} />
                  </span>
                </li>
              );
            })}
          </ul>
          <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
        </div>
      )}
    </Card>
  );
}
