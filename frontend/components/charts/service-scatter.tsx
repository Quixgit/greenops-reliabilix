"use client";

import { useMemo } from "react";
import { tokens } from "@/design-system/tokens";
import type { ServiceRow } from "@/lib/api/schemas";
import { formatCarbon, formatCost, nf } from "@/lib/formatters";
import { Chart } from "./chart";

const COLOR: Record<string, string> = { compute: "#f8a24b", storage: "#4fc3b0", database: "#8b7cf0", networking: "#5aa7f0", other: "#e5b84a" };

/**
 * Are the expensive services also the dirty ones? Each service is a bubble: x is its share of the cost, y its share of
 * the CO2e, area is its cost. Above the diagonal a service emits more than it costs; below it, less.
 */
export function ServiceScatter({ rows, onPick }: { rows: ServiceRow[]; onPick: (service: string) => void }) {
  const option = useMemo(() => {
    const cost = rows.reduce((a, r) => a + Math.max(0, r.cost), 0) || 1;
    const co2 = rows.reduce((a, r) => a + Math.max(0, r.carbon_kg_co2e), 0) || 1;
    const maxCost = Math.max(1, ...rows.map((r) => r.cost));
    const pts = rows.map((r) => ({
      name: r.service, value: [(r.cost / cost) * 100, (r.carbon_kg_co2e / co2) * 100, r.cost], raw: r,
      itemStyle: { color: `${COLOR[r.category] ?? COLOR.other}cc`, borderColor: "#fff", borderWidth: 2 },
    }));
    const top = Math.max(10, ...pts.map((p) => Math.max(p.value[0], p.value[1]))) * 1.12;
    return {
      grid: { left: 8, right: 16, top: 16, bottom: 28, containLabel: true },
      tooltip: {
        trigger: "item", backgroundColor: "#fff", borderColor: "#e8edf5", padding: [10, 14], extraCssText: "border-radius:12px;box-shadow:0 10px 30px rgba(31,52,110,.16);", textStyle: { color: "#0f1b33", fontSize: 12 },
        formatter: (p: { data: { raw: ServiceRow; value: number[] } }) => {
          const r = p.data.raw;
          return `<b>${r.service}</b><div style="color:#6b7a94;margin-top:4px">${formatCost(r.cost)} · ${nf(0).format(p.data.value[0])}% of cost</div>`
            + `<div style="color:#6b7a94">${formatCarbon(r.carbon_kg_co2e)} · ${nf(0).format(p.data.value[1])}% of CO₂e</div><div style="margin-top:4px;color:#3b7cf4">Click to see it in the table</div>`;
        },
      },
      xAxis: { type: "value", min: 0, max: top, name: "Share of cost, %", nameLocation: "middle", nameGap: 26, nameTextStyle: { color: tokens.color.muted, fontSize: 11 },
        axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: tokens.color.muted, fontSize: 11 }, splitLine: { lineStyle: { color: tokens.color.grid, type: "dashed" } } },
      yAxis: { type: "value", min: 0, max: top, name: "Share of CO₂e, %", nameTextStyle: { color: tokens.color.muted, fontSize: 11, align: "left" },
        axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: tokens.color.muted, fontSize: 11 }, splitLine: { lineStyle: { color: tokens.color.grid, type: "dashed" } } },
      series: [{
        type: "scatter", data: pts, symbolSize: (v: number[]) => 14 + 46 * Math.sqrt(Math.max(0, v[2]) / maxCost),
        emphasis: { scale: 1.12, itemStyle: { shadowBlur: 18, shadowColor: "rgba(59,124,244,.35)" } }, cursor: "pointer",
        markLine: { silent: true, symbol: "none", animation: false, lineStyle: { color: "#b8c4d9", type: "dashed", width: 1 }, label: { show: false }, data: [[{ coord: [0, 0] }, { coord: [top, top] }]] },
        animationDuration: 900, animationEasing: "elasticOut", animationDelay: (i: number) => i * 70,
      }],
    };
  }, [rows]);
  return <Chart option={option} height={300} ariaLabel="Share of cost against share of CO2e per service" onClick={(p) => p.name && onPick(p.name)} />;
}
