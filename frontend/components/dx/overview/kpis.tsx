"use client";

import Link from "next/link";
import { useMemo, type CSSProperties, type ReactNode } from "react";
import { STALE_AFTER, useCloudRows } from "@/components/dashboard/cloud-costs";
import { CountUp } from "@/components/ui/count-up";
import { useAnomalies, useBudgets, useCarbonSummary, useCloudScope, useConnections, useFinopsSummary, useForecast, useOptimization, useRegions, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { carbonParts, formatCost, nf, pctChange } from "@/lib/formatters";
import { rollup } from "@/lib/overview";
import { regionName } from "@/lib/regions";
import { Ico, type DxIconName } from "../icons";
import { Delta, Hint, Skel, Spark, Tag } from "../ui";

function Tile({ index, icon, tone, color, label, hint, loading, value, unit, delta, spark, sparkColor, foot, empty }: {
  index: number; icon: DxIconName; tone: string; color: string; label: string; hint?: ReactNode; loading: boolean; value: ReactNode | null; unit?: string; delta?: ReactNode;
  spark?: number[]; sparkColor?: string; foot?: ReactNode; empty?: ReactNode;
}) {
  return (
    <div className="dx-kpi dx-rise" style={{ "--i": index, "--k": color } as CSSProperties}>
      <div className="dx-kpi-top">
        <span className={`dx-ico is-${tone}`} style={{ width: 24, height: 24 }}><Ico name={icon} size={16} /></span>
        <span>{label}</span>
        {hint && <Hint>{hint}</Hint>}
      </div>
      {loading ? <Skel h={34} w={150} /> : value === null ? <div className="dx-kpi-num" style={{ color: "var(--dx-line-strong)" }}>—</div> : (
        <div className="dx-kpi-row">
          <div className="dx-kpi-num">
            <span>{value}{unit && <small> {unit}</small>}</span>{delta}
          </div>
          {spark && spark.length > 1 && <div className="dx-kpi-spark"><Spark values={spark} color={sparkColor} /></div>}
        </div>
      )}
      <div className="dx-kpi-foot">{loading ? <Skel h={14} w={180} /> : value === null ? empty : foot}</div>
    </div>
  );
}

const dayLabel = (iso: string) => new Date(iso).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" });

/** Four figures the page is about, and four more behind a switch: money, emissions, the grid, what can be saved. */
export function Kpis({ more }: { more: boolean }) {
  const t = useT();
  const carbon = useCarbonSummary();
  const cost = useFinopsSummary();
  const trend = useTrend();
  const conns = useConnections();
  const opt = useOptimization();
  const regions = useRegions();
  const clouds = useCloudRows().rows;
  const budgets = useBudgets();
  const anomalies = useAnomalies();
  const forecast = useForecast();
  const scope = useCloudScope();
  const c = carbon.data, f = cost.data;
  const roll = useMemo(() => rollup(trend.data ?? []), [trend.data]);
  const costSeries = (trend.data ?? []).map((p) => p.cost);
  const carbonSeries = (trend.data ?? []).map((p) => p.carbon_kg_co2e);
  const parts = c?.has_data ? carbonParts(c.carbon_kg_co2e) : null;
  const loading = carbon.isLoading || cost.isLoading || conns.isLoading;
  const cur = f?.currency || "USD";
  const noCloud = (conns.data?.length ?? 0) === 0;
  const empty = noCloud ? <Link href="/connectors" style={{ color: "var(--dx-sky-ink)", fontWeight: 600 }}>{t("ov2.empty.connect")}</Link> : <span>{t("ov2.empty.waiting")}</span>;
  const intensity = c?.has_data && c.carbon_intensity_g_per_kwh > 0 ? c.carbon_intensity_g_per_kwh : null;
  const band = intensity === null ? null : intensity < 150 ? { k: "ov2.band.clean", tone: "sage" as const } : intensity < 300 ? { k: "ov2.band.moderate", tone: "butter" as const } : intensity < 450 ? { k: "ov2.band.high", tone: "peach" as const } : { k: "ov2.band.very_high", tone: "rose" as const };
  const cleanest = regions.data?.[0];
  const stale = clouds.find((r) => r.behind > STALE_AFTER);
  const costDelta = roll.costTrend7 ?? (f?.previous_comparable ? pctChange(f.total_cost, f.previous_total_cost) : null);
  const carbonDelta = roll.carbonTrend7 ?? (c?.comparable ? pctChange(c.carbon_kg_co2e, c.previous.carbon_kg_co2e) : null);
  const open = opt.data ? opt.data.open_count + opt.data.approved_count : 0;

  // the second row
  const sf = forecast.data;
  const mine = (budgets.data ?? []).filter((b) => (b.cloud ?? null) === scope);
  const budget = mine.find((b) => b.project_id === null && b.period === "monthly") ?? mine[0];
  const spikes = anomalies.data ?? [];
  const worst = [...spikes].sort((a, b) => b.cost - b.expected - (a.cost - a.expected))[0];
  const fcur = sf?.currency || cur;

  return (
    <div>
      <div className="dx-kpis">
        <Tile index={0} icon="cost" tone="sky" color="var(--dx-cost)" label={t("ov2.kpi.cost")} hint={t("info.kpi_cost")} loading={loading} spark={costSeries} sparkColor="var(--dx-cost)" empty={empty}
          value={f?.has_data ? <CountUp value={f.total_cost} format={(n) => formatCost(n, cur)} /> : null} delta={<Delta pct={costDelta} />}
          foot={<><b>{formatCost(roll.costPerDay, cur)}</b> {t("ov2.per_day")}{stale && <span style={{ color: "var(--dx-warn)" }}> · {t("ov3.kpi.stale", { cloud: cloudMeta(stale.provider).name, day: stale.lastDay ? dayLabel(stale.lastDay) : "—" })}</span>}</>} />
        <Tile index={1} icon="emissions" tone="sage" color="var(--dx-carbon)" label={t("ov.kpi.operational")} hint={t("ov.kpi.operational_hint")} loading={loading} spark={carbonSeries} sparkColor="var(--dx-carbon)" empty={empty}
          value={parts ? <CountUp value={parts.n} format={(n) => nf(1).format(n)} /> : null} unit={parts ? `${parts.unit} CO₂e` : undefined} delta={<Delta pct={carbonDelta} />}
          foot={<><b>{nf(roll.carbonPerDay < 10 ? 2 : 1).format(roll.carbonPerDay)} kg</b> {t("ov2.per_day")} · {t("ov2.operational_only")}</>} />
        <Tile index={2} icon="intensity" tone="butter" color="var(--dx-warn)" label={t("ov2.kpi.intensity")} hint={t("info.kpi_intensity")} loading={loading} empty={empty}
          value={intensity !== null ? <CountUp value={intensity} format={(n) => nf(0).format(n)} /> : null} unit="gCO₂e/kWh"
          delta={band ? <Tag tone={band.tone}>{t(band.k)}</Tag> : null}
          foot={intensity !== null && (
            <>
              <div className="dx-scale" aria-hidden>
                {Array.from({ length: 12 }).map((_, i) => <i key={i} className={i < Math.round((Math.min(intensity, 600) / 600) * 12) ? "is-on" : ""} style={{ "--c": i < 4 ? "var(--dx-good)" : i < 8 ? "var(--dx-warn)" : "var(--dx-bad)" } as CSSProperties} />)}
              </div>
              <span>{cleanest ? t("ov2.cleanest", { region: regionName(cleanest.region), g: nf(0).format(cleanest.g_per_kwh) }) : t("ov2.energy_weighted")}</span>
            </>
          )} />
        <Tile index={3} icon="potential" tone="peach" color="var(--dx-peach-ink)" label={t("ov.kpi.potential")} hint={t("info.optimization")} loading={loading || opt.isLoading}
          value={opt.data && open > 0 ? <CountUp value={opt.data.potential_carbon_kg_month} format={(n) => `−${nf(1).format(n)}`} /> : null} unit="kg CO₂e/mo"
          empty={<span>{t("ov2.opt.none")}</span>} delta={open > 0 ? <Tag tone="peach">{t("ov.kpi.open", { count: open })}</Tag> : null}
          foot={opt.data?.potential_saving_month ? <>{t("ov.kpi.potential_cost", { amount: formatCost(opt.data.potential_saving_month, cur) })} · <Link href="/recommendations" style={{ color: "var(--dx-sky-ink)", fontWeight: 600 }}>{t("dx.review", undefined, "Review")}</Link></> : t("ov.kpi.potential_nocost")} />
      </div>

      <div className={`dx-fold ${more ? "is-open" : ""}`} aria-hidden={!more}>
        <div>
          <div className="dx-kpis" style={{ paddingTop: 16 }}>
            <Tile index={0} icon="forecast" tone="sky" color="var(--dx-navy-3)" label={t("ov2.fin.forecast")} hint={t("info.fin_forecast")} loading={trend.isLoading || forecast.isLoading}
              value={sf ? formatCost(sf.forecast.expected, fcur) : null} empty={<span>{t("ov2.fin.forecast_none")}</span>}
              foot={sf && <>{sf.forecast.method === "seasonal" && sf.forecast.high - sf.forecast.low > 1 ? t("ov2.fin.range", { low: formatCost(sf.forecast.low, fcur), high: formatCost(sf.forecast.high, fcur) }) : t("ov2.fin.forecast_basis", { spent: formatCost(sf.forecast.spent, fcur), days: sf.forecast.days_left })}</>} />
            <Tile index={1} icon="budget" tone="sage" color="var(--dx-good)" label={t("ov2.fin.budget")} hint={t("info.fin_budget")} loading={budgets.isLoading}
              value={budget ? `${Math.round(budget.used_pct)}%` : null} empty={<span>{t("ov2.fin.no_budget")} <Link href="/projects" style={{ color: "var(--dx-sky-ink)", fontWeight: 600 }}>{t("ov2.fin.set_budget")}</Link></span>}
              delta={budget ? <Tag tone={budget.state === "exceeded" ? "rose" : budget.state === "at_risk" ? "butter" : "sage"}>{budget.state}</Tag> : null}
              foot={budget && <>{formatCost(budget.spent, budget.currency)} / {formatCost(budget.amount, budget.currency)}</>} />
            <Tile index={2} icon="anomaly" tone={spikes.length ? "butter" : "sand"} color="var(--dx-warn)" label={t("ov2.fin.anomalies")} hint={t("info.fin_anomalies")} loading={anomalies.isLoading}
              value={spikes.length || (trend.data?.length ?? 0) >= 14 ? String(spikes.length) : null} empty={<span>{t("ov2.fin.anomalies_short")}</span>}
              foot={worst ? t("ov2.fin.worst", { day: dayLabel(worst.day), over: formatCost(worst.cost - worst.expected, cur) }) : t("ov2.fin.anomalies_none")} />
            <Tile index={3} icon="scale" tone="rose" color="var(--dx-rose-ink)" label={t("ov2.fin.per_currency")} hint={t("info.fin_per_currency")} loading={trend.isLoading || carbon.isLoading}
              value={roll.gramsPerCurrency !== null && c?.has_data ? nf(roll.gramsPerCurrency < 10 ? 1 : 0).format(roll.gramsPerCurrency) : null} unit={`g CO₂e / ${cur}`}
              empty={<span>{t("ov2.fin.no_data")}</span>} foot={t("ov2.fin.per_currency_hint")} />
          </div>
        </div>
      </div>
    </div>
  );
}
