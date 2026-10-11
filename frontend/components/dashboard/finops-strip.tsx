"use client";

import { AlertTriangle, CalendarClock, PiggyBank, Scale } from "lucide-react";
import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useAnomalies, useBudgets, useCarbonSummary, useFinopsSummary, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCost, nf } from "@/lib/formatters";
import { monthForecast, rollup } from "@/lib/overview";
import { cn } from "@/lib/utils";

function Tile({ index, icon: Icon, tone, title, children, loading }: { index: number; icon: typeof Scale; tone: string; title: string; children: ReactNode; loading: boolean }) {
  return (
    <Card index={index} className="p-4">
      <p className="flex items-center gap-2 text-[13px] font-medium text-slate-600">
        <span className={cn("flex size-6 items-center justify-center rounded-lg", tone)}><Icon className="size-3.5" strokeWidth={2} /></span>{title}
      </p>
      <div className="mt-2.5">{loading ? <Skeleton className="h-12" /> : children}</div>
    </Card>
  );
}

/** The FinOps side of the same data: where the month will end, how the budget is burning, what is off, and what a currency unit emits. */
export function FinopsStrip() {
  const t = useT();
  const trend = useTrend();
  const cost = useFinopsSummary();
  const budgets = useBudgets();
  const anomalies = useAnomalies();
  const carbon = useCarbonSummary();
  const cur = cost.data?.currency || "USD";
  const fc = useMemo(() => monthForecast(trend.data ?? []), [trend.data]);
  const r = useMemo(() => rollup(trend.data ?? []), [trend.data]);
  const orgBudget = (budgets.data ?? []).find((b) => b.project_id === null && b.period === "monthly") ?? (budgets.data ?? [])[0];
  const loading = trend.isLoading || cost.isLoading;
  const spikes = anomalies.data ?? [];
  const worst = [...spikes].sort((a, b) => b.cost - b.expected - (a.cost - a.expected))[0];

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Tile index={4} icon={CalendarClock} tone="bg-sky-50 text-sky-600" title={t("ov2.fin.forecast")} loading={loading}>
        {fc ? (
          <>
            <p className="text-xl font-bold tabular-nums text-ink">{formatCost(fc.forecast, cur)}</p>
            <p className="mt-0.5 text-xs text-muted">{t("ov2.fin.forecast_basis", { spent: formatCost(fc.spent, cur), days: fc.daysLeft })}</p>
          </>
        ) : <p className="text-sm text-muted">{t("ov2.fin.forecast_none")}</p>}
      </Tile>
      <Tile index={5} icon={PiggyBank} tone="bg-emerald-50 text-emerald-600" title={t("ov2.fin.budget")} loading={loading || budgets.isLoading}>
        {orgBudget ? (
          <>
            <p className="flex items-baseline justify-between text-sm"><span className="text-xl font-bold tabular-nums text-ink">{Math.round(orgBudget.used_pct)}%</span>
              <span className="text-xs text-muted">{formatCost(orgBudget.spent, orgBudget.currency)} / {formatCost(orgBudget.amount, orgBudget.currency)}</span></p>
            <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-slate-100">
              <div className={cn("bar-x h-full rounded-full", orgBudget.state === "exceeded" ? "bg-red-400" : orgBudget.state === "at_risk" ? "bg-amber-400" : "bg-emerald-400")}
                style={{ width: `${Math.min(100, Math.max(2, orgBudget.used_pct))}%` }} />
            </div>
            <p className="mt-1 text-xs text-muted">{orgBudget.projected > orgBudget.amount ? t("ov2.fin.will_exceed", { projected: formatCost(orgBudget.projected, orgBudget.currency) }) : t("ov2.fin.on_track")}</p>
          </>
        ) : (
          <p className="text-sm text-muted">{t("ov2.fin.no_budget")} <Link href="/settings" className="font-semibold text-primary hover:underline">{t("ov2.fin.set_budget")}</Link></p>
        )}
      </Tile>
      <Tile index={6} icon={AlertTriangle} tone={spikes.length ? "bg-amber-50 text-amber-600" : "bg-slate-100 text-slate-500"} title={t("ov2.fin.anomalies")} loading={loading || anomalies.isLoading}>
        {spikes.length ? (
          <>
            <p className="text-xl font-bold tabular-nums text-ink">{spikes.length}</p>
            {worst && <p className="mt-0.5 text-xs text-muted">{t("ov2.fin.worst", { day: new Date(worst.day).toLocaleDateString(undefined, { month: "short", day: "numeric" }), over: formatCost(worst.cost - worst.expected, cur) })}</p>}
          </>
        ) : <p className="text-sm text-muted">{(trend.data?.length ?? 0) < 14 ? t("ov2.fin.anomalies_short") : t("ov2.fin.anomalies_none")}</p>}
      </Tile>
      <Tile index={7} icon={Scale} tone="bg-violet-50 text-violet-600" title={t("ov2.fin.per_currency")} loading={loading || carbon.isLoading}>
        {r.gramsPerCurrency !== null && carbon.data?.has_data ? (
          <>
            <p className="text-xl font-bold tabular-nums text-ink">{nf(r.gramsPerCurrency < 10 ? 1 : 0).format(r.gramsPerCurrency)} <span className="text-xs font-medium text-muted">g CO₂e / {cur}</span></p>
            <p className="mt-0.5 text-xs text-muted">{t("ov2.fin.per_currency_hint")}</p>
          </>
        ) : <p className="text-sm text-muted">{t("ov2.fin.no_data")}</p>}
      </Tile>
    </div>
  );
}
