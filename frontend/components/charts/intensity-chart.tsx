"use client";

import { useMemo } from "react";
import { tokens } from "@/design-system/tokens";
import type { IntensityPoint } from "@/lib/api/schemas";
import { Chart } from "./chart";

export const SOURCE_LABEL: Record<string, string> = { electricitymaps: "Electricity Maps", ember: "Ember", uk_carbon_intensity: "UK Carbon Intensity", eia: "EIA (computed from generation mix)", energy_charts: "Energy-Charts (computed from generation mix)", ieso: "IESO (Ontario, computed from generation mix)", hydroquebec: "Hydro-Québec (computed from generation mix)", entsoe: "ENTSO-E (Europe, computed from generation mix)", ons: "ONS (Brazil, computed from generation mix)" };
const OTHER_COLORS = ["#8b5cf6", "#f2cf8a", "#06b6d4"];

/**
 * Daily grid carbon intensity of one region. The line is the source the platform trusts for each day; the band is that
 * day's lowest and highest reading; other sources are dashed lines of their own and are never merged into it.
 * Days without a reading are gaps in the line.
 */
export function IntensityChart({ points, from, to }: { points: IntensityPoint[]; from: string; to: string }) {
  const option = useMemo(() => {
    const byDay = new Map(points.map((p) => [p.day.slice(0, 10), p]));
    const days: string[] = [];
    for (let t = new Date(from.slice(0, 10) + "T00:00:00Z").getTime(); t < new Date(to.slice(0, 10) + "T00:00:00Z").getTime(); t += 86_400_000) days.push(new Date(t).toISOString().slice(0, 10));
    // The window ends before `to`, but the newest day (today) has readings too and is the freshest data there is: every
    // day the API returned belongs on the axis, whatever the window says.
    for (const p of points) { const d = p.day.slice(0, 10); if (!days.includes(d)) days.push(d); }
    days.sort();
    // A young history (readings only since the source was connected) would be a few dots at the far right of an empty
    // month: start the axis shortly before the first reading instead, never narrower than a week.
    if (points.length > 0 && points.length < days.length / 2) {
      const first = points.map((p) => p.day.slice(0, 10)).sort()[0];
      const at = Math.max(0, Math.min(days.indexOf(first) - 1, days.length - 7));
      days.splice(0, at);
    }
    const get = (d: string, f: (p: IntensityPoint) => number | null) => { const p = byDay.get(d); return p ? f(p) : null; };
    const others = Array.from(new Set(points.flatMap((p) => p.others.map((o) => o.source))));
    const axis = { axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: tokens.color.muted, fontSize: 12 } };
    const lo = days.map((d) => get(d, (p) => p.min));
    return {
      grid: { left: 4, right: 8, top: 36, bottom: 4, containLabel: true },
      legend: { top: 0, right: 4, icon: "roundRect", itemWidth: 14, itemHeight: 3, textStyle: { color: "#475569", fontSize: 12 } },
      tooltip: {
        trigger: "axis", backgroundColor: "#fff", borderColor: "#e8edf5", borderWidth: 1, padding: [10, 14], extraCssText: "border-radius:12px;box-shadow:0 10px 30px rgba(31,52,110,.16);",
        textStyle: { color: "#0f1b33", fontSize: 12 },
        formatter: (raw: unknown) => {
          const day = (raw as { axisValue: string }[])[0]?.axisValue;
          const p = day ? byDay.get(day) : undefined;
          if (!p) return `<div style="color:#6b7a94;font-size:11px">${day}</div><div style="margin-top:4px;color:#6b7a94">No reading this day</div>`;
          const note = p.basis === "month_mean" ? ` <span style="color:#6366f1">monthly national average</span>`
            : p.basis === "partial_day_mean" ? ` <span style="color:#b45309">partial (${p.readings} readings)</span>` : ` <span style="color:#6b7a94">${p.readings} readings</span>`;
          const rows = [`<div style="margin-top:4px"><b>${Math.round(p.g_per_kwh)}</b> gCO₂e/kWh · ${SOURCE_LABEL[p.source] ?? p.source}${note}</div>`,
            ...(p.basis === "month_mean" ? [] : [`<div style="color:#6b7a94">range ${Math.round(p.min)}–${Math.round(p.max)}</div>`]),
            ...p.others.map((o) => `<div style="color:#6b7a94">${SOURCE_LABEL[o.source] ?? o.source}: ${Math.round(o.g_per_kwh)}</div>`)];
          return `<div style="color:#6b7a94;font-size:11px">${day}</div>${rows.join("")}`;
        },
      },
      xAxis: { type: "category", data: days, boundaryGap: false, ...axis, axisLabel: { ...axis.axisLabel, formatter: (v: string) => v.slice(5), hideOverlap: true } },
      yAxis: { type: "value", name: "gCO₂e/kWh", nameTextStyle: { color: tokens.color.muted, fontSize: 11, align: "left" }, ...axis, splitLine: { lineStyle: { color: tokens.color.grid, type: "dashed" } } },
      series: [
        // the band: an invisible base at the day's minimum and a stacked area of (max - min)
        { name: "Range", type: "line", data: lo, stack: "band", symbol: "none", lineStyle: { opacity: 0 }, silent: true, tooltip: { show: false }, z: 1 },
        { name: "Daily range", type: "line", data: days.map((d) => get(d, (p) => p.max - p.min)), stack: "band", symbol: "none", lineStyle: { opacity: 0 }, areaStyle: { color: `${tokens.color.carbon}22` }, silent: true, z: 1 },
        { name: "Chosen source", type: "line", smooth: 0.2, connectNulls: false, showSymbol: true, symbolSize: 5, data: days.map((d) => get(d, (p) => p.g_per_kwh)),
          lineStyle: { width: 3, color: tokens.color.carbon }, itemStyle: { color: tokens.color.carbon }, z: 3 },
        ...others.map((src, i) => ({
          name: SOURCE_LABEL[src] ?? src, type: "line", smooth: 0.2, showSymbol: false, connectNulls: false,
          data: days.map((d) => get(d, (p) => p.others.find((o) => o.source === src)?.g_per_kwh ?? null)),
          lineStyle: { width: 2, type: "dashed", color: OTHER_COLORS[i % OTHER_COLORS.length] }, itemStyle: { color: OTHER_COLORS[i % OTHER_COLORS.length] }, z: 2,
        })),
      ],
    };
  }, [points, from, to]);
  return <Chart option={option} height={300} ariaLabel="Grid carbon intensity of the region over time" />;
}
