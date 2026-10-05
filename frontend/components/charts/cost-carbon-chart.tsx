"use client";

import { useMemo } from "react";
import { tokens } from "@/design-system/tokens";
import type { TrendPoint } from "@/lib/api/schemas";
import { formatCarbon, formatCost } from "@/lib/formatters";
import { Chart } from "./chart";

/** Dual-axis line chart: Cost (left, blue) vs CO2e (right, green) with one shared tooltip per day. */
export function CostCarbonChart({ points }: { points: TrendPoint[] }) {
  const option = useMemo(() => {
    const days = points.map((p) => p.day.slice(0, 10));
    const axis = { axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: tokens.color.muted } };
    return {
      grid: { left: 8, right: 8, top: 36, bottom: 8, containLabel: true },
      legend: { top: 0, right: 0, icon: "circle", textStyle: { color: tokens.color.muted } },
      tooltip: {
        trigger: "axis",
        valueFormatter: undefined,
        formatter: (raw: unknown) => {
          const arr = raw as { axisValue: string; seriesName: string; value: number; marker: string }[];
          const rows = arr.map((a) => `${a.marker} ${a.seriesName}: <b>${a.seriesName === "Cost" ? formatCost(a.value) : formatCarbon(a.value)}</b>`);
          return `${arr[0]?.axisValue}<br/>${rows.join("<br/>")}`;
        },
      },
      xAxis: { type: "category", data: days, boundaryGap: false, ...axis, axisLabel: { ...axis.axisLabel, formatter: (v: string) => v.slice(5) } },
      yAxis: [
        { type: "value", ...axis, splitLine: { lineStyle: { color: tokens.color.grid } }, axisLabel: { ...axis.axisLabel, formatter: (v: number) => formatCost(v) } },
        { type: "value", ...axis, splitLine: { show: false }, axisLabel: { ...axis.axisLabel, formatter: (v: number) => formatCarbon(v) } },
      ],
      series: [
        { name: "Cost", type: "line", yAxisIndex: 0, smooth: true, showSymbol: false, data: points.map((p) => p.cost), lineStyle: { width: 3, color: tokens.color.cost }, itemStyle: { color: tokens.color.cost } },
        { name: "CO₂e", type: "line", yAxisIndex: 1, smooth: true, showSymbol: false, data: points.map((p) => p.carbon_kg_co2e), lineStyle: { width: 3, color: tokens.color.carbon }, itemStyle: { color: tokens.color.carbon } },
      ],
    };
  }, [points]);
  return <Chart option={option} height={300} ariaLabel="Cost versus carbon footprint over time" />;
}
