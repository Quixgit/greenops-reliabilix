"use client";

import { Leaf } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Sparkline } from "@/components/charts/sparkline";
import { useCarbonSummary, useCarbonTrend } from "@/hooks/use-overview";
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

export function CarbonView() {
  const sum = useCarbonSummary();
  const trend = useCarbonTrend();
  const m = sum.data?.methodology;
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Carbon &amp; Energy</h1>
      <Card index={0}>
        <CardHeader title="Period totals" />
        {sum.isLoading ? <Skeleton className="h-20" /> : sum.isError ? <ErrorState /> : !sum.data!.has_data ? (
          <EmptyState icon={Leaf} text="No carbon data yet. Connect a cloud account and run a sync." action={{ href: "/connectors", label: "Open connectors" }} />
        ) : (
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
            <Figure label="Energy" value={nf(1).format(sum.data!.energy_kwh)} unit="kWh" note="estimated from usage, not metered" />
            <Figure label="CO₂e" value={nf(1).format(carbonParts(sum.data!.carbon_kg_co2e).n)} unit={carbonParts(sum.data!.carbon_kg_co2e).unit === "t" ? "t" : "kg"} note="operational, location-based" />
            <Figure label="Grid intensity" value={nf(0).format(sum.data!.carbon_intensity_g_per_kwh)} unit="gCO₂e/kWh" />
            <Figure label="SCI" value={sum.data!.sci_score === null ? "—" : nf(2).format(sum.data!.sci_score)} unit="per functional unit"
              note={sum.data!.sci_score === null ? "needs functional units reported for the project" : undefined} />
          </div>
        )}
      </Card>
      <Card index={1}>
        <CardHeader title="Daily CO₂e" />
        {trend.isLoading ? <Skeleton className="h-10" /> : trend.isError ? <ErrorState /> : trend.data!.length < 2 ?
          <p className="text-sm text-muted">Not enough days yet for a trend.</p> :
          <Sparkline values={trend.data!.map((p) => p.carbon_kg_co2e)} width={420} height={56} />}
      </Card>
      <Card index={2}>
        <CardHeader title="Methodology & data quality" action={m && <Badge tone={m.status === "verified" ? "good" : "warn"}>{m.status}</Badge>} />
        {!m ? <Skeleton className="h-24" /> : (
          <div className="space-y-3 text-sm">
            <p><b>{m.version}</b> — {m.provenance}</p>
            <p className="text-muted">Embodied carbon: <b>{m.embodied_carbon_included ? "included" : "not included"}</b>.</p>
            <ul className="list-disc space-y-1 pl-5 text-muted">{m.caveats.map((c) => <li key={c}>{c}</li>)}</ul>
            <p className="text-xs text-muted">Figures are estimates produced with the methodology version above; a new version never rewrites old results.</p>
          </div>
        )}
      </Card>
    </div>
  );
}
