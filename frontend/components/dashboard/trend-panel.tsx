"use client";

import { IconTrend as Activity } from "@/components/ui/icons";
import { useMemo, useRef, useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useAnomalies, useFinopsSummary, useTrend } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCost, nf } from "@/lib/formatters";
import { movingAverage } from "@/lib/overview";
import { cn } from "@/lib/utils";

const W = 600, H = 330, P = { l: 52, r: 46, t: 12, b: 26 };

/** A rounded-up axis maximum with four even steps (1, 2, 2.5, 5 times a power of ten). */
function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v))), n = v / p;
  return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 5 ? 5 : 10) * p;
}

/** Cost (area) and CO₂e (line) day by day on two axes, with a crosshair, spikes marked and an optional 7-day average. */
export function TrendPanel() {
  const t = useT();
  const q = useTrend();
  const cost = useFinopsSummary();
  const anomalies = useAnomalies();
  const cur = cost.data?.currency || "USD";
  const [smooth, setSmooth] = useState(false);
  const [hover, setHover] = useState<number | null>(null);
  const box = useRef<HTMLDivElement>(null);

  const pts = q.data ?? [];
  const costs = useMemo(() => (smooth ? movingAverage(pts.map((p) => p.cost)) : pts.map((p) => p.cost)), [pts, smooth]);
  const carbons = useMemo(() => (smooth ? movingAverage(pts.map((p) => p.carbon_kg_co2e)) : pts.map((p) => p.carbon_kg_co2e)), [pts, smooth]);
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

  return (
    <Card index={8} interactive={false} className="flex min-w-0 flex-col">
      <CardHeader icon={<Activity className="size-[18px] text-blue-500" />} title={t("ov2.trend.title")} info={t("info.trend")}
        action={
          <label className="flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-600">
            <input type="checkbox" className="peer sr-only" checked={smooth} onChange={(e) => setSmooth(e.target.checked)} />
            <span className="relative h-5 w-9 rounded-full bg-slate-200 transition-colors peer-checked:bg-teal-700/75 after:absolute after:left-0.5 after:top-0.5 after:size-4 after:rounded-full after:bg-white after:shadow after:transition-transform peer-checked:after:translate-x-4" />
            {t("ov2.trend.avg7")}
          </label>
        } />
      {q.isLoading ? <Skeleton className="h-[330px]" /> : n === 0 ? <EmptyState icon={Activity} text={t("ov2.trend.empty")} /> : (
        <>
          <div className="mb-2 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-slate-600">
            <span className="flex items-center gap-1.5"><span className="h-2.5 w-2.5 rounded-sm bg-blue-400" />{t("ov2.trend.cost", { cur })}</span>
            <span className="flex items-center gap-1.5"><span className="h-2.5 w-2.5 rounded-sm bg-emerald-400" />{t("ov2.trend.carbon")}</span>
            {spikeDays.size > 0 && <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-amber-500 ring-2 ring-amber-200" />{t("ov2.trend.spike")}</span>}
          </div>
          <div ref={box} className="relative" onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
            <svg viewBox={`0 0 ${W} ${H}`} className="block h-auto w-full" role="img" aria-label={t("ov2.trend.title")}>
              <defs>
                <linearGradient id="trend-cost" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#60a5fa" stopOpacity=".34" /><stop offset="1" stopColor="#60a5fa" stopOpacity="0.02" /></linearGradient>
                <linearGradient id="trend-carbon" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#34d399" stopOpacity=".34" /><stop offset="1" stopColor="#34d399" stopOpacity="0.02" /></linearGradient>
              </defs>
              {ticks.map((k) => (
                <g key={k}>
                  <line x1={P.l} x2={W - P.r} y1={yC(maxC * k)} y2={yC(maxC * k)} stroke="#e6ecf5" strokeDasharray={k === 0 ? "" : "3 4"} />
                  <text x={P.l - 8} y={yC(maxC * k) + 4} textAnchor="end" className="fill-slate-400 text-[11px]">{formatCost(maxC * k, cur)}</text>
                  <text x={W - P.r + 8} y={yK(maxK * k) + 4} className="fill-emerald-600/80 text-[11px]">{fmtK(maxK * k)}</text>
                </g>
              ))}
              {xLabels.map((i) => (
                <text key={i} x={x(i)} y={H - 8} textAnchor={i === 0 ? "start" : i === n - 1 ? "end" : "middle"} className="fill-slate-400 text-[11px]">
                  {new Date(pts[i].day).toLocaleDateString(undefined, { month: "short", day: "numeric" })}
                </text>
              ))}
              <path d={`${line(costs, yC)} L${x(n - 1)},${H - P.b} L${x(0)},${H - P.b} Z`} fill="url(#trend-cost)" className="fade-in" />
              <path d={`${line(carbons, yK)} L${x(n - 1)},${H - P.b} L${x(0)},${H - P.b} Z`} fill="url(#trend-carbon)" className="fade-in" />
              <path d={line(costs, yC)} fill="none" stroke="#3b82f6" strokeWidth="2" strokeLinejoin="round" pathLength={100} className="draw" style={{ ["--len" as string]: 100 }} />
              <path d={line(carbons, yK)} fill="none" stroke="#10b981" strokeWidth="2" strokeLinejoin="round" pathLength={100} className="draw" style={{ ["--len" as string]: 100, ["--d" as string]: "150ms" }} />
              {pts.map((p, i) => spikeDays.has(p.day.slice(0, 10)) && (
                <circle key={p.day} cx={x(i)} cy={yC(costs[i])} r="4" className="fill-amber-500 stroke-amber-200" strokeWidth="3" />
              ))}
              {hover !== null && (
                <g>
                  <line x1={x(hover)} x2={x(hover)} y1={P.t} y2={H - P.b} stroke="#94a3b8" strokeDasharray="3 3" />
                  <circle cx={x(hover)} cy={yC(costs[hover])} r="4.5" fill="#fff" stroke="#3b82f6" strokeWidth="2" />
                  <circle cx={x(hover)} cy={yK(carbons[hover])} r="4.5" fill="#fff" stroke="#10b981" strokeWidth="2" />
                </g>
              )}
            </svg>
            {h && hover !== null && (
              <div className={cn("pointer-events-none absolute top-2 z-10 min-w-[150px] rounded-xl bg-white/95 px-3 py-2 text-xs shadow-lg ring-1 ring-line backdrop-blur", (hover / Math.max(1, n - 1)) > 0.6 ? "right-[calc(100%-var(--x))]" : "left-[var(--x)]")}
                style={{ ["--x" as string]: `${(x(hover) / W) * 100}%` }}>
                <p className="font-semibold text-ink">{new Date(h.day).toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric" })}</p>
                <p className="mt-1 flex justify-between gap-4"><span className="text-blue-600">{t("ov2.trend.cost_short")}</span><span className="tabular-nums">{formatCost(h.cost, cur)}</span></p>
                <p className="flex justify-between gap-4"><span className="text-emerald-600">CO₂e</span><span className="tabular-nums">{nf(h.carbon_kg_co2e < 10 ? 2 : 1).format(h.carbon_kg_co2e)} kg</span></p>
                {spikeDays.has(h.day.slice(0, 10)) && <p className="mt-1 text-amber-700">{t("ov2.trend.spike_tip")}</p>}
              </div>
            )}
          </div>
        </>
      )}
    </Card>
  );
}
