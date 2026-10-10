"use client";

import { useMemo } from "react";
import { tokens } from "@/design-system/tokens";
import type { TrendPoint } from "@/lib/api/schemas";
import { formatCarbon, formatCost, getDefaultCurrency, getFormatLocale } from "@/lib/formatters";
import { Chart } from "./chart";

const compactMoney = (currency: string) => (v: number) =>
  new Intl.NumberFormat(getFormatLocale(), { style: "currency", currency, notation: "compact", maximumFractionDigits: 0 }).format(v);

/** Dual-axis area chart: Cost (left, blue) vs CO2e (right, green) with one shared tooltip per day. */
export function CostCarbonChart({ points, currency = getDefaultCurrency() }: { points: TrendPoint[]; currency?: string }) {
  const compact = compactMoney(currency);
  const costLabel = `Cost (${currency})`;
  const option = useMemo(() => {
    const days = points.map((p) => p.day.slice(0, 10));
    const axis = { axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: tokens.color.muted, fontSize: 12 } };
    const area = (c: string) => ({ color: { type: "linear", x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: `${c}33` }, { offset: 1, color: `${c}03` }] } });
    return {
      grid: { left: 4, right: 4, top: 40, bottom: 4, containLabel: true },
      legend: { top: 0, right: 4, icon: "circle", itemWidth: 9, itemHeight: 9, itemGap: 18, textStyle: { color: "#475569", fontSize: 12 }, data: [costLabel, "CO₂e"] },
      tooltip: {
        trigger: "axis", backgroundColor: "#fff", borderColor: "#e8edf5", borderWidth: 1, padding: [10, 14], extraCssText: "border-radius:12px;box-shadow:0 10px 30px rgba(31,52,110,.16);",
        textStyle: { color: "#0f1b33", fontSize: 12 }, axisPointer: { type: "line", lineStyle: { type: "dashed", color: "#b8c4d9" } },
        formatter: (raw: unknown) => {
          const arr = raw as { axisValue: string; seriesName: string; value: number; marker: string }[];
          const rows = arr.map((a) => `<div style="display:flex;justify-content:space-between;gap:18px;margin-top:4px">${a.marker}<span style="flex:1;color:#6b7a94">${a.seriesName.replace(/ \([A-Z]{3}\)$/, "")}</span><b>${a.seriesName.startsWith("Cost") ? formatCost(a.value) : formatCarbon(a.value)}</b></div>`);
          return `<div style="color:#6b7a94;font-size:11px">${arr[0]?.axisValue}</div>${rows.join("")}`;
        },
      },
      xAxis: { type: "category", data: days, boundaryGap: false, ...axis, axisLabel: { ...axis.axisLabel, formatter: (v: string) => v.slice(5), hideOverlap: true } },
      yAxis: [
        { type: "value", ...axis, splitLine: { lineStyle: { color: tokens.color.grid, type: "dashed" } }, axisLabel: { ...axis.axisLabel, formatter: compact } },
        { type: "value", ...axis, splitLine: { show: false }, axisLabel: { ...axis.axisLabel, formatter: (v: number) => formatCarbon(v) } },
      ],
      series: [
        { name: costLabel, type: "line", yAxisIndex: 0, smooth: 0.35, showSymbol: false, symbolSize: 9, data: points.map((p) => p.cost), z: 3,
          lineStyle: { width: 3, color: tokens.color.cost }, itemStyle: { color: tokens.color.cost, borderColor: "#fff", borderWidth: 2 }, areaStyle: area(tokens.color.cost), emphasis: { focus: "series" }, animationDelay: 0 },
        { name: "CO₂e", type: "line", yAxisIndex: 1, smooth: 0.35, showSymbol: false, symbolSize: 9, data: points.map((p) => p.carbon_kg_co2e), z: 2,
          lineStyle: { width: 3, color: tokens.color.carbon }, itemStyle: { color: tokens.color.carbon, borderColor: "#fff", borderWidth: 2 }, emphasis: { focus: "series" }, animationDelay: 250 },
      ],
    };
  }, [points, currency]);
  return <Chart option={option} height={300} ariaLabel="Cost versus carbon footprint over time" />;
}
