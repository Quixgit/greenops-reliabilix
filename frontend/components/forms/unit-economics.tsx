"use client";

import Link from "next/link";
import { useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconGauge } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useUnitEconomics, type UnitProject } from "@/hooks/use-unit-economics";
import { useT } from "@/i18n/provider";
import { formatCost, nf } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const money = (v: number, cur: string) => (v < 1 ? new Intl.NumberFormat(undefined, { style: "currency", currency: cur, maximumSignificantDigits: 2, minimumFractionDigits: 0 }).format(v) : formatCost(v, cur));

function Change({ pct }: { pct: number | null }) {
  const t = useT();
  if (pct === null) return <span className="text-xs text-muted">{t("ue.no_change")}</span>;
  const down = pct <= 0;
  return <span className={cn("rounded-full px-2 py-0.5 text-xs font-semibold tabular-nums", down ? "bg-leaf-soft text-leaf-deep" : "bg-[#fdecef] text-[#c25667]")}>{down ? "↓" : "↑"} {Math.abs(pct).toFixed(1)}%</span>;
}

/** One bar per month; the bars grow from the baseline, the height is the cost per unit, and a short month is drawn faint. */
function Bars({ p, cur, pick }: { p: UnitProject; cur: string; pick: "cost" | "carbon" }) {
  const t = useT();
  const val = (m: UnitProject["months"][number]) => (pick === "cost" ? m.cost_per_unit : m.carbon_g_per_unit);
  const max = Math.max(...p.months.map((m) => val(m) ?? 0), 1e-12);
  return (
    <div className="flex h-24 items-end gap-2" role="img" aria-label={t(pick === "cost" ? "ue.cost_per" : "ue.carbon_per", { unit: p.unit })}>
      {p.months.map((m, i) => {
        const v = val(m);
        return (
          <div key={m.month} className="group relative flex h-full min-w-0 flex-1 flex-col items-center justify-end gap-1" title={`${m.month}: ${v === null ? "–" : pick === "cost" ? money(v, cur) : `${nf(v < 10 ? 2 : 0).format(v)} g`} · ${t("ue.days", { days: m.days })}`}>
            <div className={cn("bar-y w-full max-w-9 rounded-t-md", m.days < 7 ? "bg-leaf/30" : pick === "cost" ? "bg-[#7aa8e6]" : "bg-leaf")} style={{ height: `${v === null ? 0 : Math.max(4, (v / max) * 100)}%`, "--d": `${i * 60}ms` } as CSSProperties} />
            <span className="text-[10px] text-slate-400">{m.month.slice(5)}</span>
          </div>
        );
      })}
    </div>
  );
}

/** Cost and CO2e per unit of work, month by month: the number a CFO and a sustainability lead can both quote. */
export function UnitEconomicsPanel() {
  const t = useT();
  const q = useUnitEconomics(6);
  const [pick, setPick] = useState<"cost" | "carbon">("cost");
  const d = q.data;
  return (
    <div className="space-y-4">
      <Card index={0} interactive={false}>
        <CardHeader icon={<IconGauge className="size-[18px] text-violet-500" />} title={t("ue.title")} info={t("ue.info")}
          action={<div className="flex gap-1 rounded-lg bg-slate-100 p-0.5" role="tablist">{(["cost", "carbon"] as const).map((k) => <button key={k} type="button" role="tab" aria-selected={pick === k} onClick={() => setPick(k)} className={cn("rounded-md px-3 py-1 text-xs font-semibold transition-all", pick === k ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{t(`ue.pick.${k}`)}</button>)}</div>} />
        <p className="max-w-3xl text-sm text-muted">{t("ue.lead")}</p>
      </Card>
      {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : !d || d.projects.length === 0 ? (
        <Card interactive={false}><EmptyState icon={IconGauge} text={t("ue.empty")} /><p className="-mt-2 mb-3 text-center"><Link href="/projects?units=1" className="text-sm font-semibold text-primary hover:underline">{t("ov2.eff.report")} →</Link></p></Card>
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          {d.projects.map((p, i) => {
            const last = [...p.months].reverse().find((m) => m.cost_per_unit !== null);
            return (
              <Card key={p.project_id} index={i + 1} interactive={false} className="space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="min-w-0"><h3 className="truncate text-[15px] font-semibold text-ink">{p.name}</h3><p className="text-xs text-muted">{t("ue.per", { unit: p.unit.replace(/_/g, " ") })}</p></div>
                  <Change pct={pick === "cost" ? p.cost_change_pct : p.carbon_change_pct} />
                </div>
                {last && <p className="text-2xl font-bold tracking-tight tabular-nums">{pick === "cost" ? money(last.cost_per_unit!, d.currency) : <>{nf(last.carbon_g_per_unit! < 10 ? 2 : 0).format(last.carbon_g_per_unit!)} <span className="text-sm font-medium text-muted">g CO₂e</span></>}</p>}
                <Bars p={p} cur={d.currency} pick={pick} />
                <p className="text-[11px] leading-snug text-muted">{t("ue.basis", { month: last?.month ?? "", units: nf(0).format(last?.units ?? 0), days: last?.days ?? 0 })}</p>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
