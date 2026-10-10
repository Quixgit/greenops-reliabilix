"use client";

import { ArrowDown, ArrowUp, Globe2, List, Map as MapIcon } from "lucide-react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useMemo, useState } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { intensityColor } from "@/lib/intensity-color";
import { useFinopsSummary, useRegionFootprint, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { formatCarbon, formatCost, formatPct, nf } from "@/lib/formatters";
import { REGION_LABEL, regionProvider } from "@/lib/regions";
import { cn } from "@/lib/utils";

// The map is a separate chunk loaded after the page is interactive; the placeholder keeps its exact size (no layout jump).
const RegionMap = dynamic(() => import("@/components/charts/region-map").then((m) => m.RegionMap), {
  ssr: false,
  loading: () => <div className="w-full animate-pulse rounded-2xl bg-slate-100/80" style={{ aspectRatio: "360 / 138" }} />,
});

type Row = { region: string; g: number | null; change: number | null; f?: import("@/lib/api/schemas").RegionFootprint };

/** Beside the map: where the footprint is, the cleanest and the dirtiest grid, and the best move: the answers without clicking. */
function RegionSummary({ rows, cur }: { rows: Row[]; cur: string }) {
  const t = useT();
  const used = rows.filter((r) => r.f && (r.f.carbon_kg_co2e > 0 || r.f.cost > 0)).slice(0, 4);
  const withG = rows.filter((r) => r.g !== null).sort((a, b) => a.g! - b.g!);
  const best = rows.filter((r) => r.f?.alternative).sort((a, b) => b.f!.alternative!.carbon_saving_kg - a.f!.alternative!.carbon_saving_kg)[0];
  const stat = (label: string, r: Row | undefined) => r && (
    <div className="flex items-center justify-between gap-3 rounded-xl bg-slate-50 px-3 py-2 text-sm">
      <span className="text-muted">{label}</span>
      <span className="flex items-center gap-2 font-medium"><span className="size-2.5 rounded-full" style={{ background: intensityColor(r.g!) }} />{REGION_LABEL[r.region] ?? r.region}<span className="tabular-nums text-muted">{nf(0).format(r.g!)}</span></span>
    </div>
  );
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-muted">{t("ov.reg.where")}</p>
      {used.length === 0 ? <p className="text-sm text-muted">{t("ov.reg.no_resources")}</p> : (
        <ul className="space-y-2">
          {used.map((r) => (
            <li key={r.region} className="rounded-xl border border-line/70 px-3 py-2">
              <div className="flex items-center justify-between gap-2 text-sm">
                <span className="min-w-0 truncate font-medium">{REGION_LABEL[r.region] ?? r.region}</span>
                <span className="shrink-0 tabular-nums">{formatCarbon(r.f!.carbon_kg_co2e)}</span>
              </div>
              <div className="mt-0.5 flex justify-between text-xs text-muted">
                <span className="font-mono">{r.region} · {regionProvider(r.region)}</span><span className="tabular-nums">{formatCost(r.f!.cost, cur)}</span>
              </div>
            </li>
          ))}
        </ul>
      )}
      {stat(t("ov.reg.cleanest"), withG[0])}
      {withG.length > 1 && stat(t("ov.reg.highest"), withG[withG.length - 1])}
      {best?.f?.alternative && (
        <div className="rounded-xl bg-emerald-50/70 px-3 py-2.5 text-xs leading-relaxed text-emerald-900 ring-1 ring-emerald-100">
          {t("ov.reg.best", { from: best.region, to: best.f.alternative.region, saving: formatCarbon(best.f.alternative.carbon_saving_kg), pct: nf(0).format(best.f.alternative.carbon_saving_pct) })}
        </div>
      )}
    </div>
  );
}

