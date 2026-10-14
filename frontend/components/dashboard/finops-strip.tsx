"use client";

import Link from "next/link";
import { useMemo } from "react";
import { STALE_AFTER } from "@/components/dashboard/cloud-costs";
import { Tile } from "@/components/dashboard/tile";
import { useAnomalies, useBudgets, useCarbonSummary, useCloudScope, useFinopsSummary, useForecast, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { formatCost, nf } from "@/lib/formatters";
import { monthForecast, rollup } from "@/lib/overview";
import { cn } from "@/lib/utils";

const dayLabel = (iso: string) => new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" });

/** The FinOps side of the same data: where the month will end, how the budget is burning, what is off, and what a currency unit emits. */
export function FinopsStrip() {
  const t = useT();
  const trend = useTrend();
  const cost = useFinopsSummary();
  const budgets = useBudgets();
  const anomalies = useAnomalies();
  const carbon = useCarbonSummary();
  const scope = useCloudScope();
  const cur = cost.data?.currency || "USD";
  const sf = useForecast().data;
  const fcur = sf?.currency || cur; // the forecast says which currency it is in: never printed with the summary's symbol
  const linear = useMemo(() => monthForecast(trend.data ?? []), [trend.data]);
  const fc = sf ? { forecast: sf.forecast.expected, spent: sf.forecast.spent, daysLeft: sf.forecast.days_left } : linear;
  const r = useMemo(() => rollup(trend.data ?? []), [trend.data]);
  // the budget of the cloud on view (measured on that cloud alone, in its currency); a budget of all clouds only when there is one cloud
  const mine = (budgets.data ?? []).filter((b) => (b.cloud ?? null) === scope);
  const orgBudget = mine.find((b) => b.project_id === null && b.period === "monthly") ?? mine[0];
  const staleCloud = (sf?.forecast.by_provider ?? []).find((p) => p.stale_days > STALE_AFTER);
  const loading = trend.isLoading || cost.isLoading;
  const spikes = anomalies.data ?? [];
  const worst = [...spikes].sort((a, b) => b.cost - b.expected - (a.cost - a.expected))[0];
  const seasonal = !!sf && sf.forecast.method === "seasonal";
  const multi = (cost.data?.by_provider.length ?? 0) > 1;

  return (
    <div className="grid grid-cols-1 gap-3 min-[560px]:grid-cols-2 min-[1000px]:grid-cols-4">
      <Tile index={4} icon="calendar" tone="sky" label={t("ov2.fin.forecast")} info={t("info.fin_forecast")} loading={loading}
        value={fc ? formatCost(fc.forecast, fcur) : null} empty={<span>{t("ov2.fin.forecast_none")}</span>}
        foot={fc && <>
          {seasonal && sf.forecast.high - sf.forecast.low > 1 && <span className="block font-medium text-[#3d8b62]" title={t("ov2.fin.range_hint")}>{t("ov2.fin.range", { low: formatCost(sf.forecast.low, fcur), high: formatCost(sf.forecast.high, fcur) })}</span>}
          <span className="block text-muted">{seasonal ? t("ov2.fin.forecast_seasonal", { spent: formatCost(fc.spent, fcur), days: fc.daysLeft }) : t("ov2.fin.forecast_basis", { spent: formatCost(fc.spent, fcur), days: fc.daysLeft })}</span>
          {staleCloud && <span className="block text-amber-700">{t("ov3.fin.stale_note", { cloud: cloudMeta(staleCloud.provider).name, day: dayLabel(staleCloud.last_day) })}</span>}
        </>} />
      <Tile index={5} icon="pie" tone="green" label={t("ov2.fin.budget")} info={t("info.fin_budget")} loading={loading || budgets.isLoading}
        value={orgBudget ? `${Math.round(orgBudget.used_pct)}%` : null}
        delta={orgBudget ? <span className="text-[11px] font-medium text-muted">{formatCost(orgBudget.spent, orgBudget.currency)} / {formatCost(orgBudget.amount, orgBudget.currency)}</span> : undefined}
        empty={<span>{scope ? t("ov3.budget.none_cloud", { cloud: cloudMeta(scope).name }) : t("ov2.fin.no_budget")} <Link href="/projects" className="font-semibold text-primary hover:underline">{t("ov2.fin.set_budget")}</Link></span>}
        foot={orgBudget && <>
          <span className="mb-1 block h-1.5 overflow-hidden rounded-full bg-slate-100">
            <span className={cn("bar-x block h-full rounded-full", orgBudget.state === "exceeded" ? "bg-pastel-rose" : orgBudget.state === "at_risk" ? "bg-pastel-amber" : "bg-leaf")} style={{ width: `${Math.min(100, Math.max(2, orgBudget.used_pct))}%` }} />
          </span>
          <span className="block">{orgBudget.projected > orgBudget.amount ? t("ov2.fin.will_exceed", { projected: formatCost(orgBudget.projected, orgBudget.currency) }) : orgBudget.projected_high > orgBudget.amount ? t("ov2.fin.may_exceed", { high: formatCost(orgBudget.projected_high, orgBudget.currency) }) : t("ov2.fin.on_track")}</span>
        </>} />
      <Tile index={6} icon="pulse" tone={spikes.length ? "amber" : "slate"} label={t("ov2.fin.anomalies")} info={t("info.fin_anomalies")} loading={loading || anomalies.isLoading}
        value={spikes.length || (trend.data?.length ?? 0) >= 14 ? String(spikes.length) : null}
        empty={<span>{t("ov2.fin.anomalies_short")}</span>}
        foot={spikes.length ? <>
          {worst && <span className="block">{worst.provider && multi ? `${cloudMeta(worst.provider).name} · ` : ""}{t("ov2.fin.worst", { day: new Date(worst.day).toLocaleDateString(undefined, { month: "short", day: "numeric" }), over: formatCost(worst.cost - worst.expected, cur) })}</span>}
          {worst?.drivers[0] && <span className="block truncate font-medium text-[#8a6420]" title={worst.drivers.map((d) => `${d.service} · ${d.region}: +${formatCost(d.delta, cur)}`).join("\n")}>{t(worst.drivers[0].new ? "ov2.fin.driver_new" : "ov2.fin.driver", { service: worst.drivers[0].service, region: worst.drivers[0].region, amount: formatCost(worst.drivers[0].delta, cur) })}</span>}
        </> : <span>{t("ov2.fin.anomalies_none")}</span>} />
      <Tile index={7} icon="scale" tone="violet" label={t("ov2.fin.per_currency")} info={t("info.fin_per_currency")} loading={loading || carbon.isLoading}
        value={r.gramsPerCurrency !== null && carbon.data?.has_data ? nf(r.gramsPerCurrency < 10 ? 1 : 0).format(r.gramsPerCurrency) : null} unit={`g CO₂e / ${cur}`}
        empty={<span>{t("ov2.fin.no_data")}</span>} foot={t("ov2.fin.per_currency_hint")} />
    </div>
  );
}
