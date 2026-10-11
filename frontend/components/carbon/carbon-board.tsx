"use client";

import { SelectField } from "@/components/ui/select-field";
import Link from "next/link";
import { useMemo, useRef, useState, type CSSProperties } from "react";
import { IntensityChart, SOURCE_LABEL } from "@/components/charts/intensity-chart";
import { Kpi } from "@/components/dashboard/kpi-strip";
import { Card, CardHeader } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { DeltaBadge } from "@/components/ui/delta-badge";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconBolt, IconClock, IconGauge, IconGlobe, IconLeaf, IconTrend } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useCarbonSummary, useCarbonTrend, useIntensityHistory, useProviders, useRegionFootprint, useScheduleAdvice, useServices } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { carbonParts, formatCost, nf, pctChange } from "@/lib/formatters";
import { movingAverage } from "@/lib/overview";
import { regionName } from "@/lib/regions";
import { periodRange, useUi } from "@/lib/store";
import { cn } from "@/lib/utils";

const PROVIDER: Record<string, string> = { aws: "AWS", azure: "Azure", gcp: "Google Cloud", kubernetes: "Kubernetes" };

/** The grid bands used across the platform: under 150 clean, under 300 moderate, under 450 high, above very high. */
export function band(g: number): { key: "clean" | "moderate" | "high" | "very_high"; chip: string; dot: string } {
  if (g < 150) return { key: "clean", chip: "bg-leaf-soft text-leaf-deep", dot: "bg-leaf" };
  if (g < 300) return { key: "moderate", chip: "bg-[#fdf3dc] text-[#a9741a]", dot: "bg-pastel-amber" };
  if (g < 450) return { key: "high", chip: "bg-[#fdecef] text-[#c25667]", dot: "bg-pastel-rose" };
  return { key: "very_high", chip: "bg-[#f1ecfc] text-[#7a68c8]", dot: "bg-pastel-violet" };
}

