"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Lightbulb } from "lucide-react";
import { useState } from "react";
import { Card } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api/client";
import { recommendation, type Recommendation } from "@/lib/api/schemas";
import { useRecommendations } from "@/hooks/use-overview";

const FILTERS = ["open", "approved", "applied", "dismissed", "all"] as const;

function Row({ r, canDecide }: { r: Recommendation; canDecide: boolean }) {
  const qc = useQueryClient();
  const act = useMutation({
    mutationFn: (verb: "approve" | "apply" | "dismiss") =>
      api(`recommendations/${r.id}/${verb}`, recommendation, undefined, { method: "POST", body: "{}" }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["recommendations"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  const passed = r.compliance_check.residency === "passed";
  const btn = "rounded-lg px-3 py-1.5 text-xs font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-60";
  return (
    <li className="space-y-2 py-3 text-sm">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="font-medium">{r.title}</p>
          <p className="text-muted">{r.provider} · {r.service_name} · {r.current_region} → {r.recommended_region}</p>
        </div>
        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs capitalize">{r.status}</span>
      </div>
      <p>
        <span className="font-semibold text-primary">−{r.estimated_carbon_reduction_pct.toFixed(0)}% carbon</span>
        {" "}(~{r.carbon_reduction_kg_month.toFixed(1)} kg CO₂e/month) ·{" "}
        {r.estimated_cost_impact === null ? <span className="text-muted">cost impact not estimated</span> : <span>cost {r.estimated_cost_impact < 0 ? "saving" : "increase"} {Math.abs(r.estimated_cost_impact).toFixed(2)}</span>}
        {" "}· confidence {(r.confidence * 100).toFixed(0)}%
      </p>
      <p className={passed ? "text-muted" : "text-bad"}>Data residency: {r.compliance_check.residency}{r.compliance_check.allowed_regions?.length ? ` (allowed: ${r.compliance_check.allowed_regions.join(", ")})` : ""}</p>
      {canDecide && (r.status === "open" || r.status === "approved") && (
        <div className="flex gap-2">
          {r.status === "open" && <button className={btn} disabled={act.isPending || !passed} onClick={() => act.mutate("approve")}>Approve</button>}
          {r.status === "approved" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("apply")}>Mark as applied</button>}
          <button className={btn} disabled={act.isPending} onClick={() => act.mutate("dismiss")}>Dismiss</button>
        </div>
      )}
      {act.isError && <p role="alert" className="text-xs text-bad">{act.error.message}</p>}
    </li>
  );
}

export function RecommendationsView({ canDecide }: { canDecide: boolean }) {
  const [status, setStatus] = useState<(typeof FILTERS)[number]>("open");
  const q = useRecommendations(status);
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Recommendations</h1>
      <div className="flex flex-wrap gap-2" role="tablist">
        {FILTERS.map((f) => (
          <button key={f} role="tab" aria-selected={f === status} onClick={() => setStatus(f)}
            className={`rounded-full px-3 py-1 text-xs font-medium capitalize ring-1 ring-line ${f === status ? "bg-primary text-white" : "bg-white"}`}>{f}</button>
        ))}
      </div>
      <Card>
        {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ?
          <EmptyState icon={Lightbulb} text="No recommendations here yet. They appear after a cloud account is synced and carbon is calculated." /> : (
            <ul className="divide-y divide-line/70">{q.data!.map((r) => <Row key={r.id} r={r} canDecide={canDecide} />)}</ul>
          )}
      </Card>
    </div>
  );
}
