"use client";

import { IconClock as CalendarClock, IconCoin as Scale, IconWallet as PiggyBank, IconWarning as AlertTriangle } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { Info } from "@/components/ui/info";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useAnomalies, useBudgets, useCarbonSummary, useFinopsSummary, useForecast, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCost, nf } from "@/lib/formatters";
import { monthForecast, rollup } from "@/lib/overview";
import { cn } from "@/lib/utils";

function Tile({ index, icon: Icon, tone, title, info, children, loading }: { index: number; icon: typeof Scale; tone: string; title: string; info?: string; children: ReactNode; loading: boolean }) {
  return (
    <Card index={index} className="p-4">
      <p className="flex items-center gap-2 text-[13px] font-medium text-slate-600">
        <Icon className={cn("size-7", tone)} strokeWidth={1.5} />{title}{info && <Info>{info}</Info>}
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
  const sf = useForecast().data;
  const linear = useMemo(() => monthForecast(trend.data ?? []), [trend.data]);
  const fc = sf ? { forecast: sf.forecast.expected, spent: sf.forecast.spent, daysLeft: sf.forecast.days_left } : linear;
  const r = useMemo(() => rollup(trend.data ?? []), [trend.data]);
  const orgBudget = (budgets.data ?? []).find((b) => b.project_id === null && b.period === "monthly") ?? (budgets.data ?? [])[0];
  const loading = trend.isLoading || cost.isLoading;
  const spikes = anomalies.data ?? [];
  const worst = [...spikes].sort((a, b) => b.cost - b.expected - (a.cost - a.expected))[0];

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Tile index={4} icon={CalendarClock} tone="text-[#5b8fdc]" title={t("ov2.fin.forecast")} info={t("info.fin_forecast")} loading={loading}>
        {fc ? (
          <>
            <p className="text-xl font-bold tabular-nums text-ink">{formatCost(fc.forecast, cur)}</p>
            {sf && sf.forecast.method === "seasonal" && sf.forecast.high - sf.forecast.low > 1 && <p className="text-xs font-medium text-[#3f78b8]" title={t("ov2.fin.range_hint")}>{t("ov2.fin.range", { low: formatCost(sf.forecast.low, cur), high: formatCost(sf.forecast.high, cur) })}</p>}
            <p className="mt-0.5 text-xs text-muted">{sf && sf.forecast.method === "seasonal" ? t("ov2.fin.forecast_seasonal", { spent: formatCost(fc.spent, cur), days: fc.daysLeft }) : t("ov2.fin.forecast_basis", { spent: formatCost(fc.spent, cur), days: fc.daysLeft })}{sf && sf.forecast.lag_days > 0 ? ` ${t("ov2.fin.lag", { count: sf.forecast.lag_days })}` : ""}</p>
          </>
        ) : <p className="text-sm text-muted">{t("ov2.fin.forecast_none")}</p>}
      </Tile>
      <Tile index={5} icon={PiggyBank} tone="text-leaf-deep" title={t("ov2.fin.budget")} info={t("info.fin_budget")} loading={loading || budgets.isLoading}>
        {orgBudget ? (
          <>
            <p className="flex items-baseline justify-between text-sm"><span className="text-xl font-bold tabular-nums text-ink">{Math.round(orgBudget.used_pct)}%</span>
              <span className="text-xs text-muted">{formatCost(orgBudget.spent, orgBudget.currency)} / {formatCost(orgBudget.amount, orgBudget.currency)}</span></p>
            <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-slate-100">
              <div className={cn("bar-x h-full rounded-full", orgBudget.state === "exceeded" ? "bg-pastel-rose" : orgBudget.state === "at_risk" ? "bg-pastel-amber" : "bg-leaf")}
                style={{ width: `${Math.min(100, Math.max(2, orgBudget.used_pct))}%` }} />
            </div>
            <p className="mt-1 text-xs text-muted">{orgBudget.projected > orgBudget.amount ? t("ov2.fin.will_exceed", { projected: formatCost(orgBudget.projected, orgBudget.currency) }) : orgBudget.projected_high > orgBudget.amount ? t("ov2.fin.may_exceed", { high: formatCost(orgBudget.projected_high, orgBudget.currency) }) : t("ov2.fin.on_track")}</p>
          </>
        ) : (
          <p className="text-sm text-muted">{t("ov2.fin.no_budget")} <Link href="/settings" className="font-semibold text-primary hover:underline">{t("ov2.fin.set_budget")}</Link></p>
        )}
      </Tile>
      <Tile index={6} icon={AlertTriangle} tone={spikes.length ? "text-[#d9a441]" : "text-slate-400"} title={t("ov2.fin.anomalies")} info={t("info.fin_anomalies")} loading={loading || anomalies.isLoading}>
        {spikes.length ? (
          <>
            <p className="text-xl font-bold tabular-nums text-ink">{spikes.length}</p>
            {worst && <p className="mt-0.5 text-xs text-muted">{t("ov2.fin.worst", { day: new Date(worst.day).toLocaleDateString(undefined, { month: "short", day: "numeric" }), over: formatCost(worst.cost - worst.expected, cur) })}</p>}
            {worst?.drivers[0] && <p className="mt-0.5 text-xs font-medium text-[#a9741a]" title={worst.drivers.map((d) => `${d.service} · ${d.region}: +${formatCost(d.delta, cur)}`).join("\n")}>{t(worst.drivers[0].new ? "ov2.fin.driver_new" : "ov2.fin.driver", { service: worst.drivers[0].service, region: worst.drivers[0].region, amount: formatCost(worst.drivers[0].delta, cur) })}</p>}
          </>
        ) : <p className="text-sm text-muted">{(trend.data?.length ?? 0) < 14 ? t("ov2.fin.anomalies_short") : t("ov2.fin.anomalies_none")}</p>}
      </Tile>
      <Tile index={7} icon={Scale} tone="text-[#8a78d8]" title={t("ov2.fin.per_currency")} info={t("info.fin_per_currency")} loading={loading || carbon.isLoading}>
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