/** Four numbers: what the period emitted, the energy behind it, how dirty the grid was, and the footprint per unit of work. */
export function CarbonKpis() {
  const t = useT();
  const q = useCarbonSummary();
  const trend = useCarbonTrend();
  const regions = useRegionFootprint();
  const c = q.data;
  const has = !!c?.has_data;
  const parts = has ? carbonParts(c!.carbon_kg_co2e) : null;
  const days = Math.max(1, trend.data?.length ?? 1);
  const perDay = has ? c!.carbon_kg_co2e / days : 0;
  const series = (trend.data ?? []).map((p) => p.carbon_kg_co2e);
  const intensity = has && c!.carbon_intensity_g_per_kwh > 0 ? c!.carbon_intensity_g_per_kwh : null;
  const b = intensity === null ? null : band(intensity);
  const empty = <Link href="/connectors" className="font-semibold text-primary hover:underline">{t("ov2.empty.connect")}</Link>;
  const cleanest = [...(regions.data ?? [])].filter((r) => r.g_per_kwh !== null).sort((a, z) => a.g_per_kwh! - z.g_per_kwh!)[0];

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Kpi index={0} icon={IconLeaf} tone="text-leaf-deep" label={t("cb.kpi.emissions")} info={t("cb.info.emissions")} loading={q.isLoading || trend.isLoading}
        value={parts ? <CountUp value={parts.n} format={(n) => nf(1).format(n)} /> : null} unit={parts ? `${parts.unit} CO₂e` : undefined}
        delta={has && c!.comparable ? <DeltaBadge pct={pctChange(c!.carbon_kg_co2e, c!.previous.carbon_kg_co2e)} /> : null}
        spark={series} sparkColor="#6fc3a8" empty={empty}
        foot={<><span className="font-semibold text-slate-800">{nf(perDay < 10 ? 2 : 1).format(perDay)} kg</span> {t("ov2.per_day")}<span className="block text-muted">{t("ov2.operational_only")}</span></>} />
      <Kpi index={1} icon={IconBolt} tone="text-[#d9a441]" label={t("carbon.energy")} info={t("cb.info.energy")} loading={q.isLoading || trend.isLoading}
        value={has ? <CountUp value={c!.energy_kwh} format={(n) => nf(c!.energy_kwh < 100 ? 1 : 0).format(n)} /> : null} unit="kWh"
        delta={has && c!.comparable ? <DeltaBadge pct={pctChange(c!.energy_kwh, c!.previous.energy_kwh)} /> : null} empty={empty}
        foot={<><span>{(c?.energy_cost_based_share_pct ?? 0) >= 50 ? t("cb.kpi.energy_cost_based", { pct: Math.round(c!.energy_cost_based_share_pct!) }) : t("cb.kpi.energy_note")}</span></>} />
      <Kpi index={2} icon={IconGlobe} tone="text-[#5b8fdc]" label={t("carbon.grid_intensity")} info={t("cb.info.intensity")} loading={q.isLoading || trend.isLoading}
        value={intensity !== null ? <CountUp value={intensity} format={(n) => nf(0).format(n)} /> : null} unit="gCO₂e/kWh" empty={empty}
        delta={b ? <span className="text-xs font-semibold text-muted">{t(`ov2.band.${b.key}`)}</span> : null}
        foot={<>
          {intensity !== null && (
            <span className="mb-1.5 block h-1.5 overflow-hidden rounded-full bg-gradient-to-r from-leaf via-pastel-amber to-pastel-rose">
              <span className="block h-full border-r-2 border-slate-800/70" style={{ width: `${Math.min(100, (intensity / 600) * 100)}%` } as CSSProperties} />
            </span>
          )}
          {cleanest ? t("cb.kpi.cleanest", { region: regionName(cleanest.region), g: nf(0).format(cleanest.g_per_kwh!) }) : t("ov2.energy_weighted")}
        </>} />
      <Kpi index={3} icon={IconGauge} tone="text-[#8a78d8]" label="SCI" info={t("cb.info.sci")} loading={q.isLoading || trend.isLoading}
        value={has && c!.sci_score !== null ? <CountUp value={c!.sci_score} format={(n) => nf(2).format(n)} /> : null} unit={t("cb.kpi.sci_unit")}
        empty={<span>{t("cb.kpi.sci_empty")} <Link href="/projects" className="font-semibold text-primary hover:underline">{t("cb.kpi.sci_link")}</Link></span>}
        foot={<>{t("cb.kpi.sci_note")}</>} />
    </div>
  );
}

const W = 640, H = 260, P = { l: 48, r: 14, t: 14, b: 26 };
function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = Math.pow(10, Math.floor(Math.log10(v))), n = v / p;
  return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 2.5 ? 2.5 : n <= 5 ? 5 : 10) * p;
}
const dayLabel = (d: string) => new Date(`${d.slice(0, 10)}T00:00:00Z`).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" });

