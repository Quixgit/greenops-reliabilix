"use client";

import { useMemo } from "react";
import { tokens } from "@/design-system/tokens";
import type { ProviderShare } from "@/lib/api/schemas";
import { formatCarbon, nf } from "@/lib/formatters";
import { Chart } from "./chart";

const NAME: Record<string, string> = { aws: "AWS", azure: "Azure", gcp: "GCP" };
export const providerName = (p: string) => NAME[p] ?? "Other";
export const providerColor = (p: string) => tokens.provider[p] ?? tokens.provider.other;

/** Donut + legend. Only providers that actually have data appear: one connected cloud = one 100% segment. */
export function ProviderDonut({ shares }: { shares: ProviderShare[] }) {
  const total = shares.reduce((a, s) => a + s.carbon_kg_co2e, 0);
  const option = useMemo(() => ({
    tooltip: { trigger: "item", formatter: (p: { name: string; value: number; percent: number }) => `${p.name}: <b>${formatCarbon(p.value)}</b> (${nf(1).format(p.percent)}%)` },
    title: { text: formatCarbon(total), subtext: "Total CO₂e", left: "center", top: "38%", textStyle: { fontSize: 20, fontWeight: 700, color: tokens.color.text }, subtextStyle: { color: tokens.color.muted } },
    series: [{
      type: "pie", radius: ["62%", "82%"], avoidLabelOverlap: true, label: { show: false }, padAngle: shares.length > 1 ? 2 : 0,
      itemStyle: { borderRadius: 6 },
      data: shares.map((s) => ({ name: providerName(s.provider), value: s.carbon_kg_co2e, itemStyle: { color: providerColor(s.provider) } })),
    }],
  }), [shares, total]);

  return (
    <div className="flex flex-col items-center gap-4">
      <div className="w-full"><Chart option={option} height={200} ariaLabel="Carbon footprint by cloud provider" /></div>
      <ul className="w-full space-y-3 text-sm">
        {shares.map((s) => (
          <li key={s.provider} className="flex items-center justify-between gap-3">
            <span className="flex items-center gap-2"><span className="size-2.5 rounded-full" style={{ background: providerColor(s.provider) }} />{providerName(s.provider)}</span>
            <span className="text-right leading-tight"><b className="block">{nf(0).format(s.share_pct)}%</b><span className="text-xs text-muted">{formatCarbon(s.carbon_kg_co2e)}</span></span>
          </li>
        ))}
      </ul>
    </div>
  );
}
