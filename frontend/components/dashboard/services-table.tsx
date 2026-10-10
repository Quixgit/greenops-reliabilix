"use client";

import { ArrowDown, ArrowUp, Boxes, Database, HardDrive, Network, Server, Table2, type LucideIcon } from "lucide-react";
import { useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { Sparkline } from "@/components/charts/sparkline";
import { content } from "@/content/schema";
import { useServices } from "@/hooks/use-overview";
import { formatCarbon, formatCost, formatPct, halfTrend } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const TABS = [
  { value: "all", label: "All" }, { value: "compute", label: "Compute" }, { value: "storage", label: "Storage" },
  { value: "database", label: "Database" }, { value: "networking", label: "Networking" }, { value: "other", label: "Other" },
];
const ICON: Record<string, { icon: LucideIcon; tile: string }> = {
  compute: { icon: Server, tile: "from-orange-300 to-orange-500" },
  storage: { icon: HardDrive, tile: "from-teal-300 to-teal-500" },
  database: { icon: Database, tile: "from-indigo-300 to-violet-500" },
  networking: { icon: Network, tile: "from-sky-300 to-blue-500" },
  other: { icon: Boxes, tile: "from-amber-300 to-orange-400" },
};

export function ServicesTable() {
  const [tab, setTab] = useState("all");
  const q = useServices(tab);
  const max = Math.max(1, ...(q.data ?? []).map((r) => r.cost));
  return (
    <Card index={6} interactive={false}>
      <CardHeader title="Details by Service" />
      <div className="mb-3"><PillTabs value={tab} onChange={setTab} tabs={TABS} /></div>
      {q.isLoading ? <Skeleton className="h-48" /> : q.isError ? <ErrorState /> :
        q.data!.length === 0 ? <EmptyState icon={Table2} text={content.empty.noUsage} /> : (
          <div key={tab} className="-mx-1 overflow-x-auto px-1">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs text-muted">
                  <th className="pb-2 font-medium">Service</th><th className="hidden w-28 pb-2 xl:table-cell" aria-label="Share of cost" />
                  <th className="w-20 pb-2 text-right font-medium">Cost</th><th className="w-20 pb-2 text-right font-medium">CO₂e</th>
                  <th className="w-36 pb-2 pl-4 font-medium">Trend</th>
                </tr>
              </thead>
              <tbody>
                {q.data!.map((r, i) => {
                  const meta = ICON[r.category] ?? ICON.other;
                  const trend = halfTrend(r.trend);
                  const good = trend !== null && trend <= 0;
                  return (
                    <tr key={r.service} className="rise group border-t border-line/70 transition-colors hover:bg-slate-50/70" style={{ "--i": i + 1 } as CSSProperties}>
                      <td className="py-2.5">
                        <span className="flex min-w-0 items-center gap-3">
                          <span className={cn("flex size-9 items-center justify-center rounded-xl bg-gradient-to-br text-white shadow-sm transition-transform duration-300 group-hover:scale-110", meta.tile)}><meta.icon className="size-[18px]" strokeWidth={1.8} /></span>
                          <span className="min-w-0 max-w-[15rem] truncate font-medium" title={r.service}>{r.service.replace(/^Amazon |^AWS /, "")}</span>
                        </span>
                      </td>
                      <td className="hidden w-28 px-2 xl:table-cell">
                        <div className="h-2 overflow-hidden rounded-full bg-slate-100" role="presentation">
                          <div className="grow-x h-full rounded-full bg-gradient-to-r from-blue-500 to-blue-400" style={{ width: `${Math.max(3, (r.cost / max) * 100)}%`, "--d": `${i * 70 + 150}ms` } as CSSProperties} />
                        </div>
                      </td>
                      <td className="whitespace-nowrap py-2.5 text-right tabular-nums">{formatCost(r.cost)}</td>
                      <td className="whitespace-nowrap py-2.5 text-right tabular-nums">{formatCarbon(r.carbon_kg_co2e)}</td>
                      <td className="py-2.5 pl-4">
                        <span className="flex items-center gap-2">
                          {trend !== null && (
                            <span title="Second half of the period compared with the first half" className={cn("inline-flex w-12 shrink-0 items-center text-xs font-semibold", good ? "text-emerald-600" : "text-bad")}>
                              {trend <= 0 ? <ArrowDown className="size-3" strokeWidth={2.5} /> : <ArrowUp className="size-3" strokeWidth={2.5} />}{formatPct(trend)}
                            </span>
                          )}
                          <Sparkline values={r.trend} width={64} delay={i * 70 + 200} />
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
    </Card>
  );
}