/** Daily CO₂e as a soft area with a crosshair, the 7-day average as a line, and the peak and the quietest day called out. */
export function CarbonTrend() {
  const t = useT();
  const q = useCarbonTrend();
  const [avg, setAvg] = useState(true);
  const [hover, setHover] = useState<number | null>(null);
  const box = useRef<HTMLDivElement>(null);
  const pts = q.data ?? [];
  const vals = pts.map((p) => p.carbon_kg_co2e);
  const smooth = useMemo(() => movingAverage(vals), [vals]);
  const n = pts.length;
  const max = niceMax(Math.max(0, ...vals));
  const x = (i: number) => P.l + (n <= 1 ? 0 : (i / (n - 1)) * (W - P.l - P.r));
  const y = (v: number) => H - P.b - (v / max) * (H - P.t - P.b);
  const path = (a: number[]) => a.map((v, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const total = vals.reduce((s, v) => s + v, 0);
  const peak = n ? vals.indexOf(Math.max(...vals)) : -1;
  const low = n ? vals.indexOf(Math.min(...vals)) : -1;
  const xs = n > 1 ? Array.from({ length: Math.min(6, n) }, (_, k) => Math.round((k / (Math.min(6, n) - 1)) * (n - 1))) : [0];
  const onMove = (e: React.MouseEvent) => {
    const r = box.current?.getBoundingClientRect();
    if (!r || n < 2) return;
    const px = ((e.clientX - r.left) / r.width) * W;
    setHover(Math.max(0, Math.min(n - 1, Math.round(((px - P.l) / (W - P.l - P.r)) * (n - 1)))));
  };
  const fmt = (v: number) => `${nf(v < 10 ? 2 : 1).format(v)} kg`;

  return (
    <Card index={4} interactive={false} className="min-w-0">
      <CardHeader icon={<IconTrend className="size-[18px] text-leaf-deep" />} title={t("carbon.daily_co2e")} info={t("info.carbon_daily")}
        action={
          <label className="flex cursor-pointer items-center gap-2 text-xs font-medium text-slate-600">
            <input type="checkbox" className="peer sr-only" checked={avg} onChange={(e) => setAvg(e.target.checked)} />
            <span className="relative h-5 w-9 rounded-full bg-slate-200 transition-colors peer-checked:bg-leaf-deep/75 after:absolute after:left-0.5 after:top-0.5 after:size-4 after:rounded-full after:bg-white after:shadow after:transition-transform peer-checked:after:translate-x-4" />
            {t("ov2.trend.avg7")}
          </label>
        } />
      {q.isLoading ? <Skeleton className="h-[260px]" /> : q.isError ? <ErrorState /> : n < 2 ? <EmptyState icon={IconTrend} text={t("cb.trend.empty")} /> : (
        <>
          <div ref={box} className="relative" onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
            <svg viewBox={`0 0 ${W} ${H}`} className="block h-auto w-full" role="img" aria-label={t("carbon.daily_co2e")}>
              <defs>
                <linearGradient id="cb-area" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#6fc3a8" stopOpacity=".38" /><stop offset="1" stopColor="#6fc3a8" stopOpacity=".02" /></linearGradient>
              </defs>
              {[0, 0.25, 0.5, 0.75, 1].map((k) => (
                <g key={k}>
                  <line x1={P.l} x2={W - P.r} y1={y(max * k)} y2={y(max * k)} stroke="#e8edf5" strokeDasharray={k === 0 ? undefined : "3 5"} />
                  <text x={P.l - 8} y={y(max * k) + 4} textAnchor="end" fontSize="11" fill="#7b8794">{nf(max < 5 ? 1 : 0).format(max * k)}</text>
                </g>
              ))}
              {xs.map((i) => <text key={i} x={x(i)} y={H - 6} textAnchor={i === 0 ? "start" : i === n - 1 ? "end" : "middle"} fontSize="11" fill="#7b8794">{dayLabel(pts[i].day)}</text>)}
              <path d={`${path(vals)} L${x(n - 1)},${y(0)} L${x(0)},${y(0)} Z`} fill="url(#cb-area)" className="fade-in" />
              <path d={path(vals)} fill="none" stroke="#6fc3a8" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" pathLength={100} className="draw" style={{ ["--len" as string]: 100 }} />
              {avg && <path d={path(smooth)} fill="none" stroke="#2f8f78" strokeWidth="1.6" strokeDasharray="5 5" strokeLinecap="round" />}
              {peak >= 0 && <g><circle cx={x(peak)} cy={y(vals[peak])} r="5" fill="#f2cf8a" stroke="#fff" strokeWidth="2" /></g>}
              {hover !== null && (
                <g>
                  <line x1={x(hover)} x2={x(hover)} y1={P.t} y2={H - P.b} stroke="#2f8f78" strokeOpacity=".35" />
                  <circle cx={x(hover)} cy={y(vals[hover])} r="5" fill="#fff" stroke="#2f8f78" strokeWidth="2.4" />
                </g>
              )}
            </svg>
            {hover !== null && (
              <div className="pointer-events-none absolute top-2 z-10 rounded-xl border border-line bg-white px-3 py-2 text-xs shadow-[0_10px_30px_rgba(31,52,110,0.16)]"
                style={{ left: `${(x(hover) / W) * 100}%`, transform: `translateX(${hover > n * 0.65 ? "-110%" : "10%"})` }}>
                <p className="font-semibold text-ink">{dayLabel(pts[hover].day)}</p>
                <p className="mt-0.5 text-slate-600">{fmt(vals[hover])} CO₂e</p>
                {avg && <p className="text-muted">{t("cb.trend.avg")}: {fmt(smooth[hover])}</p>}
              </div>
            )}
          </div>
          <dl className="mt-4 grid grid-cols-3 gap-3 border-t border-line/70 pt-3 text-sm">
            <div><dt className="text-xs text-muted">{t("cb.trend.average")}</dt><dd className="font-semibold tabular-nums">{fmt(total / n)}</dd></div>
            <div><dt className="flex items-center gap-1.5 text-xs text-muted"><span className="size-2 rounded-full bg-pastel-amber" />{t("cb.trend.peak")}</dt><dd className="font-semibold tabular-nums">{fmt(vals[peak])} <span className="font-normal text-muted">· {dayLabel(pts[peak].day)}</span></dd></div>
            <div><dt className="text-xs text-muted">{t("cb.trend.lowest")}</dt><dd className="font-semibold tabular-nums">{fmt(vals[low])} <span className="font-normal text-muted">· {dayLabel(pts[low].day)}</span></dd></div>
          </dl>
        </>
      )}
    </Card>
  );
}

type Row = { key: string; name: string; sub?: string; kg: number; g?: number | null };

/** Where the footprint comes from: by cloud, by region (with the grid it sits on) or by service. */
export function CarbonBreakdown() {
  const t = useT();
  const [by, setBy] = useState<"cloud" | "region" | "service">("cloud");
  const [all, setAll] = useState(false);
  const providers = useProviders();
  const regions = useRegionFootprint();
  const services = useServices("all");
  const q = by === "cloud" ? providers : by === "region" ? regions : services;
  const rows: Row[] = by === "cloud"
    ? (providers.data ?? []).map((p) => ({ key: p.provider, name: PROVIDER[p.provider] ?? p.provider, kg: p.carbon_kg_co2e }))
    : by === "region"
      ? (regions.data ?? []).map((r) => ({ key: r.provider + r.region, name: regionName(r.region), sub: PROVIDER[r.provider] ?? r.provider, kg: r.carbon_kg_co2e, g: r.g_per_kwh }))
      : (services.data ?? []).map((s) => ({ key: s.service, name: s.service, sub: s.category, kg: s.carbon_kg_co2e }));
  rows.sort((a, z) => z.kg - a.kg);
  const total = rows.reduce((s, r) => s + r.kg, 0);
  const shown = all ? rows : rows.slice(0, 7);
  const TABS = ["cloud", "region", "service"] as const;
  return (
    <Card index={5} interactive={false} className="flex min-w-0 flex-col">
      <CardHeader icon={<IconGlobe className="size-[18px] text-leaf-deep" />} title={t("cb.break.title")} info={t("cb.info.breakdown")} />
      <div role="tablist" className="mb-4 flex gap-1 rounded-xl bg-slate-100 p-1 text-sm">
        {TABS.map((k) => (
          <button key={k} role="tab" type="button" aria-selected={by === k} onClick={() => { setBy(k); setAll(false); }}
            className={cn("flex-1 rounded-lg px-3 py-1.5 font-medium transition-all", by === k ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{t(`cb.break.${k}`)}</button>
        ))}
      </div>
      {q.isLoading ? <Skeleton className="h-52" /> : q.isError ? <ErrorState /> : rows.length === 0 ? <EmptyState icon={IconGlobe} text={t("cb.break.empty")} /> : (
        <ul key={by} className="space-y-3.5">
          {shown.map((r, i) => {
            const share = total > 0 ? (r.kg / total) * 100 : 0;
            const b = r.g ? band(r.g) : null;
            return (
              <li key={r.key} className="form-swap" style={{ animationDelay: `${i * 40}ms` }}>
                <div className="flex items-baseline justify-between gap-3 text-sm">
                  <span className="min-w-0 truncate font-medium text-ink">{r.name}{r.sub && <span className="ml-2 text-xs font-normal text-muted">{r.sub}</span>}</span>
                  <span className="shrink-0 tabular-nums"><b>{nf(r.kg < 10 ? 2 : 1).format(r.kg)}</b> <span className="text-xs text-muted">kg · {nf(share < 10 ? 1 : 0).format(share)}%</span></span>
                </div>
                <div className="mt-1.5 flex items-center gap-2">
                  <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-leaf" style={{ width: `${Math.max(2, share)}%`, "--d": `${i * 60}ms` } as CSSProperties} /></span>
                  {b && <span className={cn("shrink-0 rounded-full px-2 py-0.5 text-[11px] font-semibold tabular-nums", b.chip)}>{nf(0).format(r.g!)} g/kWh</span>}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {rows.length > 7 && <button type="button" onClick={() => setAll((v) => !v)} className="mt-4 self-start text-sm font-medium text-leaf-deep hover:underline">{all ? t("cb.break.less") : t("cb.break.more", { count: rows.length - 7 })}</button>}
    </Card>
  );
}

/** Grid intensity of one region over time, with the regions as chips and how complete the history is. */
export function GridHistory() {
  const t = useT();
  const [region, setRegion] = useState<string | undefined>();
  const q = useIntensityHistory(region);
  const { period } = useUi();
  const { from, to } = periodRange(period);
  const d = q.data;
  const cover = d && d.window_days > 0 ? Math.round((d.days_with_data / d.window_days) * 100) : 0;
  return (
    <Card index={6} interactive={false}>
      <CardHeader icon={<IconGlobe className="size-[18px] text-[#5b8fdc]" />} title={t("carbon.intensity_history")} info={t("info.carbon_history")}
        action={d && d.points.length > 0 && <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-600">{t("cb.hist.coverage", { pct: cover })}</span>} />
      {q.isLoading ? <Skeleton className="h-72" /> : q.isError ? <ErrorState /> : !d || d.regions.length === 0 ? (
        <EmptyState icon={IconGlobe} text={t("cb.hist.no_regions")} />
      ) : (
        <>
          <div className="mb-3 flex flex-wrap gap-2">
            {d.regions.map((r) => (
              <button key={r.region} type="button" onClick={() => setRegion(r.region)} aria-pressed={d.region === r.region}
                className={cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", d.region === r.region ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep")}>{r.region}</button>
            ))}
          </div>
          {d.points.length === 0 ? <EmptyState icon={IconGlobe} text={t("cb.hist.empty", { region: d.region })} /> : (
            <>
              <IntensityChart points={d.points} from={from} to={to} />
              <p className="mt-2 text-xs leading-relaxed text-muted">
                {t("carbon.days_with_data", { days: d.days_with_data, window: d.window_days })} {t("cb.hist.sources", { list: d.policy.priority.map((p) => SOURCE_LABEL[p] ?? p).join(", "), min: d.policy.min_day_readings })}
                {d.points.some((x) => x.source === "ember") && ` ${t("cb.hist.ember")}`}
              </p>
              {d.days_with_data < d.window_days / 2 && (
                <div className="mt-3 rounded-2xl bg-[#eef3fd] px-4 py-3 text-xs leading-relaxed text-[#3d5a8c]">
                  <b>{t("cb.hist.few_title")}</b> {t("cb.hist.few_body")}
                </div>
              )}
            </>
          )}
        </>
      )}
    </Card>
  );
}

/** The best time to start a flexible job, from the forecast: pick region and length, read a start time and the saving. */
export function ScheduleAdvisor() {
  const t = useT();
  const regions = useIntensityHistory(undefined).data?.regions ?? [];
  const [region, setRegion] = useState<string | undefined>();
  const [duration, setDuration] = useState(2);
  const [within, setWithin] = useState(24);
  const chosen = region ?? regions[0]?.region;
  const q = useScheduleAdvice(chosen, duration, within);
  const when = (iso: string) => new Date(iso).toLocaleString(undefined, { weekday: "short", hour: "2-digit", minute: "2-digit" });
  const w = q.data?.window;
  const seg = (opts: number[], v: number, set: (n: number) => void, suffix: string) => (
    <div className="flex gap-1 rounded-xl bg-slate-100 p-1 text-sm">
      {opts.map((o) => (
        <button key={o} type="button" onClick={() => set(o)} aria-pressed={v === o}
          className={cn("flex-1 rounded-lg px-2.5 py-1.5 font-medium tabular-nums transition-all", v === o ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{o}{suffix}</button>
      ))}
    </div>
  );
  return (
    <Card index={7} interactive={false} className="flex min-w-0 flex-col">
      <CardHeader icon={<IconClock className="size-[18px] text-[#d9a441]" />} title={t("schedule.title")} info={t("info.carbon_schedule")} />
      <p className="mb-4 text-sm leading-relaxed text-muted">{t("schedule.hint")}</p>
      {regions.length === 0 ? <p className="text-sm text-muted">{t("schedule.no_regions")}</p> : (
        <div className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="block text-xs font-medium text-slate-500">{t("schedule.region")}
              <SelectField value={chosen} onChange={(e) => setRegion(e.target.value)} className="mt-1 w-full rounded-xl border border-line bg-white px-3 py-2 text-sm font-medium text-ink">
                {regions.map((r) => <option key={r.region} value={r.region}>{r.region}</option>)}
              </SelectField></label>
            <div className="text-xs font-medium text-slate-500">{t("schedule.duration")}<div className="mt-1">{seg([1, 2, 4, 8], duration, (n) => { setDuration(n); if (within < n) setWithin(n <= 12 ? 12 : 24); }, " h")}</div></div>
            <div className="text-xs font-medium text-slate-500">{t("schedule.within")}<div className="mt-1">{seg([12, 24, 48].filter((h) => h >= duration), within, setWithin, " h")}</div></div>
          </div>
          {q.isLoading ? <Skeleton className="h-36" /> : q.isError || !w ? (
            <p className="rounded-2xl bg-slate-50 px-4 py-6 text-center text-sm text-muted">{t("schedule.none")}</p>
          ) : (
            <div key={`${chosen}${duration}${within}`} className="form-swap rounded-2xl bg-leaf-soft/70 p-4">
              <div className="flex flex-wrap items-center justify-between gap-4">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-leaf-deep/80">{t("cb.sched.best")}</p>
                  <p className="mt-1 text-2xl font-bold tracking-tight text-ink">{when(w.start)}</p>
                  <p className="text-sm text-slate-600">{t("cb.sched.until", { end: when(w.end) })}</p>
                </div>
                {w.saving_pct >= 1 && (
                  <div className="text-right">
                    <p className="text-3xl font-bold tracking-tight text-leaf-deep">−{Math.round(w.saving_pct)}%</p>
                    <p className="text-xs text-leaf-deep/80">{t("cb.sched.cleaner")}</p>
                  </div>
                )}
              </div>
              <div className="mt-4 space-y-2 text-xs">
                {[{ label: t("cb.sched.now"), g: w.now_average_g_per_kwh, c: "bg-pastel-rose" }, { label: t("cb.sched.best_window"), g: w.average_g_per_kwh, c: "bg-leaf" }].map((r) => {
                  const m = Math.max(w.now_average_g_per_kwh, w.average_g_per_kwh, 1);
                  return (
                    <div key={r.label} className="flex items-center gap-3">
                      <span className="w-24 shrink-0 text-slate-600">{r.label}</span>
                      <span className="h-2 flex-1 overflow-hidden rounded-full bg-white/80"><span className={cn("bar-x block h-full rounded-full", r.c)} style={{ width: `${(r.g / m) * 100}%` }} /></span>
                      <span className="w-20 shrink-0 text-right font-semibold tabular-nums text-ink">{nf(0).format(r.g)} g</span>
                    </div>
                  );
                })}
              </div>
              <p className="mt-3 text-[11px] text-leaf-deep/70">{t("schedule.basis", { source: SOURCE_LABEL[q.data!.source] ?? q.data!.source })}</p>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

const BASIS: Record<string, string> = {
  day_mean: "basis.day_mean", month_mean: "basis.month_mean", partial_day_mean: "basis.partial", latest_before: "basis.latest_before", latest_overall: "basis.latest", live_provider: "basis.live",
};
const SEG = ["bg-leaf", "bg-pastel-blue", "bg-pastel-amber", "bg-pastel-violet", "bg-pastel-rose"];

/** How trustworthy the intensity behind the CO₂e is: the share measured on the day itself against fallbacks. */
export function IntensityOrigin() {
  const t = useT();
  const q = useCarbonSummary();
  const o = q.data?.intensity_origins ?? [];
  const measured = q.data?.intensity_measured_share_pct ?? null;
  if (!q.data?.has_data || o.length === 0) return null;
  const r = 34, C = 2 * Math.PI * r;
  return (
    <Card index={8} interactive={false}>
      <CardHeader icon={<IconLeaf className="size-[18px] text-leaf-deep" />} title={t("carbon.intensity_origin")} info={t("info.carbon_origin")} />
      <div className="flex flex-col gap-6 sm:flex-row sm:items-center">
        <div className="relative mx-auto size-28 shrink-0">
          <svg viewBox="0 0 80 80" className="size-full -rotate-90">
            <circle cx="40" cy="40" r={r} fill="none" stroke="#eef2f7" strokeWidth="8" />
            {measured !== null && <circle cx="40" cy="40" r={r} fill="none" stroke="#6fc3a8" strokeWidth="8" strokeLinecap="round" strokeDasharray={`${(measured / 100) * C} ${C}`} className="transition-all duration-1000" />}
          </svg>
          <div className="absolute inset-0 grid place-items-center text-center">
            <span><span className="block text-xl font-bold tabular-nums">{measured === null ? "—" : `${nf(0).format(measured)}%`}</span><span className="block text-[10px] text-muted">{t("cb.origin.measured")}</span></span>
          </div>
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-sm leading-relaxed text-slate-600">{t("cb.origin.body")}</p>
          <div className="mt-3 flex h-2.5 overflow-hidden rounded-full bg-slate-100">
            {o.map((x, i) => <span key={x.source + x.basis} className={cn("bar-x h-full", SEG[i % SEG.length])} style={{ width: `${x.share_pct}%` }} title={`${x.share_pct.toFixed(0)}%`} />)}
          </div>
          <ul className="mt-3 grid gap-x-6 gap-y-1.5 text-sm sm:grid-cols-2">
            {o.map((x, i) => (
              <li key={x.source + x.basis} className="flex items-center gap-2">
                <span className={cn("size-2.5 shrink-0 rounded-full", SEG[i % SEG.length])} />
                <span className="min-w-0 flex-1 truncate">{x.source ? (SOURCE_LABEL[x.source] ?? x.source) : t("cb.origin.unknown")}{x.basis && <span className="text-muted"> · {t(`cb.${BASIS[x.basis] ?? "basis.unknown"}`)}</span>}</span>
                <span className="tabular-nums font-semibold">{x.share_pct.toFixed(0)}%</span>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Card>
  );
}
