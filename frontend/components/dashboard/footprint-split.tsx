"use client";

import { IconDatabase as Layers } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useFinopsSummary, useRegionFootprint } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCarbon, formatCost } from "@/lib/formatters";
import { intensityColor } from "@/lib/intensity-color";
import { share } from "@/lib/overview";
import { regionName } from "@/lib/regions";
import { cn } from "@/lib/utils";

const NAME: Record<string, string> = { aws: "AWS", azure: "Azure", gcp: "Google Cloud", kubernetes: "Kubernetes" };

/** Where cost and emissions come from, as paired bars: by cloud or by region. A region carries its grid intensity. */
export function FootprintSplit() {
  const t = useT();
  const q = useRegionFootprint();
  const cur = useFinopsSummary().data?.currency || "USD";
  const [chosen, setBy] = useState<"provider" | "region" | null>(null);
  const providers = new Set((q.data ?? []).map((r) => r.provider)).size;
  // one cloud makes the by-cloud view a single bar: open on regions then, until the person chooses
  const by = chosen ?? (providers > 1 ? "provider" : "region");
  const rows = useMemo(() => {
    const m = new Map<string, { key: string; label: string; sub: string; cost: number; carbon: number; g: number | null }>();
    for (const r of q.data ?? []) {
      const key = by === "provider" ? r.provider : `${r.provider}/${r.region}`;
      const x = m.get(key) ?? { key, label: by === "provider" ? NAME[r.provider] ?? r.provider : regionName(r.region), sub: by === "provider" ? "" : NAME[r.provider] ?? r.provider, cost: 0, carbon: 0, g: null };
      x.cost += r.cost; x.carbon += r.carbon_kg_co2e;
      if (by === "region") x.g = r.g_per_kwh;
      m.set(key, x);
    }
    return [...m.values()].sort((a, b) => b.cost - a.cost);
  }, [q.data, by]);
  const totalCost = rows.reduce((n, r) => n + r.cost, 0), totalCarbon = rows.reduce((n, r) => n + r.carbon, 0);

  return (
    <Card index={9} interactive={false} className="min-w-0">
      <CardHeader icon={<Layers className="size-[18px] text-violet-500" />} title={t("ov2.split.title")} info={t("info.split")}
        action={
          <div role="tablist" className="flex rounded-lg bg-slate-100 p-0.5 text-xs">
            {(["provider", "region"] as const).map((k) => (
              <button key={k} role="tab" aria-selected={by === k} onClick={() => setBy(k)}
                className={cn("rounded-md px-2.5 py-1 font-medium transition-all duration-200", by === k ? "bg-white text-ink shadow-sm" : "text-muted hover:text-ink")}>{t(`ov2.split.by_${k}`)}</button>
            ))}
          </div>
        } />
      {q.isLoading ? <Skeleton className="h-60" /> : rows.length === 0 ? <EmptyState icon={Layers} text={t("ov2.split.empty")} action={{ href: "/connectors", label: t("ov2.empty.connect") }} /> : (
        <ul key={by} className="space-y-4">
          {rows.slice(0, 8).map((r, i) => (
            <li key={r.key} className="rise" style={{ "--i": i } as CSSProperties}>
              <div className="flex items-baseline justify-between gap-3">
                <span className="flex min-w-0 items-center gap-2 text-sm font-medium">
                  {r.g !== null && <span className="size-2.5 shrink-0 rounded-full ring-1 ring-black/5" style={{ background: intensityColor(r.g) }} title={`${Math.round(r.g)} gCO₂e/kWh`} />}
                  <span className="truncate">{r.label}</span>
                  {r.sub && <span className="shrink-0 text-xs font-normal text-muted">{r.sub}</span>}
                </span>
                <span className="shrink-0 text-xs tabular-nums text-muted">{formatCost(r.cost, cur)} · {formatCarbon(r.carbon)}</span>
              </div>
              <div className="mt-1.5 space-y-1">
                <div className="h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="bar-x h-full rounded-full bg-pastel-blue" style={{ width: `${Math.max(2, share(r.cost, totalCost))}%`, "--d": `${i * 60}ms` } as CSSProperties} /></div>
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
