"use client";

import { IconChart } from "@/components/ui/icons";
import { useMemo, useState } from "react";
import { useQueries } from "@tanstack/react-query";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { CloudMark } from "@/components/dashboard/cloud-mark";
import { api } from "@/lib/api/client";
import * as s from "@/lib/api/schemas";
import { useCloudList, useCloudScope, useFinopsSummary } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { formatCarbon, formatCost, getFormatLocale, nf, pctChange } from "@/lib/formatters";
import { useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

const MONTHS = 12;

interface Month { key: string; cost: number; carbon: number; days: number; of: number; partial: boolean }

/** Daily points into calendar months. A month is partial when fewer of its days have data than it has (the first month of a connection, the one in progress). */
function toMonths(points: { day: string; cost: number; carbon_kg_co2e: number }[]): Month[] {
  const m = new Map<string, Month>();
  for (const p of points) {
    const k = p.day.slice(0, 7);
    const x = m.get(k) ?? { key: k, cost: 0, carbon: 0, days: 0, of: new Date(Date.UTC(+k.slice(0, 4), +k.slice(5, 7), 0)).getUTCDate(), partial: false };
    x.cost += p.cost; x.carbon += p.carbon_kg_co2e; x.days += 1;
    m.set(k, x);
  }
  return [...m.values()].sort((a, b) => a.key.localeCompare(b.key)).map((x) => ({ ...x, partial: x.days < x.of }));
}

const monthName = (k: string, long = false) =>
  new Date(`${k}-01T12:00:00Z`).toLocaleDateString(getFormatLocale(), { month: long ? "long" : "short", ...(long ? { year: "numeric" } : {}), timeZone: "UTC" });

const HATCH = "[background-image:repeating-linear-gradient(135deg,transparent_0_5px,rgba(255,255,255,.6)_5px_8px)]";

/**
 * The last twelve months, one cloud at a time: a cloud's months are in the currency it is billed in and are never added to another's. With
 * one cloud in view it is a bar chart with CO₂e under it; with all clouds it is one row per cloud on its own scale. A month that is not
 * complete (in progress, or before the connection began) is hatched and is not compared with a full one.
 */
export function MonthlyHistoryBody() {
  const t = useT();
  const { projectId } = useUi();
  const scope = useCloudScope();
  const list = useCloudList().data ?? [];
  const cur = useFinopsSummary().data?.currency || "USD";
  const [hover, setHover] = useState<string | null>(null);
  const now = new Date();
  const from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - (MONTHS - 1), 1)).toISOString().slice(0, 10);
  const to = now.toISOString().slice(0, 10);
  const targets: (string | null)[] = scope ? [scope] : list.length > 1 ? list.map((c) => c.provider) : [null];
  const qs = useQueries({
    queries: targets.map((c) => ({
      queryKey: ["trend-months", projectId, from, to, c],
      queryFn: () => api("dashboard/trend", s.trend, { from, to, project_id: projectId ?? undefined, cloud: c ?? undefined }),
      staleTime: 5 * 60_000,
    })),
  });
  const loading = qs.some((q) => q.isLoading);
  const stamp = qs.map((q) => q.dataUpdatedAt).join(",");
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const series = useMemo(() => qs.map((q) => toMonths(q.data ?? [])), [stamp]);
  const empty = !loading && series.every((m) => m.length === 0);
  const rows = targets.length > 1;
  const currencyOf = (c: string | null) => (c ? list.find((x) => x.provider === c)?.native[0]?.currency : null) ?? cur;

  return (
    <div className="min-w-0">
      <p className="mb-2 text-xs text-muted">{rows ? t("ov3.months.own_currency") : currencyOf(targets[0])}</p>
      {loading ? <Skeleton className="h-52" /> : empty ? <EmptyState icon={IconChart} text={t("ov3.months.empty")} /> : rows ? (
        <ul className="space-y-3">
          {targets.map((c, i) => {
            const m = series[i], max = Math.max(1e-9, ...m.map((x) => x.cost)), cc = currencyOf(c), color = cloudMeta(c ?? "").color;
            const last = m[m.length - 1], prevFull = [...m].reverse().find((x, k) => k > 0 && !x.partial);
            const d = last && prevFull && !last.partial ? pctChange(last.cost, prevFull.cost) : null;
            return (
              <li key={c} className="grid grid-cols-[minmax(7.5rem,10rem)_minmax(0,1fr)] items-end gap-x-4 gap-y-1 sm:grid-cols-[11rem_minmax(0,1fr)_7.5rem]">
                <span className="flex items-center gap-2 self-center text-sm font-semibold text-ink"><CloudMark provider={c ?? ""} className="size-6" />{cloudMeta(c ?? "").name}<span className="rounded-md bg-slate-100 px-1.5 py-px text-[10px] font-bold text-slate-600">{cc}</span></span>
                <span className="flex h-14 items-end gap-1">
                  {m.map((x) => {
                    const id = `${c}|${x.key}`;
                    return (
                      <span key={x.key} className="group relative flex h-full min-w-0 flex-1 items-end" onMouseEnter={() => setHover(id)} onMouseLeave={() => setHover(null)}>
                        <span className={cn("block w-full rounded-t-md transition-all duration-300", x.partial && cn("opacity-60", HATCH))}
                          style={{ height: `${Math.max(4, (x.cost / max) * 100)}%`, background: color }} />
                        {hover === id && (
                          <span className="pointer-events-none absolute -top-1 left-1/2 z-10 -translate-x-1/2 -translate-y-full whitespace-nowrap rounded-lg bg-white px-2.5 py-1.5 text-[11px] shadow-lg ring-1 ring-line">
                            <b>{monthName(x.key, true)}</b> · {formatCost(x.cost, cc)} · {formatCarbon(x.carbon)} CO₂e{x.partial && <span className="text-muted"> · {t("ov3.months.days", { days: x.days, of: x.of })}</span>}
                          </span>
                        )}
                      </span>
                    );
                  })}
                </span>
                <span className="col-span-2 text-right sm:col-span-1 sm:self-center">
                  {last && <span className="block text-sm font-semibold tabular-nums text-ink">{formatCost(last.cost, cc)}</span>}
                  <span className="block text-[11px] text-muted">{d === null ? (last?.partial ? t("ov3.months.so_far", { days: last.days }) : "") : t("ov3.months.vs_prev", { pct: `${d > 0 ? "+" : "−"}${nf(1).format(Math.abs(d))}%` })}</span>
                </span>
              </li>
            );
          })}
          <li className="grid grid-cols-[minmax(7.5rem,10rem)_minmax(0,1fr)] gap-x-4 sm:grid-cols-[11rem_minmax(0,1fr)_7.5rem]">
            <span />
            <span className="flex gap-1">{series[0].map((x) => <span key={x.key} className="min-w-0 flex-1 text-center text-[10px] text-slate-400">{monthName(x.key)}</span>)}</span>
          </li>
        </ul>
      ) : (() => {
        const m = series[0], maxCost = Math.max(1e-9, ...m.map((x) => x.cost)), maxCo2 = Math.max(1e-9, ...m.map((x) => x.carbon));
        const color = cloudMeta(targets[0] ?? "").color, c0 = currencyOf(targets[0]);
        return (
          <div className="relative" onMouseLeave={() => setHover(null)}>
            <ul className="flex h-52 items-end justify-center gap-1.5 sm:gap-2.5" role="list">
              {m.map((x) => {
                const prev = m[m.indexOf(x) - 1], d = prev && !prev.partial && !x.partial ? pctChange(x.cost, prev.cost) : null;
                return (
                  <li key={x.key} className="group relative flex h-full min-w-0 max-w-[84px] flex-1 flex-col justify-end" onMouseEnter={() => setHover(x.key)} onFocus={() => setHover(x.key)} tabIndex={0}
                    aria-label={`${monthName(x.key, true)}: ${formatCost(x.cost, c0)}, ${formatCarbon(x.carbon)} CO₂e`}>
                    <span className="mb-1 text-center text-[10px] font-semibold tabular-nums text-slate-600">{formatCost(x.cost, c0)}</span>
                    <div className={cn("rounded-t-lg transition-all duration-300 group-hover:brightness-95", x.partial && cn("opacity-60", HATCH))}
                      style={{ height: `${Math.max(2, (x.cost / maxCost) * 70)}%`, background: color }} />
                    <div className="mt-1.5 h-1.5 rounded-full bg-leaf/80" style={{ width: `${Math.max(8, (x.carbon / maxCo2) * 100)}%` }} />
                    <span className={cn("mt-1.5 text-center text-[11px] leading-none", hover === x.key ? "font-semibold text-ink" : "text-slate-500")}>{monthName(x.key)}</span>
                    {hover === x.key && (
                      <span className="pointer-events-none absolute -top-2 left-1/2 z-10 w-44 -translate-x-1/2 rounded-xl bg-white/95 px-3 py-2 text-left text-xs shadow-lg ring-1 ring-line backdrop-blur">
                        <b className="block text-ink">{monthName(x.key, true)}</b>
                        <span className="mt-0.5 flex justify-between"><span>{t("ov2.trend.cost_short")}</span><span className="tabular-nums">{formatCost(x.cost, c0)}</span></span>
                        <span className="flex justify-between"><span className="text-leaf-deep">CO₂e</span><span className="tabular-nums">{formatCarbon(x.carbon)}</span></span>
                        {x.partial && <span className="mt-1 block text-muted">{t("ov3.months.days", { days: x.days, of: x.of })}</span>}
                        {d !== null && <span className="mt-1 block text-muted">{t("ov3.months.vs_prev", { pct: `${d > 0 ? "+" : "−"}${nf(1).format(Math.abs(d))}%` })}</span>}
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>
            <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-muted">
              <span className="flex items-center gap-1.5"><span className="h-2 w-4 rounded-sm" style={{ background: color }} />{t("ov2.trend.cost", { cur: c0 })}</span>
              <span className="flex items-center gap-1.5"><span className="h-1.5 w-4 rounded-full bg-leaf" />CO₂e</span>
              {m.some((x) => x.partial) && <span>{t("ov3.months.partial")}</span>}
            </p>
          </div>
        );
      })()}
      {rows && <p className="mt-3 text-[11px] text-muted">{t("ov3.months.partial")}</p>}
    </div>
  );
}
