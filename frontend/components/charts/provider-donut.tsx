"use client";

import { useMemo } from "react";
import { tokens } from "@/design-system/tokens";
import type { ProviderShare } from "@/lib/api/schemas";
import { carbonParts, formatCarbon, nf } from "@/lib/formatters";
import { Chart } from "./chart";

const NAME: Record<string, string> = { aws: "AWS", azure: "Azure", gcp: "GCP" };
export const providerName = (p: string) => NAME[p] ?? "Other";
export const providerColor = (p: string) => tokens.provider[p] ?? tokens.provider.other;

/** Donut with the total in the middle, legend beside it. Only providers with data appear: one cloud = one 100% ring. */
export function ProviderDonut({ shares }: { shares: ProviderShare[] }) {
  const total = shares.reduce((a, s) => a + s.carbon_kg_co2e, 0);
  const t = carbonParts(total);
  const option = useMemo(() => ({
    tooltip: { trigger: "item", backgroundColor: "#fff", borderColor: "#e8edf5", padding: [8, 12], extraCssText: "border-radius:12px;box-shadow:0 10px 30px rgba(31,52,110,.16);", textStyle: { color: "#0f1b33", fontSize: 12 },
      formatter: (p: { name: string; value: number; percent: number; marker: string }) => `${p.marker} ${p.name}: <b>${formatCarbon(p.value)}</b> (${nf(1).format(p.percent)}%)` },
    title: { text: `${nf(1).format(t.n)} ${t.unit}`, subtext: "CO₂e", left: "center", top: "38%", itemGap: 2, textStyle: { fontSize: 22, fontWeight: 700, color: tokens.color.text }, subtextStyle: { color: tokens.color.muted, fontSize: 12 } },
    series: [{
      type: "pie", radius: ["64%", "84%"], avoidLabelOverlap: true, label: { show: false }, padAngle: shares.length > 1 ? 2 : 0,
      itemStyle: { borderRadius: 7, borderColor: "#fff", borderWidth: 2 },
      emphasis: { scaleSize: 7, itemStyle: { shadowBlur: 18, shadowColor: "rgba(31,52,110,0.25)" } },
      animationType: "expansion", animationDuration: 1000,
      data: shares.map((s) => ({ name: providerName(s.provider), value: s.carbon_kg_co2e, itemStyle: { color: providerColor(s.provider) } })),
    }],
  }), [shares, t.n, t.unit]);

  return (
    <div className="grid items-center gap-4 sm:grid-cols-[minmax(150px,0.9fr)_minmax(0,1.2fr)]">
      <Chart option={option} height={200} ariaLabel="Carbon footprint by cloud provider" />
      <ul className="w-full space-y-3.5 text-sm">
        {shares.map((s, i) => (
          <li key={s.provider} className="rise flex items-center gap-2.5" style={{ "--i": i + 3 } as React.CSSProperties}>
            <span className="size-3 rounded-full" style={{ background: providerColor(s.provider) }} />
            <span className="w-12 font-medium">{providerName(s.provider)}</span>
            <span className="w-10 text-right tabular-nums text-slate-600">{nf(0).format(s.share_pct)}%</span>
            <span className="ml-auto whitespace-nowrap text-right tabular-nums text-slate-600">{formatCarbon(s.carbon_kg_co2e)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
