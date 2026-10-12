"use client";

import { IconDatabase as Layers } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, useState, type CSSProperties } from "react";
import { Card } from "@/components/ui/card";
import { Segmented } from "@/components/ui/controls";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useFinopsSummary, useRegionFootprint } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCarbon, formatCost, formatNative } from "@/lib/formatters";
import { intensityColor } from "@/lib/intensity-color";
import { share } from "@/lib/overview";
import { cloudMeta } from "@/lib/clouds";
import { CloudMark } from "@/components/dashboard/cloud-mark";
import { regionName } from "@/lib/regions";

const NAME = (p: string) => cloudMeta(p).name;

/** Where cost and emissions come from, as paired bars: by cloud or by region. A region carries its grid intensity. */
export function FootprintSplit() {
  const t = useT();
  const q = useRegionFootprint();
  const summary = useFinopsSummary().data;
  const cur = summary?.currency || "USD";
  const native = useMemo(() => new Map((summary?.by_provider ?? []).map((p) => [p.provider, p.native])), [summary]);
  const [chosen, setBy] = useState<"provider" | "region" | null>(null);
  const providers = new Set((q.data ?? []).map((r) => r.provider)).size;
  // one cloud makes the by-cloud view a single bar: open on regions then, until the person chooses
  const by = chosen ?? (providers > 1 ? "provider" : "region");
  const rows = useMemo(() => {
    const m = new Map<string, { key: string; label: string; sub: string; cost: number; carbon: number; g: number | null; prov: string }>();
    for (const r of q.data ?? []) {
      const key = by === "provider" ? r.provider : `${r.provider}/${r.region}`;
      const x = m.get(key) ?? { key, label: by === "provider" ? NAME(r.provider) : regionName(r.region), sub: by === "provider" ? "" : NAME(r.provider), cost: 0, carbon: 0, g: null, prov: r.provider };
      x.cost += r.cost; x.carbon += r.carbon_kg_co2e;
      if (by === "region") x.g = r.g_per_kwh;
      m.set(key, x);
    }
    return [...m.values()].sort((a, b) => b.cost - a.cost);
  }, [q.data, by]);
  const totalCost = rows.reduce((n, r) => n + r.cost, 0), totalCarbon = rows.reduce((n, r) => n + r.carbon, 0);

  return (
    <Card index={9} interactive={false} className="min-w-0 p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink"><Glyph name="layers" className="size-[18px] text-[#7461d4]" />{t("ov2.split.title")}</h2><Info corner>{t("info.split")}</Info>
        <Segmented size="sm" label={t("ov2.split.title")} value={by} onChange={setBy}
          options={[{ value: "provider", label: t("ov2.split.by_provider"), disabled: providers < 2 }, { value: "region", label: t("ov2.split.by_region") }]} />
      </div>
      {q.isLoading ? <Skeleton className="h-60" /> : rows.length === 0 ? <EmptyState icon={Layers} text={t("ov2.split.empty")} action={{ href: "/connectors", label: t("ov2.empty.connect") }} /> : (
        <ul key={by} className="space-y-3">
          {rows.slice(0, 6).map((r, i) => (
            <li key={r.key} className="rise" style={{ "--i": i } as CSSProperties}>
              <div className="flex items-baseline justify-between gap-3">
                <span className="flex min-w-0 items-center gap-2 text-sm font-medium">
                  {by === "provider" ? <CloudMark provider={r.prov} className="size-6" />
                    : r.g !== null && <span className="size-2.5 shrink-0 rounded-full ring-1 ring-black/5" style={{ background: intensityColor(r.g) }} title={`${Math.round(r.g)} gCO₂e/kWh`} />}
                  <span className="truncate">{r.label}</span>
                  {r.sub && providers > 1 && <span className="shrink-0 rounded-md px-1.5 py-px text-[10px] font-semibold" style={{ background: cloudMeta(r.prov).soft, color: cloudMeta(r.prov).ink }}>{r.sub}</span>}
                </span>
                <span className="shrink-0 text-right tabular-nums">
                  <span className="block text-sm font-semibold text-ink">{formatCost(r.cost, cur)}</span>
                  <span className="block text-[11px] text-muted">
                    {by === "provider" && (native.get(r.prov) ?? []).filter((n) => n.currency !== cur).map((n) => `${formatNative(n.amount, n.currency)} · `).join("")}{formatCarbon(r.carbon)}
                  </span>
                </span>
              </div>
              <div className="mt-1.5 space-y-1">
                <div className="h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="bar-x h-full rounded-full" style={{ width: `${Math.max(2, share(r.cost, totalCost))}%`, background: by === "provider" ? cloudMeta(r.prov).color : "#8db4ee", "--d": `${i * 60}ms` } as CSSProperties} /></div>
                <div className="h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="bar-x h-full rounded-full bg-leaf" style={{ width: `${Math.max(2, share(r.carbon, totalCarbon))}%`, "--d": `${i * 60 + 80}ms` } as CSSProperties} /></div>
              </div>
            </li>
          ))}
          <li className="flex items-center justify-between border-t border-line/70 pt-3 text-xs text-muted">
            <span className="flex items-center gap-3"><span className="flex items-center gap-1.5"><span className="h-1.5 w-4 rounded bg-pastel-blue" />{t("ov2.split.cost")}</span><span className="flex items-center gap-1.5"><span className="h-1.5 w-4 rounded bg-leaf" />CO₂e</span></span>
            <Link href="/carbon" className="font-semibold text-primary hover:underline">{t("ov2.split.details")}</Link>
          </li>
        </ul>
      )}
    </Card>
  );
}
