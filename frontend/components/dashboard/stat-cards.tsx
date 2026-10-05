"use client";

import { Gauge, Leaf, Wallet, Zap, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { DeltaBadge } from "@/components/ui/delta-badge";
import { useCarbonSummary, useConnections, useFinopsSummary } from "@/hooks/use-overview";
import { content } from "@/content/schema";
import { formatCarbon, formatCost, nf, pctChange } from "@/lib/formatters";
import { PERIODS, useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

function StatCard({ icon: Icon, tone, label, value, unit, delta, loading, emptyText, compare }: {
  icon: LucideIcon; tone: "green" | "blue"; label: string; value: ReactNode | null; unit?: string;
  delta: ReactNode; loading: boolean; emptyText: string; compare: string;
}) {
  return (
    <Card className="p-4">
      <div className="flex items-center gap-3">
        <span className={cn("flex size-10 shrink-0 items-center justify-center rounded-xl", tone === "green" ? "bg-carbon-soft text-carbon" : "bg-cost-soft text-primary")}>
          <Icon className="size-[18px]" />
        </span>
        <p className="text-sm font-medium text-muted">{label}</p>
      </div>
      {loading ? <Skeleton className="mt-3 h-8 w-28" /> : value === null ? (
        <>
          <p className="mt-3 text-3xl font-bold tracking-tight text-slate-300" aria-label="no data">—</p>
          <p className="mt-1 text-xs text-muted">{emptyText}</p>
        </>
      ) : (
        <>
          <p className="mt-3 flex items-baseline gap-1.5 whitespace-nowrap text-3xl font-bold tracking-tight">{value}{unit && <span className="text-xs font-medium text-muted">{unit}</span>}</p>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted">{delta}<span>{compare}</span></div>
        </>
      )}
    </Card>
  );
}

export function StatCards() {
  const carbon = useCarbonSummary();
  const cost = useFinopsSummary();
  const conns = useConnections();
  const period = useUi((s) => s.period);
  const days = PERIODS.find((p) => p.key === period)!.days;

  // Why is there nothing? No cloud connected vs. connected but not synced yet.
  const emptyText = (conns.data?.length ?? 0) === 0 ? content.empty.noCloud : content.empty.awaitingSync;
  const compare = `vs previous ${days} days`;
  const c = carbon.data, f = cost.data;
  const hasCarbon = !!c?.has_data;

  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard icon={Leaf} tone="green" label="Total CO₂e" loading={carbon.isLoading || conns.isLoading}
        value={hasCarbon ? formatCarbon(c!.carbon_kg_co2e) : null} emptyText={emptyText} compare={compare}
        delta={c && <DeltaBadge pct={pctChange(c.carbon_kg_co2e, c.previous.carbon_kg_co2e)} />} />
      <StatCard icon={Wallet} tone="blue" label="Total Cloud Cost" loading={cost.isLoading || conns.isLoading}
        value={f?.has_data ? formatCost(f.total_cost, f.currency || "USD") : null} emptyText={emptyText} compare={compare}
        delta={f && <DeltaBadge pct={pctChange(f.total_cost, f.previous_total_cost)} />} />
      <StatCard icon={Zap} tone="green" label="Carbon Intensity" loading={carbon.isLoading || conns.isLoading}
        value={hasCarbon && c!.carbon_intensity_g_per_kwh > 0 ? nf(0).format(c!.carbon_intensity_g_per_kwh) : null} unit="gCO₂e/kWh" emptyText={emptyText} compare={compare}
        delta={c && <DeltaBadge pct={pctChange(c.carbon_intensity_g_per_kwh, c.previous.carbon_intensity_g_per_kwh)} />} />
      {/* SCI = carbon per functional unit: lower is better, so a falling score is the green direction. */}
      <StatCard icon={Gauge} tone="blue" label="SCI Score" loading={carbon.isLoading || conns.isLoading}
        value={hasCarbon && c!.sci_score > 0 ? nf(1).format(c!.sci_score) : null} unit="gCO₂e/unit" emptyText={emptyText} compare={compare}
        delta={c && <DeltaBadge pct={pctChange(c.sci_score, c.previous.sci_score)} better="lower" />} />
    </div>
  );
}
