"use client";

import { IconWarning } from "@/components/ui/icons";
import { useMemo, type CSSProperties } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import { settingsView } from "@/lib/api/schemas";
import { Card } from "@/components/ui/card";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { CountUp } from "@/components/ui/count-up";
import { Skeleton } from "@/components/ui/skeleton";
import { MiniArea } from "@/components/charts/mini-area";
import { CloudMark } from "@/components/dashboard/cloud-mark";
import { useCloudScope, useFinopsSummary, useForecast, useRegionFootprint, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCarbon, formatCost, formatNative, nf } from "@/lib/formatters";
import { cloudMeta } from "@/lib/clouds";
import { periodRange, useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

/** Whole days between the newest day a cloud has cost for and yesterday (the newest day a bill can be complete for). */
export function daysBehind(lastDay: string | null, now = new Date()): number {
  if (!lastDay) return 0;
  const today = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  const last = Date.UTC(+lastDay.slice(0, 4), +lastDay.slice(5, 7) - 1, +lastDay.slice(8, 10));
  return Math.max(0, Math.round((today - last) / 86_400_000) - 1);
}
/** A cloud bills with a day or two of delay: up to two days behind is normal, more is a gap the figures should say. */
export const STALE_AFTER = 2;

export interface CloudRow {
  provider: string; cost: number; share: number;
  native: { currency: string; amount: number }[]; foreign: boolean; rate: number | null;
  daysWithData: number; lastDay: string | null; behind: number;
  carbon: number; forecast: number | null; spark: number[];
}

/** One row per cloud from the cost summary, the carbon footprint, the month forecast and the daily series; the order is by cost. */
export function useCloudRows(): { rows: CloudRow[]; cur: string; windowDays: number; loading: boolean } {
  const summary = useFinopsSummary();
  const footprint = useRegionFootprint();
  const forecast = useForecast();
  const trend = useTrend();
  const { period, custom } = useUi();
  void custom;
  const { from, to } = periodRange(period);
  const windowDays = Math.round((Date.parse(to) - Date.parse(from)) / 86_400_000) + 1;
  const cur = summary.data?.currency || "USD";
  const rows = useMemo(() => {
    const by = summary.data?.by_provider ?? [];
    const total = by.reduce((n, p) => n + p.cost, 0);
    const carbon = new Map<string, number>();
    for (const r of footprint.data ?? []) carbon.set(r.provider, (carbon.get(r.provider) ?? 0) + r.carbon_kg_co2e);
    const fc = new Map((forecast.data?.forecast.by_provider ?? []).map((p) => [p.provider, p.expected]));
    return by.map((p): CloudRow => {
      const foreign = p.native.some((n) => n.currency !== cur);
      const one = p.native.length === 1 ? p.native[0] : null;
      return {
        provider: p.provider, cost: p.cost, share: total > 0 ? (p.cost / total) * 100 : 0, native: p.native, foreign,
        rate: foreign && one && one.amount > 0 ? p.cost / one.amount : null,
        daysWithData: p.days_with_data, lastDay: p.last_day, behind: daysBehind(p.last_day),
        carbon: carbon.get(p.provider) ?? 0, forecast: fc.get(p.provider) ?? null,
        spark: (trend.data ?? []).map((d) => d.cost_by_provider[p.provider] ?? 0),
      };
    });
  }, [summary.data, footprint.data, forecast.data, trend.data, cur]);
  return { rows, cur, windowDays, loading: summary.isLoading };
}

/** Switches the currency every total is shown in (the organization's reporting currency): one chip per currency the clouds are billed in. */
function CurrencySwitch({ options, current }: { options: string[]; current: string }) {
  const t = useT();
  const qc = useQueryClient();
  const set = useMutation({
    mutationFn: (c: string) => api("settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values: { "finops.reporting_currency": c } }) }),
    onSuccess: () => qc.invalidateQueries(),
  });
  return (
    <div role="radiogroup" aria-label={t("ov3.clouds.show_in")} className="flex items-center gap-1 rounded-lg bg-slate-100 p-0.5 text-xs">
      <span className="px-2 text-[11px] text-muted">{t("ov3.clouds.show_in")}</span>
      {options.map((c) => (
        <button key={c} type="button" role="radio" aria-checked={c === current} disabled={set.isPending} onClick={() => c !== current && set.mutate(c)}
          className={cn("rounded-md px-2.5 py-1 font-semibold transition-all duration-200", c === current ? "bg-white text-ink shadow-sm" : "text-muted hover:text-ink")}>{c}</button>
      ))}
    </div>
  );
}

/**
 * One card per cloud, each in the currency it is billed in, with what that comes to in the reporting currency beside it, the share of the
 * bill, the carbon, the month-end figure and how fresh its data is. Two clouds in two currencies are never one unlabelled sum, and a cloud whose
 * data stopped arriving says so instead of quietly lowering the total.
 */
export function CloudCosts() {
  const t = useT();
  const { rows, cur, windowDays, loading } = useCloudRows();
  const scope = useCloudScope();
  const setCloud = useUi((s) => s.setCloudFilter);
  const stale = rows.filter((r) => r.behind > STALE_AFTER);
  const mixed = rows.some((r) => r.foreign);
  const currencies = [...new Set([cur, ...rows.flatMap((r) => r.native.map((n) => n.currency))])];
  if (scope || (!loading && rows.length < 2 && stale.length === 0)) return null;
  return (
    <Card index={3} interactive={false} className="p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink">
          <Glyph name="cloud" className="size-[18px] text-[#3b93b8]" />{t("ov3.clouds.title")}
        </h2>
        {mixed && <CurrencySwitch options={currencies} current={cur} />}
      </div>
      {loading ? <Skeleton className="h-24" /> : (
        <>
          <div className="mb-3 flex h-2 overflow-hidden rounded-full bg-slate-100" role="img" aria-label={t("ov3.clouds.share_bar")}>
            {rows.map((r, i) => (
              <span key={r.provider} className="bar-x h-full first:rounded-l-full last:rounded-r-full" title={`${cloudMeta(r.provider).name} · ${nf(0).format(r.share)}%`}
                style={{ width: `${r.share}%`, background: cloudMeta(r.provider).color, "--d": `${i * 90}ms` } as CSSProperties} />
            ))}
          </div>
          <ul className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,270px),1fr))] gap-2.5">
            {rows.map((r, i) => {
              const m = cloudMeta(r.provider);
              const gap = r.behind > STALE_AFTER;
              const one = r.native.length === 1 ? r.native[0] : null;
              const day = r.lastDay ? new Date(r.lastDay).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" }) : "—";
              return (
                <li key={r.provider} className="rise" style={{ "--i": i } as CSSProperties}>
                  <button type="button" onClick={() => setCloud(r.provider)} title={t("ov3.clouds.filter_tip", { cloud: m.name })}
                    className="group relative grid h-full w-full grid-cols-[auto_minmax(0,1fr)_5.5rem] items-center gap-x-3 gap-y-1 rounded-2xl p-3 text-left ring-1 ring-line transition-all duration-300 hover:-translate-y-0.5 hover:shadow-md hover:ring-slate-300"
                    style={{ background: `linear-gradient(150deg, ${m.soft}, #fff 65%)` }}>
                    <CloudMark provider={r.provider} className="row-span-2 size-10 transition-transform duration-300 group-hover:scale-110" />
                    <span className="flex min-w-0 items-baseline gap-2">
                      <span className="truncate text-sm font-semibold text-ink">{m.name}</span>
                      <span className="text-[11px] font-semibold tabular-nums" style={{ color: m.ink }}>{nf(0).format(r.share)}%</span>
                    </span>
                    <span className="row-span-2 h-10 self-center">{r.spark.length > 1 && <MiniArea values={r.spark} stroke={m.color} height={40} className="h-10" />}</span>
                    <span className="min-w-0">
                      <span className="block text-xl font-bold leading-6 tracking-tight text-ink tabular-nums">
                        {one && r.foreign ? <CountUp value={one.amount} format={(n) => formatNative(n, one.currency)} />
                          : r.native.length > 1 ? r.native.map((n) => formatNative(n.amount, n.currency)).join(" + ")
                          : <CountUp value={r.cost} format={(n) => formatCost(n, cur)} />}
                      </span>
                      <span className="block truncate text-[11px] text-muted">
                        {r.foreign ? <><b className="font-semibold text-slate-700">≈ {formatCost(r.cost, cur)}</b>{r.rate !== null && one && ` · 1 ${one.currency} = ${nf(r.rate < 10 ? 3 : 2).format(r.rate)} ${cur}`}</> : t("ov3.clouds.billed_in", { cur })}
                      </span>
                    </span>
                    <span className="col-span-3 mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-slate-600">
                      <span className="h-1.5 min-w-[3rem] flex-1 overflow-hidden rounded-full bg-white/80 ring-1 ring-black/5">
                        <span className="bar-x block h-full rounded-full" style={{ width: `${Math.max(3, r.share)}%`, background: m.color, "--d": `${i * 90 + 120}ms` } as CSSProperties} />
                      </span>
                      {r.carbon > 0 && <span className="tabular-nums">{formatCarbon(r.carbon)} CO₂e</span>}
                      {r.forecast !== null && <span className="tabular-nums text-muted" title={t("ov3.clouds.forecast_tip")}>{t("ov3.clouds.month_end", { amount: formatCost(r.forecast, cur) })}</span>}
                      <span className={cn("inline-flex items-center gap-1 rounded-md px-1.5 py-px font-medium", gap ? "bg-amber-50 text-amber-800 ring-1 ring-amber-200" : "text-muted")}
                        title={gap ? t("ov3.clouds.stale", { day, days: r.behind }) : undefined}>
                        {gap ? <IconWarning className="size-3 text-amber-600" /> : <span className="size-1.5 rounded-full bg-leaf" />}
                        {gap ? t("ov3.clouds.stale_short", { days: r.behind }) : t("ov3.clouds.fresh", { day })}
                        {r.daysWithData > 0 && r.daysWithData < windowDays - STALE_AFTER && <span className="text-muted">· {t("ov3.clouds.coverage", { days: r.daysWithData, window: windowDays })}</span>}
                      </span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        </>
      )}
    </Card>
  );
}
