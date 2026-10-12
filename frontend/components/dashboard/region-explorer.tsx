"use client";

import { IconGlobe as Globe2 } from "@/components/ui/icons";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useEffect, useMemo, useState, type CSSProperties } from "react";
import { MiniArea } from "@/components/charts/mini-area";
import { Card } from "@/components/ui/card";
import { Segmented } from "@/components/ui/controls";
import { Glyph } from "@/components/ui/glyphs";
import type { MapMode } from "@/components/charts/dot-world-map";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { useFinopsSummary, useIntensityHistory, useRegionFootprint, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import type { RegionFootprint } from "@/lib/api/schemas";
import { formatCarbon, formatCost, nf } from "@/lib/formatters";
import { intensityColor } from "@/lib/intensity-color";
import { isMeasured, latencyBasisLabel } from "@/lib/latency-basis";
import { REGION_GEO, flag, regionName, regionProvider, latencyBetween } from "@/lib/regions";
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

/** A bar of the estimated round trip against the organization's limit: sky while within it, rose when beyond. */
function LatencyGauge({ ms, km, limit, title, basis, errorPct, right }: { ms: number; km: number; limit: number; title: string; basis?: string; errorPct?: number; right?: React.ReactNode }) {
  const t = useT();
  const over = ms > limit;
  return (
    <div title={title}>
      <div className="relative h-2 rounded-full bg-slate-100">
        <span className={cn("absolute inset-y-0 left-0 rounded-full transition-all duration-700", over ? "bg-[#eab3bf]" : "bg-[#9cc3f2]")} style={{ width: `${Math.max(4, Math.min(100, (ms / (limit * 1.6)) * 100))}%` }} />
        <span className="absolute -top-0.5 h-3 w-px bg-slate-400/70" style={{ left: `${(limit / (limit * 1.6)) * 100}%` }} aria-hidden />
      </div>
      <p className="mt-1.5 flex items-center justify-between gap-2 text-[11px] text-muted">
        <span className={cn("font-semibold", over ? "text-[#b9566a]" : "text-[#3f78b8]")}>{basis === "estimated" || !basis ? "≈ " : ""}{ms} ms · {t("ov2.reg.km", { km: nf(0).format(km) })}</span>
        {right}
      </p>
      {basis && <p className={cn("mt-0.5 text-[10px]", isMeasured(basis) ? "text-leaf-deep" : "text-slate-400")}>{latencyBasisLabel(t, basis, errorPct)}</p>}
    </div>
  );
}

/**
 * Whether a move is realistic: for a region the organization does not run in, each of its own regions with the estimated round trip
 * and the grid difference; for one it does run in, the cleaner option with its round trip. The same lines are drawn on the map.
 */
function Placement({ row, rows, limit }: { row: Row; rows: Row[]; limit: number }) {
  const t = useT();
  const alt = row.fp?.alternative;
  const own = rows.filter((r) => r.fp && r.fp.carbon_kg_co2e > 0 && r.g !== null && r.region !== row.region).sort((a, b) => b.fp!.carbon_kg_co2e - a.fp!.carbon_kg_co2e).slice(0, 3);
  const items = row.g === null || row.fp ? [] : own.map((o) => {
    const lat = latencyBetween(o.region, row.region);
    return { o, km: lat?.km ?? null, ms: lat?.ms ?? null, lat, diff: ((o.g! - row.g!) / o.g!) * 100 };
  });
  const best = items.filter((i) => i.ms !== null && i.ms <= limit && i.diff >= 5).sort((a, b) => b.diff - a.diff)[0];
  if (!alt && items.length === 0) return null;
  return (
    <section className="rounded-2xl bg-white p-3.5 ring-1 ring-line">
      <p className="mb-2.5 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-slate-400">{t("ov2.pl.title")}<Info>{t("ov2.pl.note")}</Info></p>
      {alt ? (
        <div className="space-y-2.5">
          <div className="flex items-end justify-between gap-3">
            <div className="min-w-0">
              <p className="text-xs text-muted">{t("ov2.pl.cleaner_option")}</p>
              <p className="truncate text-sm font-semibold">{regionName(alt.region)}</p>
            </div>
            <p className="shrink-0 rounded-full bg-leaf-soft px-2.5 py-1 text-sm font-bold tabular-nums text-leaf-deep">−{nf(0).format(alt.carbon_saving_pct)}%</p>
          </div>
          <p className="text-xs text-muted">{t("ov2.reg.move_detail", { kg: formatCarbon(alt.carbon_saving_kg) })}</p>
          {alt.latency_ms != null
            ? <LatencyGauge ms={alt.latency_ms} km={alt.distance_km ?? 0} basis={alt.latency_basis} errorPct={alt.latency_error_pct} limit={alt.latency_budget_ms ?? limit} title={t("rec.latency_hint", { from: regionName(row.region), to: regionName(alt.region), km: alt.distance_km ?? "?", budget: alt.latency_budget_ms ?? "?" })} />
            : <p className="text-[11px] text-muted">{t("rec.latency")}: {t("rec.latency_unknown")}</p>}
        </div>
      ) : (
        <div className="space-y-3">
          <p className={cn("rounded-xl px-3 py-2 text-xs font-medium", best ? "bg-leaf-soft text-leaf-deep" : "bg-slate-50 text-slate-600")}>
            {best ? t("ov2.pl.verdict_yes", { from: regionName(best.o.region), ms: best.ms!, pct: nf(0).format(best.diff) }) : t("ov2.pl.verdict_no", { ms: limit })}
          </p>
          <ul className="space-y-3">
            {items.map(({ o, km, ms, lat, diff }) => (
              <li key={o.region}>
                <p className="mb-1.5 text-xs font-medium"><span className="font-normal text-muted">{t("ov2.pl.from")} </span>{regionName(o.region)}</p>
                {ms !== null && km !== null && (
                  <LatencyGauge ms={ms} km={km} limit={limit} basis={lat?.basis} errorPct={lat?.errorPct} title={`${regionName(o.region)} → ${regionName(row.region)}: ${ms} ms`}
                    right={<span className={cn("shrink-0 rounded-full px-2 py-0.5 font-semibold ring-1", Math.abs(diff) < 5 ? "bg-slate-50 text-slate-600 ring-line" : diff > 0 ? "bg-leaf-soft text-leaf-deep ring-leaf/20" : "bg-amber-50 text-[#8a6218] ring-amber-100")}>
                      {Math.abs(diff) < 5 ? t("ov2.reg.same_grid") : diff > 0 ? t("ov2.reg.cleaner_by", { pct: nf(0).format(diff) }) : t("ov2.reg.dirtier_by", { pct: nf(0).format(-diff) })}
                    </span>} />
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

/** What one selected region means for the organization: the grid now and over time, the footprint there, what moving would do. */
function Detail({ row, rows, cur, limit }: { row: Row; rows: Row[]; cur: string; limit: number }) {
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
        {row.g !== null && <p className="mt-1.5 text-[11px] text-muted">{t("ov2.reg.now_note")}</p>}
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

      <Placement row={row} rows={rows} limit={limit} />

      <div className="mt-auto space-y-2 text-sm">
        {alt ? null : row.fp && cleaner && cleaner.g! < g ? (
          <div className="rounded-xl bg-amber-50 px-3 py-3 text-xs leading-relaxed text-amber-900 ring-1 ring-amber-100">
            {t("ov2.reg.cleaner", { region: regionName(cleaner.region), g: nf(0).format(cleaner.g!), pct: nf(0).format((1 - cleaner.g! / g) * 100) })}{" "}
            {row.fp.relocation_blocked === "residency_unknown" && <AllowRegions className="text-xs">{t("ov2.opt.set_regions")}</AllowRegions>}
            {row.fp.relocation_blocked === "latency" && <span className="font-medium">{t("ov2.reg.blocked_latency")}.</span>}
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
  const [mode, setMode] = useState<MapMode>("intensity");
  const limit = (fp.data ?? []).map((f) => f.alternative?.latency_budget_ms).find((v) => v) ?? 60;

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

  const maxCarbon = Math.max(1, ...used.map((r) => r.fp!.carbon_kg_co2e));

  return (
    <Card index={11} interactive={false} className="p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink">
          <Glyph name="pin" className="size-[18px] text-[#2f8f78]" />{t("ov.reg.title")}<Info>{t("info.regions")}</Info>
        </h2>
        <div className="flex flex-wrap items-center gap-3">
          <Segmented size="sm" label={t("ov3.map.layer")} value={mode} onChange={setMode}
            options={[
              { value: "intensity", label: t("map.layer_intensity", undefined, "Grid intensity") },
              { value: "footprint", label: t("map.layer_footprint", undefined, "My footprint"), disabled: (fp.data ?? []).length === 0 },
              { value: "relocation", label: t("map.layer_relocation", undefined, "Relocation"), disabled: (fp.data ?? []).length === 0 },
            ]} />
          <Link href="/carbon" className="text-xs font-semibold text-primary hover:underline">{t("ov.svc.link")}</Link>
        </div>
      </div>
      {q.isLoading ? <Skeleton className="h-72" /> : q.isError ? <ErrorState /> : rows.length === 0 ? <EmptyState icon={Globe2} text={t("ov.reg.empty")} /> : (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
          <div className="flex min-w-0 flex-col gap-3">
            <RegionMap limitMs={limit} regions={q.data!} footprint={fp.data ?? []} selected={current?.region ?? null} onSelect={setSel} mode={mode} onModeChange={setMode} />
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="mr-1 flex items-center gap-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">{t("ov2.reg.yours")}<Info>{t("info.regions_yours")}</Info></span>
              {used.length === 0 ? <span className="text-xs text-muted">{t("ov.reg.no_resources")}</span> : used.slice(0, 6).map((r, i) => (
                <button key={r.region} type="button" onClick={() => setSel(r.region)} aria-pressed={current?.region === r.region} style={{ "--i": i } as CSSProperties}
                  className={cn("rise group relative inline-flex items-center gap-1.5 overflow-hidden rounded-full px-2.5 py-1 text-xs font-medium ring-1 transition-all duration-200",
                    current?.region === r.region ? "bg-leaf-soft/70 text-ink ring-leaf-deep/40" : "bg-white text-slate-700 ring-line hover:ring-slate-300")}>
                  <span className="size-2 shrink-0 rounded-full" style={{ background: r.g === null ? "#e4eaf3" : intensityColor(r.g) }} />
                  {regionName(r.region)}
                  <span className="tabular-nums text-muted">{formatCarbon(r.fp!.carbon_kg_co2e)}</span>
                  <span aria-hidden className="absolute inset-x-0 bottom-0 h-0.5 origin-left bg-leaf/70" style={{ transform: `scaleX(${Math.max(0.05, r.fp!.carbon_kg_co2e / maxCarbon)})` }} />
                </button>
              ))}
            </div>
            {ranked.length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted">
                <span className="mr-1 text-[11px] font-semibold uppercase tracking-wide text-slate-400">{t("ov2.reg.cleanest_now")}</span>
                {ranked.slice(0, 3).map((r) => (
                  <button key={r.region} type="button" onClick={() => setSel(r.region)} className="inline-flex items-center gap-1.5 rounded-full bg-slate-50 px-2.5 py-1 font-medium text-slate-700 ring-1 ring-line transition-colors hover:bg-leaf-soft">
                    <span className="size-2 rounded-full" style={{ background: intensityColor(r.g!) }} />{regionName(r.region)} · {nf(0).format(r.g!)}
                  </button>
                ))}
              </div>
            )}
          </div>
          <div className="min-w-0 rounded-2xl bg-slate-50/60 p-3.5 ring-1 ring-line/70">
            {current ? <Detail row={current} rows={rows} cur={cur} limit={limit} /> : null}
          </div>
        </div>
      )}
    </Card>
  );
}
