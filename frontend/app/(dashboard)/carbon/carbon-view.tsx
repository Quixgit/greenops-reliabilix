"use client";

import { IconLeaf as Leaf } from "@/components/ui/icons";
import { useState } from "react";
import { IntensityChart, SOURCE_LABEL } from "@/components/charts/intensity-chart";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/charts/sparkline";
import { SciCalculator } from "@/components/carbon/sci-calculator";
import { useCarbonSummary, useCarbonTrend, useIntensityHistory, useMethodologyVersions, useScheduleAdvice } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { periodRange } from "@/lib/store";
import { useUi } from "@/lib/store";
import { carbonParts, nf } from "@/lib/formatters";

function Figure({ label, value, unit, note }: { label: string; value: string; unit: string; note?: string }) {
  return (
    <div>
      <p className="text-sm font-medium text-slate-700">{label}</p>
      <p className="text-2xl font-bold tracking-tight text-ink">{value}<span className="ml-1 text-sm font-medium text-slate-600">{unit}</span></p>
      {note && <p className="mt-0.5 text-xs text-muted">{note}</p>}
    </div>
  );
}

const BASIS: Record<string, string> = {
  day_mean: "Mean of the day's readings", month_mean: "Monthly national average", partial_day_mean: "Mean of a few readings (lower confidence)",
  latest_before: "Last reading before the day", latest_overall: "Latest reading, an estimate", live_provider: "Fetched live",
};

function IntensityHistoryCard() {
  const t = useT();
  const [region, setRegion] = useState<string | undefined>();
  const q = useIntensityHistory(region);
  const { period } = useUi();
  const { from, to } = periodRange(period);
  const d = q.data;
  return (
    <Card index={2}>
      <CardHeader info={t("info.carbon_history")} title={t("carbon.intensity_history")} action={d && d.regions.length > 0 && (
        <select aria-label="Region" value={d.region} onChange={(e) => setRegion(e.target.value)} className="rounded-lg bg-white px-2.5 py-1.5 text-sm ring-1 ring-line">
          {d.regions.map((r) => <option key={r.region} value={r.region}>{r.region}</option>)}
        </select>)} />
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError ? <ErrorState /> : !d || d.regions.length === 0 ? (
        <EmptyState icon={Leaf} text="No regions with carbon yet. Intensity history appears for the regions your workloads run in." />
      ) : d.points.length === 0 ? (
        <EmptyState icon={Leaf} text={`No stored grid readings for ${d.region} in this period. Readings are collected hourly from the configured sources; days without a reading stay empty.`} />
      ) : (<>
        <IntensityChart points={d.points} from={from} to={to} />
        <p className="mt-2 text-xs text-muted">
          {t("carbon.days_with_data", { days: d.days_with_data, window: d.window_days })} One source is used per day, in this order: {d.policy.priority.map((p) => SOURCE_LABEL[p] ?? p).join(", ")} (at least {d.policy.min_day_readings} readings a day).
          Sources are shown side by side and never averaged together.
          {d.points.some((x) => x.source === "ember") && " Monthly national averages are from Ember (CC-BY 4.0)."}
        </p>
        {d.days_with_data < d.window_days / 2 && (
          <div className="mt-3 rounded-xl bg-sky-50 px-4 py-3 text-xs leading-relaxed text-sky-900 ring-1 ring-sky-100">
            <b>Why so few days?</b> Hourly readings are stored from the moment a source was connected; most plans do not return the past. The history
            of this region is filled automatically from sources that provide it (Ember monthly averages, UK Carbon Intensity for Great Britain, EIA for
            US balancing authorities) once the operator has connected them, and every day adds 24 more readings.
          </div>
        )}
      </>)}
    </Card>
  );
}

/** When to run a flexible job (a pipeline, a backup, a batch) so that it uses the cleanest grid hours, from the forecast. */
function ScheduleCard() {
  const t = useT();
  const regions = useIntensityHistory(undefined).data?.regions ?? [];
  const [region, setRegion] = useState<string | undefined>();
  const [duration, setDuration] = useState(2);
  const [within, setWithin] = useState(24);
  const chosen = region ?? regions[0]?.region;
  const q = useScheduleAdvice(chosen, duration, within);
  const hhmm = (iso: string) => new Date(iso).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", day: "numeric", month: "short" });
  const sel = "rounded-lg bg-white px-2.5 py-1.5 text-sm ring-1 ring-line";
  return (
    <Card index={3}>
      <CardHeader info={t("info.carbon_schedule")} title={t("schedule.title")} />
      <p className="mb-3 text-sm text-muted">{t("schedule.hint")}</p>
      {regions.length === 0 ? <p className="text-sm text-muted">{t("schedule.no_regions")}</p> : (<>
        <div className="flex flex-wrap items-center gap-3 text-sm">
          <label className="flex items-center gap-2">{t("schedule.region")}
            <select value={chosen} onChange={(e) => setRegion(e.target.value)} className={sel}>{regions.map((r) => <option key={r.region} value={r.region}>{r.region}</option>)}</select></label>
          <label className="flex items-center gap-2">{t("schedule.duration")}
            <select value={duration} onChange={(e) => setDuration(Number(e.target.value))} className={sel}>{[1, 2, 4, 8, 12].map((h) => <option key={h} value={h}>{h} h</option>)}</select></label>
          <label className="flex items-center gap-2">{t("schedule.within")}
            <select value={within} onChange={(e) => setWithin(Number(e.target.value))} className={sel}>{[12, 24, 48].filter((h) => h >= duration).map((h) => <option key={h} value={h}>{h} h</option>)}</select></label>
        </div>
        <div className="mt-4">
          {q.isLoading ? <Skeleton className="h-12" /> : q.isError || !q.data ? <p className="text-sm text-muted">{t("schedule.none")}</p> : (
            <div className="rounded-2xl bg-leaf-soft px-4 py-3 text-sm ring-1 ring-leaf-soft">
              <p className="font-semibold text-leaf-deep">{t("schedule.best", { start: hhmm(q.data.window.start) })}</p>
              <p className="mt-1 text-leaf-deep/80">
                {t("schedule.avg", { best: Math.round(q.data.window.average_g_per_kwh), now: Math.round(q.data.window.now_average_g_per_kwh) })}
                {q.data.window.saving_pct >= 1 && <> · <b>{t("schedule.saving", { pct: Math.round(q.data.window.saving_pct) })}</b></>}
              </p>
              <p className="mt-1 text-xs text-leaf-deep/70">{t("schedule.basis", { source: SOURCE_LABEL[q.data.source] ?? q.data.source })}</p>
            </div>
          )}
        </div>
      </>)}
    </Card>
  );
}

