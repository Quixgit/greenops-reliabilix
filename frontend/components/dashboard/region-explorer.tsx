"use client";

import { IconGlobe as Globe2 } from "@/components/ui/icons";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useEffect, useMemo, useState, type CSSProperties } from "react";
import { MiniArea } from "@/components/charts/mini-area";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { useFinopsSummary, useIntensityHistory, useRegionFootprint, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import type { RegionFootprint } from "@/lib/api/schemas";
import { formatCarbon, formatCost, nf } from "@/lib/formatters";
import { intensityColor } from "@/lib/intensity-color";
import { REGION_GEO, flag, regionName, regionProvider } from "@/lib/regions";
import { cn } from "@/lib/utils";
import { AllowRegions } from "./allow-regions";

const RegionMap = dynamic(() => import("@/components/charts/region-map").then((m) => m.RegionMap), {
  ssr: false,
  loading: () => <div className="w-full animate-pulse rounded-2xl bg-slate-100/80" style={{ aspectRatio: "360 / 138" }} />,
});

type Row = { region: string; g: number | null; change: number | null; fp?: RegionFootprint };

function band(g: number | null): { key: string; tone: string } {
  if (g === null) return { key: "unknown", tone: "text-slate-500 bg-slate-100" };
  if (g < 150) return { key: "clean", tone: "text-leaf-deep bg-leaf-soft" };
  if (g < 300) return { key: "moderate", tone: "text-lime-700 bg-lime-50" };
  if (g < 450) return { key: "high", tone: "text-amber-700 bg-amber-50" };
  return { key: "very_high", tone: "text-rose-700 bg-rose-50" };
}

/** What one selected region means for the organization: the grid now and over time, the footprint there, what moving would do. */
function Detail({ row, rows, cur }: { row: Row; rows: Row[]; cur: string }) {
  const t = useT();
  const hist = useIntensityHistory(row.region);
  const geo = REGION_GEO[row.region];
  const cloud = regionProvider(row.region);
  const sameCloud = rows.filter((r) => r.g !== null && regionProvider(r.region) === cloud && r.region !== row.region).sort((a, b) => a.g! - b.g!);
  const cleaner = sameCloud[0];
  const b = band(row.g);
  const series = (hist.data?.points ?? []).map((p) => p.g_per_kwh);
  const alt = row.fp?.alternative;
  const g = row.g ?? 0;
  return (
    <div key={row.region} className="promo-in flex h-full flex-col gap-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="flex items-center gap-2 text-lg font-semibold leading-tight tracking-tight">
            {geo && <span aria-hidden className="text-xl">{flag(geo.cc)}</span>}<span className="truncate">{regionName(row.region)}</span>
          </p>
          <p className="mt-0.5 font-mono text-xs text-muted">{row.region} · {cloud}</p>
        </div>
        <span className={cn("shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold", b.tone)}>{t(`ov2.band.${b.key}`, undefined, b.key)}</span>
      </div>

      <div className="rounded-2xl bg-gradient-to-br from-slate-50 to-white p-4 ring-1 ring-line">
        <div className="flex items-end justify-between gap-3">
          <p className="text-4xl font-bold leading-none tracking-tight tabular-nums">{row.g === null ? "—" : nf(0).format(row.g)}<span className="ml-1.5 text-sm font-medium text-muted">gCO₂e/kWh</span></p>
          {row.change !== null && <span className={cn("text-xs font-semibold", row.change <= 0 ? "text-leaf-deep" : "text-bad")}>{row.change <= 0 ? "↓" : "↑"} {nf(1).format(Math.abs(row.change))}% · 24h</span>}
        </div>
        <div className="relative mt-3 h-2 rounded-full bg-gradient-to-r from-leaf via-pastel-amber to-pastel-rose">
          {row.g !== null && <span className="absolute top-1/2 size-4 -translate-x-1/2 -translate-y-1/2 rounded-full border-[3px] border-white bg-slate-800 shadow transition-all duration-700" style={{ left: `${Math.min(100, (g / 600) * 100)}%` }} />}
        </div>
        <div className="mt-1 flex justify-between text-[10px] text-muted"><span>0</span><span>300</span><span>600+</span></div>
        {series.length > 1 && (
          <div className="mt-3">
            <p className="mb-1 text-[11px] font-medium text-muted">{t("ov2.reg.history", { days: hist.data?.days_with_data ?? series.length })}</p>
            <MiniArea values={series} stroke={row.g !== null ? "#2f8f78" : "#94a3b8"} height={40} />
          </div>
        )}
      </div>

      {row.fp ? (
        <div className="grid grid-cols-3 gap-2 text-center">
          {[
            [t("ov2.reg.co2"), formatCarbon(row.fp.carbon_kg_co2e)], [t("ov2.reg.cost"), formatCost(row.fp.cost, cur)], [t("ov2.reg.energy"), `${nf(row.fp.energy_kwh < 10 ? 1 : 0).format(row.fp.energy_kwh)} kWh`],
          ].map(([k, v], i) => (
            <div key={k} className="rise rounded-xl bg-slate-50 px-2 py-2.5 ring-1 ring-line/70" style={{ "--i": i } as CSSProperties}>
              <p className="text-[11px] text-muted">{k}</p><p className="mt-0.5 text-sm font-semibold tabular-nums">{v}</p>
            </div>
          ))}
        </div>
      ) : <p className="rounded-xl bg-slate-50 px-3 py-2.5 text-xs text-muted ring-1 ring-line/70">{t("ov2.reg.unused")}</p>}

      <div className="mt-auto space-y-2 text-sm">
        {alt ? (
          <div className="rounded-xl bg-leaf-soft px-3 py-3 text-leaf-deep ring-1 ring-leaf-soft">
            <p className="font-semibold">{t("ov2.reg.move", { to: regionName(alt.region), pct: nf(0).format(alt.carbon_saving_pct) })}</p>
            <p className="mt-0.5 text-xs">{t("ov2.reg.move_detail", { kg: formatCarbon(alt.carbon_saving_kg) })}</p>
          </div>
        ) : row.fp && cleaner && cleaner.g! < g ? (
          <div className="rounded-xl bg-amber-50 px-3 py-3 text-xs leading-relaxed text-amber-900 ring-1 ring-amber-100">
            {t("ov2.reg.cleaner", { region: regionName(cleaner.region), g: nf(0).format(cleaner.g!), pct: nf(0).format((1 - cleaner.g! / g) * 100) })}{" "}
            {row.fp.relocation_blocked === "residency_unknown" && <AllowRegions className="text-xs">{t("ov2.opt.set_regions")}</AllowRegions>}
          </div>
        ) : row.fp ? <p className="rounded-xl bg-slate-50 px-3 py-2.5 text-xs text-muted ring-1 ring-line/70">{t("ov2.reg.none_cleaner")}</p> : null}
        <Link href="/carbon" className="inline-block text-xs font-semibold text-primary hover:underline">{t("ov.svc.link")}</Link>
      </div>
    </div>
  );
}

/** The map and what it says: pick a region on the map or in the ranking and the panel on the right answers for it. */
export function RegionExplorer() {
  const t = useT();
  const q = useRegions();
  const fp = useRegionFootprint();
  const cur = useFinopsSummary().data?.currency || "USD";
  const [sel, setSel] = useState<string | null>(null);

  const rows: Row[] = useMemo(() => {
    const byRegion = new Map((fp.data ?? []).map((f) => [f.region, f]));
    const seen = new Set<string>();
    const out: Row[] = (q.data ?? []).map((r) => { seen.add(r.region); return { region: r.region, g: r.g_per_kwh, change: r.change_pct, fp: byRegion.get(r.region) }; });
    for (const f of fp.data ?? []) if (!seen.has(f.region)) out.push({ region: f.region, g: f.g_per_kwh, change: null, fp: f });
    return out;
  }, [q.data, fp.data]);
  const used = useMemo(() => rows.filter((r) => r.fp && (r.fp.carbon_kg_co2e > 0 || r.fp.cost > 0)).sort((a, b) => b.fp!.carbon_kg_co2e - a.fp!.carbon_kg_co2e), [rows]);
  const ranked = useMemo(() => rows.filter((r) => r.g !== null).sort((a, b) => a.g! - b.g!), [rows]);
  useEffect(() => { if (!sel && used[0]) setSel(used[0].region); }, [used, sel]);
  const current = rows.find((r) => r.region === sel) ?? used[0] ?? ranked[0] ?? null;

  // one entry per place: Paris is one grid whichever cloud has a region there
  const places = useMemo(() => {
    const m = new Map<string, { first: Row; clouds: Set<string> }>();
    for (const r of ranked) {
      const key = regionName(r.region); // the label a person reads: two regions with the same label are one place
      const e = m.get(key);
      if (e) e.clouds.add(regionProvider(r.region)); else m.set(key, { first: r, clouds: new Set([regionProvider(r.region)]) });
    }
    return [...m.values()];
  }, [ranked]);
  const maxCarbon = Math.max(1, ...used.map((r) => r.fp!.carbon_kg_co2e));

  return (
    <Card index={11} interactive={false}>
      <CardHeader icon={<Globe2 className="size-[18px] text-leaf-deep" />} title={t("ov.reg.title")}
        info={t("info.regions")}
        action={<Link href="/carbon" className="text-xs font-semibold text-primary hover:underline">{t("ov.svc.link")}</Link>} />
      {q.isLoading ? <Skeleton className="h-80" /> : q.isError ? <ErrorState /> : rows.length === 0 ? <EmptyState icon={Globe2} text={t("ov.reg.empty")} /> : (
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1.55fr)_minmax(0,1fr)]">
          <div className="flex min-w-0 flex-col gap-4">
            <RegionMap regions={q.data!} footprint={fp.data ?? []} selected={current?.region ?? null} onSelect={setSel} />
            <div>
              <p className="mb-2 flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-muted">{t("ov2.reg.yours")}<Info>{t("info.regions_yours")}</Info></p>
              {used.length === 0 ? <p className="text-sm text-muted">{t("ov.reg.no_resources")}</p> : (
                <ul className="grid gap-2 sm:grid-cols-2">
                  {used.slice(0, 6).map((r, i) => (
                    <li key={r.region} className="rise" style={{ "--i": i } as CSSProperties}>
                      <button type="button" onClick={() => setSel(r.region)} aria-pressed={current?.region === r.region}
                        className={cn("w-full rounded-xl border px-3 py-2 text-left transition-all duration-200 hover:-translate-y-0.5",
                          current?.region === r.region ? "border-leaf-deep/60 bg-leaf-soft/60 shadow-sm" : "border-line bg-white hover:border-slate-300 hover:shadow-sm")}>
                        <span className="flex items-center justify-between gap-2 text-sm">
                          <span className="flex min-w-0 items-center gap-2 font-medium"><span className="size-2.5 shrink-0 rounded-full ring-1 ring-black/5" style={{ background: r.g === null ? "#e4eaf3" : intensityColor(r.g) }} /><span className="truncate">{regionName(r.region)}</span></span>
                          <span className="shrink-0 tabular-nums text-xs text-muted">{formatCarbon(r.fp!.carbon_kg_co2e)}</span>
                        </span>
                        <span className="mt-1.5 block h-1 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-leaf" style={{ width: `${Math.max(4, (r.fp!.carbon_kg_co2e / maxCarbon) * 100)}%`, "--d": `${i * 60}ms` } as CSSProperties} /></span>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            {ranked.length > 0 && (
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                <span className="font-medium">{t("ov2.reg.cleanest_now")}</span>
                {ranked.slice(0, 3).map((r) => (
                  <button key={r.region} type="button" onClick={() => setSel(r.region)} className="inline-flex items-center gap-1.5 rounded-full bg-slate-50 px-2.5 py-1 font-medium text-slate-700 ring-1 ring-line transition-colors hover:bg-leaf-soft">
                    <span className="size-2 rounded-full" style={{ background: intensityColor(r.g!) }} />{regionName(r.region)} · {nf(0).format(r.g!)}
                  </button>
                ))}
              </div>
            )}
          </div>
          <div className="min-w-0 rounded-2xl border border-line/80 bg-white p-4 shadow-[0_1px_2px_rgba(16,24,40,0.04)]">
            {current ? <Detail row={current} rows={rows} cur={cur} /> : null}
          </div>
        </div>
      )}
          </Card>
  );
}
