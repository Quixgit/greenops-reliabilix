"use client";

import { useMemo, useRef, useState } from "react";
import { Card } from "@/components/ui/card";
import { CheckChip, Segmented, Switch } from "@/components/ui/controls";
import { EmptyState } from "@/components/ui/empty-state";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { MonthlyHistoryBody } from "@/components/dashboard/monthly-history";
import { useAnomalies, useFinopsSummary, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { cloudMeta } from "@/lib/clouds";
import { formatCost, nf } from "@/lib/formatters";
import { movingAverage } from "@/lib/overview";
import { cn } from "@/lib/utils";

const W = 640, H = 350, P = { l: 50, r: 44, t: 10, b: 24 };

/** A rounded-up axis maximum with four even steps (1, 2, 2.5, 5 times a power of ten). */
function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v))), n = v / p;
  return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 5 ? 5 : 10) * p;
}

type Metric = "both" | "cost" | "carbon";

/**
 * The page's main chart. Daily: cost (area, stacked by cloud when there are several and their layers can be switched off one by one) and CO₂e
 * (line) on two axes, with a crosshair and the cost spikes marked. Monthly: the last twelve months, one cloud at a time in its own currency.
 */
export function TrendPanel() {
  const t = useT();
  const q = useTrend();
  const cost = useFinopsSummary();
  const anomalies = useAnomalies();
  const cur = cost.data?.currency || "USD";
  const [view, setView] = useState<"daily" | "monthly">("daily");
  const [metric, setMetric] = useState<Metric>("both");
  const [smooth, setSmooth] = useState(false);
  const [off, setOff] = useState<Set<string>>(new Set());
  const [hover, setHover] = useState<number | null>(null);
  const box = useRef<HTMLDivElement>(null);

  const pts = useMemo(() => q.data ?? [], [q.data]);
  const allLayers = useMemo(() => {
    const totals = new Map<string, number>();
    for (const p of pts) for (const [k, v] of Object.entries(p.cost_by_provider)) totals.set(k, (totals.get(k) ?? 0) + Math.abs(v));
    return [...totals.entries()].filter(([, v]) => v > 0).sort((a, b) => b[1] - a[1]).map(([k]) => k);
  }, [pts]);
  const stacked = allLayers.length > 1;
  const layers = useMemo(() => {
    const prep = (v: number[]) => (smooth ? movingAverage(v) : v);
    return stacked ? allLayers.filter((id) => !off.has(id)).map((id) => ({ id, name: cloudMeta(id).name, color: cloudMeta(id).color, vals: prep(pts.map((p) => p.cost_by_provider[id] ?? 0)) })) : [];
  }, [pts, smooth, stacked, allLayers, off]);
  const costs = useMemo(() => (stacked ? pts.map((_, i) => layers.reduce((n, l) => n + l.vals[i], 0)) : smooth ? movingAverage(pts.map((p) => p.cost)) : pts.map((p) => p.cost)), [pts, smooth, stacked, layers]);
  const carbons = useMemo(() => (smooth ? movingAverage(pts.map((p) => p.carbon_kg_co2e)) : pts.map((p) => p.carbon_kg_co2e)), [pts, smooth]);
  const showCost = metric !== "carbon", showCarbon = metric !== "cost";
  const n = pts.length;
  const maxC = niceMax(Math.max(0, ...costs)), maxK = niceMax(Math.max(0, ...carbons));
  const x = (i: number) => P.l + (n <= 1 ? 0 : (i / (n - 1)) * (W - P.l - P.r));
  const yC = (v: number) => H - P.b - (v / maxC) * (H - P.t - P.b);
  const yK = (v: number) => H - P.b - (v / maxK) * (H - P.t - P.b);
  const line = (vals: number[], y: (v: number) => number) => vals.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const spikeDays = new Set((anomalies.data ?? []).map((a) => a.day.slice(0, 10)));
  const ticks = [0, 0.25, 0.5, 0.75, 1];
  const xLabels = n > 1 ? Array.from({ length: Math.min(6, n) }, (_, k) => Math.round((k / (Math.min(6, n) - 1)) * (n - 1))) : [0];

  const onMove = (e: React.MouseEvent) => {
    const r = box.current?.getBoundingClientRect();
    if (!r || n < 2) return;
    const px = ((e.clientX - r.left) / r.width) * W;
    setHover(Math.max(0, Math.min(n - 1, Math.round(((px - P.l) / (W - P.l - P.r)) * (n - 1)))));
  };
  const h = hover !== null ? pts[hover] : null;
  const fmtK = (v: number) => `${nf(maxK < 5 ? 1 : 0).format(v)} kg`;
  const toggleCloud = (id: string, on: boolean) => setOff((s) => { const c = new Set(s); if (on) c.delete(id); else if (allLayers.length - c.size > 1) c.add(id); return c; });

  return (
    <Card index={8} interactive={false} className="flex min-w-0 flex-col p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink">
          <Glyph name="trend" className="size-[18px] text-[#4a7fd0]" />{t("ov2.trend.title")}
        </h2>
        <Info corner>{t("info.trend")}</Info>
        <Segmented size="sm" label={t("ov2.trend.title")} value={view} onChange={setView}
          options={[{ value: "daily", label: t("ov3.trend.daily") }, { value: "monthly", label: t("ov3.months.title") }]} />
      </div>
      {view === "monthly" ? <MonthlyHistoryBody /> : q.isLoading ? <Skeleton className="h-[300px]" /> : n === 0 ? <EmptyState icon={() => <Glyph name="trend" className="size-6" />} text={t("ov2.trend.empty")} /> : (
        <>
          <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-2">
            <Segmented size="sm" label={t("ov3.trend.metric")} value={metric} onChange={setMetric}
              options={[{ value: "both", label: t("ov3.trend.both") }, { value: "cost", label: t("ov3.trend.cost") }, { value: "carbon", label: "CO₂e" }]} />
            <Switch checked={smooth} onChange={setSmooth} label={t("ov2.trend.avg7")} />
            {stacked && showCost && (
              <span className="flex flex-wrap items-center gap-1.5">
                {allLayers.map((id) => <CheckChip key={id} checked={!off.has(id)} onChange={(v) => toggleCloud(id, v)} color={cloudMeta(id).color}>{cloudMeta(id).name}</CheckChip>)}
              </span>
            )}
            {spikeDays.size > 0 && showCost && <span className="flex items-center gap-1.5 text-xs text-slate-600"><span className="size-2 rounded-full bg-pastel-amber ring-2 ring-amber-200" />{t("ov2.trend.spike")}</span>}
          </div>
          <div ref={box} className="relative" onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
            <svg viewBox={`0 0 ${W} ${H}`} className="block h-auto w-full" role="img" aria-label={t("ov2.trend.title")}>
              <defs>
                <linearGradient id="trend-cost" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#a9d1b9" stopOpacity=".38" /><stop offset="1" stopColor="#a9d1b9" stopOpacity="0.03" /></linearGradient>
                <linearGradient id="trend-carbon" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#5f8d6c" stopOpacity=".36" /><stop offset="1" stopColor="#5f8d6c" stopOpacity="0.02" /></linearGradient>
              </defs>
              {ticks.map((k) => (
                <g key={k}>
                  <line x1={P.l} x2={W - P.r} y1={yC(maxC * k)} y2={yC(maxC * k)} stroke="#e8f2ea" strokeDasharray={k === 0 ? "" : "3 4"} />
                  {showCost && <text x={P.l - 8} y={yC(maxC * k) + 4} textAnchor="end" className="fill-slate-400 text-[11px]">{formatCost(maxC * k, cur)}</text>}
                  {showCarbon && <text x={showCost ? W - P.r + 8 : P.l - 8} y={yK(maxK * k) + 4} textAnchor={showCost ? "start" : "end"} className="fill-leaf-deep/80 text-[11px]">{fmtK(maxK * k)}</text>}
                </g>
              ))}
              {xLabels.map((i) => (
                <text key={i} x={x(i)} y={H - 7} textAnchor={i === 0 ? "start" : i === n - 1 ? "end" : "middle"} className="fill-slate-400 text-[11px]">
                  {new Date(pts[i].day).toLocaleDateString(undefined, { month: "short", day: "numeric" })}
                </text>
              ))}
              {showCost && (stacked ? layers.map((l, k) => {
                const below = (i: number) => layers.slice(k + 1).reduce((a, m) => a + m.vals[i], 0); // the first layer is on top of the stack
                const top = costs.map((_, i) => below(i) + l.vals[i]), base = costs.map((_, i) => below(i));
                const up = top.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${yC(v).toFixed(1)}`).join(" ");
                const down = base.map((_, i) => `L${x(n - 1 - i).toFixed(1)},${yC(base[n - 1 - i]).toFixed(1)}`).join(" ");
                return <path key={l.id} d={`${up} ${down} Z`} fill={l.color} fillOpacity=".5" className="fade-in" />;
              }) : <path d={`${line(costs, yC)} L${x(n - 1)},${H - P.b} L${x(0)},${H - P.b} Z`} fill="url(#trend-cost)" className="fade-in" />)}
              {showCarbon && <path d={`${line(carbons, yK)} L${x(n - 1)},${H - P.b} L${x(0)},${H - P.b} Z`} fill="url(#trend-carbon)" className="fade-in" />}
              {showCost && <path d={line(costs, yC)} fill="none" stroke="#a9d1b9" strokeWidth="2" strokeLinejoin="round" pathLength={100} className="draw" style={{ ["--len" as string]: 100 }} />}
              {showCarbon && <path d={line(carbons, yK)} fill="none" stroke="#5f8d6c" strokeWidth="2" strokeLinejoin="round" pathLength={100} className="draw" style={{ ["--len" as string]: 100, ["--d" as string]: "150ms" }} />}
              {showCost && pts.map((p, i) => spikeDays.has(p.day.slice(0, 10)) && (
                <circle key={p.day} cx={x(i)} cy={yC(costs[i])} r="4" className="fill-pastel-amber stroke-amber-200" strokeWidth="3" />
              ))}
              {hover !== null && (
                <g>
                  <line x1={x(hover)} x2={x(hover)} y1={P.t} y2={H - P.b} stroke="#94aa9c" strokeDasharray="3 3" />
                  {showCost && <circle cx={x(hover)} cy={yC(costs[hover])} r="4.5" fill="#fff" stroke="#a9d1b9" strokeWidth="2" />}
                  {showCarbon && <circle cx={x(hover)} cy={yK(carbons[hover])} r="4.5" fill="#fff" stroke="#5f8d6c" strokeWidth="2" />}
                </g>
              )}
            </svg>
            {h && hover !== null && (
              <div className={cn("pointer-events-none absolute top-2 z-10 min-w-[160px] rounded-xl bg-white/95 px-3 py-2 text-xs shadow-lg ring-1 ring-line backdrop-blur", (hover / Math.max(1, n - 1)) > 0.6 ? "right-[calc(100%-var(--x))]" : "left-[var(--x)]")}
                style={{ ["--x" as string]: `${(x(hover) / W) * 100}%` }}>
                <p className="font-semibold text-ink">{new Date(h.day).toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" })}</p>
                {showCost && stacked && layers.map((l) => (
                  <p key={l.id} className="mt-0.5 flex justify-between gap-4"><span className="flex items-center gap-1.5"><span className="size-2 rounded-sm" style={{ background: l.color }} />{l.name}</span><span className="tabular-nums">{formatCost(l.vals[hover], cur)}</span></p>
                ))}
                {showCost && <p className="mt-1 flex justify-between gap-4"><span className="text-blue-600">{stacked ? t("ov3.trend.total") : t("ov2.trend.cost_short")}</span><span className="tabular-nums">{formatCost(stacked ? costs[hover] : h.cost, cur)}</span></p>}
                {showCarbon && <p className="flex justify-between gap-4"><span className="text-leaf-deep">CO₂e</span><span className="tabular-nums">{nf(h.carbon_kg_co2e < 10 ? 2 : 1).format(h.carbon_kg_co2e)} kg</span></p>}
                {spikeDays.has(h.day.slice(0, 10)) && <p className="mt-1 text-amber-700">{t("ov2.trend.spike_tip")}</p>}
              </div>
            )}
          </div>
        </>
      )}
    </Card>
  );
}
