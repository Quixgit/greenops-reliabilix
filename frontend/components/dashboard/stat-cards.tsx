"use client";

import type { ReactNode } from "react";
import { GaugeIcon, LeafIcon, StackIcon } from "@/components/ui/brand-icons";
import { Card } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { DeltaBadge } from "@/components/ui/delta-badge";
import { Skeleton } from "@/components/ui/skeleton";
import { useCarbonSummary, useConnections, useFinopsSummary } from "@/hooks/use-overview";
import { content } from "@/content/schema";
import type { MethodologyInfo } from "@/lib/api/schemas";
import { carbonParts, nf, pctChange } from "@/lib/formatters";
import { PERIODS, useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

const TONE = {
  green: "bg-gradient-to-br from-emerald-50 to-emerald-100 ring-emerald-100",
  blue: "bg-gradient-to-br from-sky-50 to-blue-100 ring-blue-100",
  purple: "bg-gradient-to-br from-violet-50 to-violet-100 ring-violet-100",
} as const;

function StatCard({ index, icon: Icon, tone, label, value, unit, delta, loading, emptyText, compare, note }: {
  index: number; icon: (p: { className?: string }) => ReactNode; tone: keyof typeof TONE; label: string; value: ReactNode | null; unit?: string;
  delta: ReactNode; loading: boolean; emptyText: string; compare: string;
  /** A qualification that must stay next to the number (for example what the figure leaves out). */
  note?: string;
}) {
  return (
    <Card index={index} className="group p-4">
      <div className="flex items-center gap-3">
        <span className={cn("flex size-11 shrink-0 items-center justify-center rounded-full ring-1 transition-transform duration-500 ease-[var(--ease-out-soft)] group-hover:scale-110 group-hover:rotate-6", TONE[tone])}>
          <Icon className="size-[26px]" />
        </span>
        <div className="min-w-0">
          <p className="text-sm font-medium text-slate-700">{label}</p>
          {loading ? <Skeleton className="mt-1.5 h-8 w-28" /> : value === null ? (
            <p className="text-3xl font-bold tracking-tight text-slate-300" aria-label="no data">—</p>
          ) : (
            <p className="flex flex-wrap items-baseline gap-x-2 text-2xl font-bold leading-tight tracking-tight text-ink">
              <span>{value}{unit && <span className="ml-1 text-sm font-medium text-slate-600">{unit}</span>}</span>
              {delta}
            </p>
          )}
        </div>
      </div>
      {!loading && (value === null
        ? <p className="mt-3 text-center text-xs text-muted">{emptyText}</p>
        : <p className="mt-3 text-center text-xs text-muted">{delta ? compare : "Not enough history to compare yet"}</p>)}
      {!loading && value !== null && note && <p className="mt-1 text-center text-xs text-amber-700">{note}</p>}
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
  const compare = `vs. previous ${days} days`;
  const c = carbon.data, f = cost.data;
  const hasCarbon = !!c?.has_data;
  // Never show a score that looks complete when it is not: the API says what the methodology leaves out.
  const embodiedNote = c && !c.methodology.embodied_carbon_included ? "Operational emissions only: embodied carbon is not included" : undefined;
  const parts = hasCarbon ? carbonParts(c!.carbon_kg_co2e) : null;
  const money = (n: number) => new Intl.NumberFormat("en-US", { style: "currency", currency: f?.currency || "USD", maximumFractionDigits: 0 }).format(n);

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[minmax(0,1fr)] gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard index={0} icon={LeafIcon} tone="green" label="Total CO₂e" loading={carbon.isLoading || conns.isLoading}
          value={parts ? <CountUp value={parts.n} format={(n) => nf(1).format(n)} /> : null} unit={parts?.unit} emptyText={emptyText} compare={compare} note={embodiedNote}
          delta={c?.comparable ? <DeltaBadge pct={pctChange(c.carbon_kg_co2e, c.previous.carbon_kg_co2e)} /> : null} />
        <StatCard index={1} icon={StackIcon} tone="blue" label="Total Cloud Cost" loading={cost.isLoading || conns.isLoading}
          value={f?.has_data ? <CountUp value={f.total_cost} format={money} /> : null} emptyText={emptyText} compare={compare}
          delta={f?.previous_comparable ? <DeltaBadge pct={pctChange(f.total_cost, f.previous_total_cost)} /> : null} />
        <StatCard index={2} icon={LeafIcon} tone="green" label="Carbon Intensity" loading={carbon.isLoading || conns.isLoading}
          value={hasCarbon && c!.carbon_intensity_g_per_kwh > 0 ? <CountUp value={c!.carbon_intensity_g_per_kwh} format={(n) => nf(0).format(n)} /> : null} unit="gCO₂e/kWh" emptyText={emptyText} compare={compare}
          delta={c?.comparable ? <DeltaBadge pct={pctChange(c.carbon_intensity_g_per_kwh, c.previous.carbon_intensity_g_per_kwh)} /> : null} />
        {/* SCI = carbon per functional unit: lower is better, so a falling score is the green direction. */}
        <StatCard index={3} icon={GaugeIcon} tone="purple" label="SCI Score" loading={carbon.isLoading || conns.isLoading}
          value={hasCarbon && c!.sci_score !== null ? <CountUp value={c!.sci_score} format={(n) => nf(1).format(n)} /> : null} unit="gCO₂e/unit"
          emptyText={hasCarbon ? "Report functional units to see SCI" : emptyText} compare={compare} note={embodiedNote ? "Embodied carbon is not included yet: this SCI is operational only" : undefined}
          delta={c?.comparable && c.sci_score !== null && c.previous.sci_score !== null ? <DeltaBadge pct={pctChange(c.sci_score, c.previous.sci_score)} better="lower" /> : null} />
      </div>
      {hasCarbon && c!.methodology.status === "provisional" && <MethodologyNotice m={c!.methodology} />}
    </div>
  );
}

/** What the figures rest on, in the API's own words: provisional methodology and what it leaves out. */
function MethodologyNotice({ m }: { m: MethodologyInfo }) {
  return (
    <details className="rise rounded-xl bg-amber-50 px-4 py-2.5 text-xs text-amber-900 ring-1 ring-amber-200" style={{ "--i": 4 } as React.CSSProperties}>
      <summary className="cursor-pointer font-medium">Provisional estimates (methodology {m.version}) — read before using these figures externally</summary>
      <ul className="mt-2 list-disc space-y-1 pl-5">{m.caveats.map((t) => <li key={t}>{t}</li>)}</ul>
    </details>
  );
}
