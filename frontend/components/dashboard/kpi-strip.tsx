"use client";

import Link from "next/link";
import { useMemo, type CSSProperties } from "react";
import { AllowRegions } from "./allow-regions";
import type { ReactNode } from "react";
import { IconLeaf as Leaf } from "@/components/ui/icons";
import { MiniArea } from "@/components/charts/mini-area";
import { Info } from "@/components/ui/info";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { CountUp } from "@/components/ui/count-up";
import { DeltaBadge } from "@/components/ui/delta-badge";
import { STALE_AFTER, useCloudRows } from "@/components/dashboard/cloud-costs";
import { Tile } from "@/components/dashboard/tile";
import { useCarbonSummary, useConnections, useFinopsSummary, useOptimization, useRegions, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { carbonParts, formatCost, nf, pctChange } from "@/lib/formatters";
import { rollup } from "@/lib/overview";
import { regionName } from "@/lib/regions";

/** The tall KPI card the Carbon page still uses. */
export function Kpi({ index, icon: Icon, tone, label, info, value, unit, delta, spark, sparkColor, foot, loading, empty }: {
  index: number; icon: typeof Leaf; tone: string; label: string; info?: string; value: ReactNode | null; unit?: string; delta?: ReactNode;
  spark?: number[]; sparkColor?: string; foot?: ReactNode; loading: boolean; empty?: ReactNode;
}) {
  return (
    <Card index={index} className="group flex flex-col overflow-hidden p-0">
      <div className="flex items-start gap-3 p-4 pb-2 pr-8">{info && <Info corner>{info}</Info>}
        <Icon className={cn("size-12 shrink-0 transition-transform duration-500 group-hover:scale-110 group-hover:-rotate-6", tone)} strokeWidth={1.3} />
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-1 text-[13px] font-medium text-slate-600">{label}</p>
          {loading ? <Skeleton className="mt-1.5 h-8 w-28" /> : value === null ? (
            <p className="mt-0.5 text-2xl font-bold text-slate-300" aria-label="no data">—</p>
          ) : (
            <p className="mt-0.5 flex flex-wrap items-baseline gap-x-2 text-[26px] font-bold leading-tight tracking-tight text-ink">
              <span>{value}{unit && <span className="ml-1 text-sm font-medium text-slate-500">{unit}</span>}</span>
              {delta}
            </p>
          )}
        </div>
      </div>
      <div className="h-12 px-4">{!loading && value !== null && spark && <MiniArea values={spark} stroke={sparkColor} />}</div>
      <div className="mt-auto flex min-h-[3.75rem] flex-col justify-center border-t border-line/70 bg-slate-50/60 px-4 py-2 text-xs leading-snug text-slate-600">
        {loading ? <Skeleton className="h-4 w-40" /> : value === null ? (empty ?? "") : foot}
      </div>
    </Card>
  );
}

/** The four numbers the page is about: what the cloud costs, what it emits, how dirty the grid under it is, what can be saved. */
export function KpiStrip() {
  const t = useT();
  const carbon = useCarbonSummary();
  const cost = useFinopsSummary();
  const trend = useTrend();
  const conns = useConnections();
  const opt = useOptimization();
  const regions = useRegions();
  const clouds = useCloudRows().rows;
  const c = carbon.data, f = cost.data;
  const roll = useMemo(() => rollup(trend.data ?? []), [trend.data]);
  const costSeries = (trend.data ?? []).map((p) => p.cost);
  const carbonSeries = (trend.data ?? []).map((p) => p.carbon_kg_co2e);
  const hasCarbon = !!c?.has_data;
  const parts = hasCarbon ? carbonParts(c!.carbon_kg_co2e) : null;
  const noCloud = (conns.data?.length ?? 0) === 0;
  const empty = noCloud ? <Link href="/connectors" className="font-semibold text-primary hover:underline">{t("ov2.empty.connect")}</Link> : <span>{t("ov2.empty.waiting")}</span>;
  const loading = carbon.isLoading || cost.isLoading || conns.isLoading;
  const cur = f?.currency || "USD";
  const cleanest = regions.data?.[0];
  const foreign = clouds.filter((r) => r.foreign).map((r) => `${cloudMeta(r.provider).name} (${r.native.map((n) => n.currency).join("+")})`);
  const stale = clouds.find((r) => r.behind > STALE_AFTER);

  const trendBadge = (series: number | null, prev: number | null, fallback: number | null) =>
    series !== null ? <DeltaBadge pct={series} /> : prev !== null && fallback !== null ? <DeltaBadge pct={fallback} /> : null;
  const intensity = hasCarbon && c!.carbon_intensity_g_per_kwh > 0 ? c!.carbon_intensity_g_per_kwh : null;
  const band = intensity === null ? "" : intensity < 150 ? "ov2.band.clean" : intensity < 300 ? "ov2.band.moderate" : intensity < 450 ? "ov2.band.high" : "ov2.band.very_high";

  return (
    <div className="grid grid-cols-1 gap-3 min-[560px]:grid-cols-2 min-[1000px]:grid-cols-4">
      <Tile index={0} icon="wallet" tone="blue" label={t("ov2.kpi.cost")} info={t("info.kpi_cost")} loading={loading} spark={costSeries} sparkColor="#7aa7ea" empty={empty}
        value={f?.has_data ? <CountUp value={f.total_cost} format={(n) => formatCost(n, cur)} /> : null}
        delta={trendBadge(roll.costTrend7, f?.previous_comparable ? 1 : null, f?.previous_comparable ? pctChange(f.total_cost, f.previous_total_cost) : null)}
        foot={<>
          <span><b className="font-semibold text-slate-800">{formatCost(roll.costPerDay, cur)}</b> {t("ov2.per_day")}{roll.costTrend7 !== null && <span className="text-muted"> · {t("ov2.week_vs_week")}</span>}</span>
          {foreign.length > 0 && <span className="block truncate text-muted" title={t("ov2.kpi.converted", { cur })}>{t("ov3.kpi.converted_from", { clouds: foreign.join(", ") })}</span>}
          {stale && <span className="block text-amber-700">{t("ov3.kpi.stale", { cloud: cloudMeta(stale.provider).name, day: stale.lastDay ? new Date(stale.lastDay).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" }) : "—" })}</span>}
          {f && f.excluded.length > 0 && <span className="block text-amber-700">{t("ov.kpi.excluded", { currencies: f.excluded.map((e) => e.currency).join(", "), rows: f.excluded.reduce((n, e) => n + e.rows, 0) })}</span>}
        </>} />
      <Tile index={1} icon="leaf" tone="green" label={t("ov.kpi.operational")} info={t("ov.kpi.operational_hint")} loading={loading} spark={carbonSeries} sparkColor="#6fc3a8" empty={empty}
        value={parts ? <CountUp value={parts.n} format={(n) => nf(1).format(n)} /> : null} unit={parts ? `${parts.unit} CO₂e` : undefined}
        delta={trendBadge(roll.carbonTrend7, c?.comparable ? 1 : null, c?.comparable ? pctChange(c.carbon_kg_co2e, c.previous.carbon_kg_co2e) : null)}
        foot={<><span><b className="font-semibold text-slate-800">{nf(roll.carbonPerDay < 10 ? 2 : 1).format(roll.carbonPerDay)} kg</b> {t("ov2.per_day")}</span><span className="block text-muted">{t("ov2.operational_only")}</span></>} />
      <Tile index={2} icon="bolt" tone="amber" label={t("ov2.kpi.intensity")} info={t("info.kpi_intensity")} loading={loading} empty={empty}
        value={intensity !== null ? <CountUp value={intensity} format={(n) => nf(0).format(n)} /> : null} unit="gCO₂e/kWh"
        delta={intensity !== null ? <span className="rounded-md bg-amber-50 px-1.5 py-px text-[11px] font-semibold text-amber-700">{t(band)}</span> : null}
        foot={<>
          {intensity !== null && (
            <span className="mb-1 block h-1.5 overflow-hidden rounded-full bg-gradient-to-r from-leaf via-pastel-amber to-pastel-rose">
              <span className="block h-full border-r-2 border-slate-800/70" style={{ width: `${Math.min(100, (intensity / 600) * 100)}%` } as CSSProperties} />
            </span>
          )}
          <span className="block truncate">{cleanest ? t("ov2.cleanest", { region: regionName(cleanest.region), g: nf(0).format(cleanest.g_per_kwh) }) : t("ov2.energy_weighted")}</span>
        </>} />
      <Tile index={3} icon="gauge" tone="violet" label={t("ov.kpi.potential")} info={t("info.optimization")} loading={loading}
        value={opt.data && opt.data.open_count + opt.data.approved_count > 0 ? <CountUp value={opt.data.potential_carbon_kg_month} format={(n) => `−${nf(1).format(n)}`} /> : null} unit="kg CO₂e/mo"
        empty={<span>{t("ov2.opt.none")} <AllowRegions className="text-[12px]" /></span>}
        delta={opt.data ? <span className="text-[11px] font-semibold text-muted">{t("ov.kpi.open", { count: opt.data.open_count + opt.data.approved_count })}</span> : null}
        foot={opt.data?.potential_saving_month ? t("ov.kpi.potential_cost", { amount: formatCost(opt.data.potential_saving_month, cur) }) : t("ov.kpi.potential_nocost")} />
    </div>
  );
}