function IntensityOriginCard({ origins, measured }: { origins: { source: string; basis: string; share_pct: number }[]; measured: number | null }) {
  const t = useT();
  if (origins.length === 0) return null;
  return (
    <Card index={4}>
      <CardHeader info={t("info.carbon_origin")} title={t("carbon.intensity_origin")} />
      <p className="mb-3 text-sm text-muted">
        {measured === null ? "" : <><b className="text-ink">{measured.toFixed(0)}%</b> of the carbon in this period uses the mean of the day’s own readings. </>}
        The rest uses a fewer-readings mean or the latest reading, which is less precise.
      </p>
      <ul className="space-y-2 text-sm">
        {origins.map((o) => (
          <li key={o.source + o.basis} className="flex items-center gap-3">
            <span className="w-52 shrink-0 truncate">{o.source ? (SOURCE_LABEL[o.source] ?? o.source) : "Before origin was recorded"}</span>
            <span className="h-2 flex-1 overflow-hidden rounded-full bg-slate-100"><span className="block h-full rounded-full bg-leaf" style={{ width: `${Math.max(2, o.share_pct)}%` }} /></span>
            <span className="w-14 shrink-0 text-right tabular-nums">{o.share_pct.toFixed(0)}%</span>
            <span className="hidden w-64 shrink-0 text-xs text-muted md:block">{o.basis ? BASIS[o.basis] ?? o.basis : "unknown basis"}</span>
          </li>
        ))}
      </ul>
    </Card>
  );
}

export function CarbonView() {
  const t = useT();
  const sum = useCarbonSummary();
  const trend = useCarbonTrend();
  const m = sum.data?.methodology;
  const versions = useMethodologyVersions();
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">{t("carbon.title")}</h1>
      <Card index={0}>
        <CardHeader info={t("info.carbon_totals")} title={t("carbon.period_totals")} />
        {sum.isLoading ? <Skeleton className="h-20" /> : sum.isError ? <ErrorState /> : !sum.data!.has_data ? (
          <EmptyState icon={Leaf} text="No carbon data yet. Connect a cloud account and run a sync." action={{ href: "/connectors", label: "Open connectors" }} />
        ) : (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
            <Figure label={t("carbon.energy")} value={nf(1).format(sum.data!.energy_kwh)} unit="kWh" note="estimated from usage, not metered" />
            <Figure label="CO₂e" value={nf(1).format(carbonParts(sum.data!.carbon_kg_co2e).n)} unit={carbonParts(sum.data!.carbon_kg_co2e).unit === "t" ? "t" : "kg"} note="operational, location-based" />
            <Figure label={t("carbon.grid_intensity")} value={nf(0).format(sum.data!.carbon_intensity_g_per_kwh)} unit="gCO₂e/kWh" />
            <Figure label="SCI" value={sum.data!.sci_score === null ? "—" : nf(2).format(sum.data!.sci_score)} unit="per functional unit"
              note={sum.data!.sci_score === null ? "needs functional units reported for the project" : undefined} />
          </div>
        )}
      </Card>
      <Card index={1}>
        <CardHeader info={t("info.carbon_daily")} title={t("carbon.daily_co2e")} />
        {trend.isLoading ? <Skeleton className="h-10" /> : trend.isError ? <ErrorState /> : trend.data!.length < 2 ?
          <p className="text-sm text-muted">Not enough days yet for a trend.</p> :
          <Sparkline values={trend.data!.map((p) => p.carbon_kg_co2e)} width={420} height={56} />}
      </Card>
      {sum.data?.has_data && <SciCalculator summary={sum.data} />}
      <IntensityHistoryCard />
      <ScheduleCard />
      {sum.data && <IntensityOriginCard origins={sum.data.intensity_origins} measured={sum.data.intensity_measured_share_pct} />}
      <Card index={4}>
        <CardHeader info={t("info.carbon_method")} title={t("carbon.methodology")} action={m && <Badge tone={m.status === "verified" ? "good" : "warn"}>{m.status}</Badge>} />
        {!m ? <Skeleton className="h-24" /> : (
          <div className="space-y-3 text-sm">
            <p><b>{m.version}</b> — {m.provenance}</p>
            <p className="text-muted">Embodied carbon: <b>{m.embodied_carbon_included ? "included" : "not included"}</b>.</p>
            <ul className="list-disc space-y-1 pl-5 text-muted">{m.caveats.map((c) => <li key={c}>{c}</li>)}</ul>
            {versions.data && versions.data.length > 0 && (
              <p className="text-xs text-muted">
                Known versions: {versions.data.map((v) => `${v.version}${v.active ? " (active)" : ""}`).join(", ")}. A published version is never edited.
              </p>
            )}
            <p className="text-xs text-muted">Figures are estimates produced with the methodology version above; a new version never rewrites old results.</p>
          </div>
        )}
      </Card>
    </div>
  );
}
