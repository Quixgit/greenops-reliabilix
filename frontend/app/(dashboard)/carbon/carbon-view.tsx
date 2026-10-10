"use client";

import { Leaf } from "lucide-react";
import { useState } from "react";
import { IntensityChart, SOURCE_LABEL } from "@/components/charts/intensity-chart";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/charts/sparkline";
import { useCarbonSummary, useCarbonTrend, useIntensityHistory, useMethodologyVersions } from "@/hooks/use-overview";
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
  day_mean: "Mean of the day's readings", partial_day_mean: "Mean of a few readings (lower confidence)",
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
      <CardHeader title={t("carbon.intensity_history")} action={d && d.regions.length > 0 && (
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
        </p>
      </>)}
    </Card>
  );
}

function IntensityOriginCard({ origins, measured }: { origins: { source: string; basis: string; share_pct: number }[]; measured: number | null }) {
  const t = useT();
  if (origins.length === 0) return null;
  return (
    <Card index={3}>
      <CardHeader title={t("carbon.intensity_origin")} />
      <p className="mb-3 text-sm text-muted">
        {measured === null ? "" : <><b className="text-ink">{measured.toFixed(0)}%</b> of the carbon in this period uses the mean of the day’s own readings. </>}
        The rest uses a fewer-readings mean or the latest reading, which is less precise.
      </p>
      <ul className="space-y-2 text-sm">
        {origins.map((o) => (
          <li key={o.source + o.basis} className="flex items-center gap-3">
            <span className="w-52 shrink-0 truncate">{o.source ? (SOURCE_LABEL[o.source] ?? o.source) : "Before origin was recorded"}</span>
            <span className="h-2 flex-1 overflow-hidden rounded-full bg-slate-100"><span className="block h-full rounded-full bg-emerald-500" style={{ width: `${Math.max(2, o.share_pct)}%` }} /></span>
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
        <CardHeader title={t("carbon.period_totals")} />
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
        <CardHeader title={t("carbon.daily_co2e")} />
        {trend.isLoading ? <Skeleton className="h-10" /> : trend.isError ? <ErrorState /> : trend.data!.length < 2 ?
          <p className="text-sm text-muted">Not enough days yet for a trend.</p> :
          <Sparkline values={trend.data!.map((p) => p.carbon_kg_co2e)} width={420} height={56} />}
      </Card>
      <IntensityHistoryCard />
      {sum.data && <IntensityOriginCard origins={sum.data.intensity_origins} measured={sum.data.intensity_measured_share_pct} />}
      <Card index={4}>
        <CardHeader title={t("carbon.methodology")} action={m && <Badge tone={m.status === "verified" ? "good" : "warn"}>{m.status}</Badge>} />
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
