"use client";

import { useMemo, useState } from "react";
import type { RegionIntensity } from "@/lib/api/schemas";
import { ensureWorldMap } from "@/lib/echarts";
import { REGION_COUNTRY } from "@/lib/regions";
import { tokens } from "@/design-system/tokens";
import { Chart } from "./chart";

/** World choropleth of grid carbon intensity (gCO2e/kWh), Low (green) -> High (red). */
export function RegionMap({ regions }: { regions: RegionIntensity[] }) {
  const [ready, setReady] = useState(false);
  const option = useMemo(() => {
    const byCountry = new Map<string, number[]>();
    for (const r of regions) {
      const c = REGION_COUNTRY[r.region];
      if (c) byCountry.set(c, [...(byCountry.get(c) ?? []), r.g_per_kwh]);
    }
    const data = [...byCountry].map(([name, v]) => ({ name, value: v.reduce((a, b) => a + b, 0) / v.length }));
    const max = Math.max(500, ...data.map((d) => d.value));
    return {
      tooltip: { trigger: "item", backgroundColor: "#fff", borderColor: "#e8edf5", padding: [8, 12], extraCssText: "border-radius:12px;box-shadow:0 10px 30px rgba(31,52,110,.16);", textStyle: { color: "#0f1b33", fontSize: 12 }, formatter: (p: { name: string; value?: number }) => p.value == null || isNaN(p.value) ? p.name : `${p.name}<br/><b>${Math.round(p.value)}</b> gCO₂e/kWh` },
      visualMap: { min: 0, max, orient: "horizontal", left: "center", bottom: 0, text: ["High", "Low"], calculable: false, itemWidth: 14, itemHeight: 120,
        inRange: { color: ["#5fcf80", "#d6e66b", "#f7c948", "#f59a4a", "#ee6055"] }, textStyle: { color: tokens.color.muted } },
      series: [{
        type: "map", map: "world", roam: false, nameProperty: "name", data,
        itemStyle: { areaColor: "#e9eef6", borderColor: "#ffffff", borderWidth: 0.6 },
        emphasis: { label: { show: false }, itemStyle: { borderColor: "#3b7cf4", borderWidth: 1.2, shadowBlur: 10, shadowColor: "rgba(59,124,244,0.35)" } },
        left: 0, right: 0, top: 4, bottom: 34, aspectScale: 0.92, boundingCoords: [[-168, 80], [179, -48]],
      }],
    };
  }, [regions]);
  if (!ready && typeof window !== "undefined") { ensureWorldMap(); queueMicrotask(() => setReady(true)); }
  return ready ? <Chart option={option} height={210} ariaLabel="Carbon intensity of the electricity grid by region" /> : <div style={{ height: 210 }} />;
}