export function RegionIntensity() {
  const t = useT();
  const q = useRegions();
  const fp = useRegionFootprint();
  const cur = useFinopsSummary().data?.currency || "USD";
  const [view, setView] = useState<"map" | "list">("map");

  // One row per region: the grid reading, what the account spends and emits there, and the cleaner option if any.
  const rows = useMemo(() => {
    const byRegion = new Map((fp.data ?? []).map((f) => [f.region, f]));
    const seen = new Set<string>();
    const out = (q.data ?? []).map((r) => { seen.add(r.region); return { region: r.region, g: r.g_per_kwh as number | null, change: r.change_pct, f: byRegion.get(r.region) }; });
    for (const f of fp.data ?? []) if (!seen.has(f.region)) out.push({ region: f.region, g: f.g_per_kwh, change: null, f });
    // used regions first (largest emitter first), then the rest from clean to dirty
    return out.sort((a, b) => (b.f?.carbon_kg_co2e ?? -1) - (a.f?.carbon_kg_co2e ?? -1) || (a.g ?? 1e9) - (b.g ?? 1e9));
  }, [q.data, fp.data]);

  const toggle = (
    <div className="inline-flex rounded-xl bg-slate-100 p-0.5 text-xs font-medium" role="tablist" aria-label="Regions view">
      {([["map", MapIcon, t("ov.reg.map")], ["list", List, t("ov.reg.list")]] as const).map(([k, Icon, label]) => (
        <button key={k} type="button" role="tab" aria-selected={view === k} onClick={() => setView(k)}
          className={cn("inline-flex items-center gap-1.5 rounded-[10px] px-3 py-1.5 transition", view === k ? "bg-white text-ink shadow-sm" : "text-muted hover:text-ink")}>
          <Icon className="size-3.5" />{label}
        </button>
      ))}
    </div>
  );

  return (
    <Card index={7} interactive={false}>
      <CardHeader title={t("ov.reg.title")} action={<span className="flex items-center gap-3">{toggle}<Link href="/carbon" className="hidden text-xs font-semibold text-primary hover:underline sm:inline">{t("ov.svc.link")}</Link></span>} />
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError ? <ErrorState /> :
        rows.length === 0 ? <EmptyState icon={Globe2} text={t("ov.reg.empty")} /> : view === "map" ? (
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1.7fr)_minmax(0,1fr)]">
            <div className="min-w-0"><RegionMap regions={q.data!} footprint={fp.data ?? []} /></div>
            <RegionSummary rows={rows} cur={cur} />
          </div>
        ) : (
          <div className="-mx-1 overflow-x-auto px-1">
            <table className="w-full min-w-[640px] text-sm">
              <thead>
                <tr className="text-left text-xs text-muted">
                  <th className="pb-2 font-medium">{t("ov.reg.region")}</th><th className="pb-2 font-medium">{t("ov.reg.cloud")}</th>
                  <th className="pb-2 text-right font-medium" title="Latest grid reading of the most trusted source">gCO₂e/kWh</th>
                  <th className="pb-2 text-right font-medium">{t("ov.reg.cost")}</th><th className="pb-2 text-right font-medium" title="Operational emissions of your resources in the period">CO₂e</th>
                  <th className="pb-2 pl-4 font-medium" title={t("ov.reg.improve_hint")}>{t("ov.reg.improve")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={r.region} className="border-t border-line/70 transition-colors hover:bg-slate-50/70">
                    <td className="py-2.5"><div className="font-medium">{REGION_LABEL[r.region] ?? r.region}</div><div className="font-mono text-[11px] text-muted">{r.region}</div></td>
                    <td className="py-2.5 text-xs text-muted">{regionProvider(r.region)}</td>
                    <td className="py-2.5 text-right tabular-nums">
                      <span className="mr-2 inline-block size-2.5 rounded-full align-middle" style={{ background: r.g === null ? "#e4eaf3" : intensityColor(r.g) }} />
                      {r.g === null ? "—" : nf(0).format(r.g)}
                      {r.change !== null && (
                        <span className={cn("ml-2 inline-flex items-center text-xs font-semibold", r.change <= 0 ? "text-emerald-600" : "text-bad")}>
                          {r.change <= 0 ? <ArrowDown className="size-3" strokeWidth={2.5} /> : <ArrowUp className="size-3" strokeWidth={2.5} />}{formatPct(r.change)}
                        </span>
                      )}
                    </td>
                    <td className="py-2.5 text-right tabular-nums">{r.f ? formatCost(r.f.cost, cur) : <span className="text-muted">—</span>}</td>
                    <td className="py-2.5 text-right tabular-nums">{r.f ? formatCarbon(r.f.carbon_kg_co2e) : <span className="text-muted">—</span>}</td>
                    <td className="py-2.5 pl-4 text-xs">
                      {r.f?.alternative ? (
                        <span className="text-emerald-700"><b>{r.f.alternative.region}</b> · −{nf(0).format(r.f.alternative.carbon_saving_pct)}% grid, up to ≈{formatCarbon(r.f.alternative.carbon_saving_kg)}
                          {r.f.alternative.cost_delta !== null && <span className="block font-medium text-slate-700">{t("map.cost_delta_short", { amount: `${r.f.alternative.cost_delta > 0 ? "+" : "−"}${formatCost(Math.abs(r.f.alternative.cost_delta), cur)}` })}</span>}
                          <span className="block text-muted">{t("ov.reg.unchecked")}</span></span>
                      ) : <span className="text-muted" title={r.f?.relocation_blocked ? t("ov.reg.blocked_hint") : undefined}>
                          {!r.f ? t("ov.reg.unused") : r.f.relocation_blocked === "residency_unknown" ? t("ov.reg.residency_unknown") : r.f.relocation_blocked === "residency" ? t("ov.reg.residency_none") : t("ov.reg.none_cleaner")}</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
    </Card>
  );
}
