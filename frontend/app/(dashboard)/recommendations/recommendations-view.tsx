"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Segmented } from "@/components/ui/controls";
import { CloudSwitcher } from "@/components/dashboard/cloud-switcher";
import { IconBulb as Lightbulb } from "@/components/ui/icons";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { api } from "@/lib/api/client";
import { automationJob, recommendation, type ImpactVerification, type Recommendation } from "@/lib/api/schemas";
import { useRecommendations } from "@/hooks/use-overview";
import { CompareOptions, LatencyChip, Metric, parseOptions } from "@/components/dashboard/region-options";
import { regionName } from "@/lib/regions";
import { formatCost } from "@/lib/formatters";
import { CommitmentsPanel } from "@/components/recommendations/commitments-panel";
import { useT } from "@/i18n/provider";

const FILTERS = ["open", "approved", "applied", "verified", "dismissed", "all"] as const;

function Row({ r, canDecide, canPlan }: { r: Recommendation; canDecide: boolean; canPlan: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const router = useRouter();
  const plan = useMutation({
    mutationFn: () => api("automation/jobs", automationJob, undefined, { method: "POST", body: JSON.stringify({ recommendation_id: r.id }) }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["automation-jobs"] }); void qc.invalidateQueries({ queryKey: ["automation-jobs-lite"] }); router.push("/automation"); },
  });
  const act = useMutation({
    mutationFn: (verb: "approve" | "apply" | "dismiss") =>
      api(`recommendations/${r.id}/${verb}`, recommendation, undefined, { method: "POST", body: "{}" }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["recommendations"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  const passed = r.compliance_check.residency === "passed" || r.compliance_check.residency === "not_applicable";
  const btn = "rounded-xl px-3 py-1.5 text-xs font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-60";
  return (
    <li className="space-y-2 py-3 text-sm">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="font-medium">{r.title}</p>
          <p className="text-muted">{r.provider} · {r.service_name} · {regionName(r.current_region)} → {regionName(r.recommended_region)}</p>
        </div>
        <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs capitalize">{r.status}</span>
      </div>
      {r.type === "k8s_rightsizing" ? <K8sDetail r={r} /> : (
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <Metric label={t("rec.carbon")} tone="leaf">−{r.estimated_carbon_reduction_pct.toFixed(0)}% <span className="font-normal opacity-80">(~{r.carbon_reduction_kg_month.toFixed(1)} kg CO₂e/mo)</span></Metric>
            {r.estimated_cost_impact === null
              ? <Metric label={t("rec.cost")} tone="slate" title={t("rec.cost_unknown")}>{t("rec.cost_unknown")}</Metric>
              : <Metric label={t("rec.cost")} tone={r.estimated_cost_impact < 0 ? "leaf" : r.estimated_cost_impact === 0 ? "slate" : "amber"} title={r.cost_basis === "region_price_list" ? t("rec.cost_basis_list") : r.cost_basis === "region_price_index" ? t("rec.cost_basis_index") : undefined}>
                  {r.estimated_cost_impact === 0 ? t("rec.cost_same")
                    : r.cost_basis === "region_price_list" ? t(r.estimated_cost_impact < 0 ? "rec.cost_rough" : "rec.cost_rough_more", { amount: formatCost(Math.abs(r.estimated_cost_impact)) })
                    : r.estimated_cost_impact < 0 ? t("rec.cost_saving", { amount: formatCost(Math.abs(r.estimated_cost_impact)) }) : t("rec.cost_more", { amount: formatCost(r.estimated_cost_impact) })}
                </Metric>}
            <LatencyChip details={r.details} from={r.current_region} to={r.recommended_region} />
            <span className="text-xs text-muted">confidence {(r.confidence * 100).toFixed(0)}%</span>
          </div>
          <CompareOptions options={parseOptions(r.details)} budget={r.details?.latency_budget_ms} />
        </div>
      )}
      {r.type !== "k8s_rightsizing" && <p className={passed ? "text-muted" : "text-bad"}>Data residency: {r.compliance_check.residency}{r.compliance_check.allowed_regions?.length ? ` (allowed: ${r.compliance_check.allowed_regions.join(", ")})` : ""}</p>}
      {r.status === "applied" && r.type !== "k8s_rightsizing" && <p className="text-muted">Applied — measured result appears once 18 days of data after the change are in.</p>}
      {r.verification && <VerifiedImpact v={r.verification} />}
      {canDecide && (r.status === "open" || r.status === "approved") && (
        <div className="flex gap-2">
          {r.status === "open" && <button className={btn} disabled={act.isPending || !passed} onClick={() => act.mutate("approve")}>Approve</button>}
          {r.status === "approved" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("apply")}>Mark as applied</button>}
          <button className={btn} disabled={act.isPending} onClick={() => act.mutate("dismiss")}>Dismiss</button>
        </div>
      )}
      {canPlan && r.status === "approved" && r.type !== "k8s_rightsizing" && (
        <div><button className={btn} disabled={plan.isPending} onClick={() => plan.mutate()}>Create the plan</button>
          <span className="ml-2 text-xs text-muted">A reviewable plan with steps and rollback; nothing is changed in your cloud.</span></div>
      )}
      {act.isError && <p role="alert" className="text-xs text-bad">{act.error.message}</p>}
      {plan.isError && <p role="alert" className="text-xs text-bad">{plan.error.message}</p>}
    </li>
  );
}

/** A Kubernetes request change claims no carbon or cost saving: it shows the measured peak next to the request. */
function K8sDetail({ r }: { r: Recommendation }) {
  const d = r.details ?? {};
  const gib = (b?: string) => (b ? `${(Number(b) / 1024 ** 3).toFixed(1)} GiB` : "");
  return (
    <div className="space-y-1 text-sm">
      {d.cpu_request_recommended_cores && (
        <p>CPU request <b>{d.cpu_request_current_cores}</b> → <b className="text-primary">{d.cpu_request_recommended_cores}</b> cores
          <span className="text-muted"> (measured peak {d.cpu_peak_cores} cores)</span></p>
      )}
      {d.memory_request_recommended_bytes && (
        <p>Memory request <b>{gib(d.memory_request_current_bytes)}</b> → <b className="text-primary">{gib(d.memory_request_recommended_bytes)}</b>
          <span className="text-muted"> (measured peak {gib(d.memory_peak_bytes)})</span></p>
      )}
      <p className="text-xs text-muted">
        From {d.hours} hours of measurements in {r.current_region}. The recommended request is the highest hourly usage plus 25 % headroom.
        It frees reserved capacity; fewer nodes follow only if the scheduler can pack better, so no saving is claimed.
        Hourly averages hide short spikes: check them before applying.
      </p>
    </div>
  );
}

const OUTCOME: Record<string, { label: string; tone: "good" | "warn" | "bad" | "neutral" }> = {
  realized: { label: "Realized", tone: "good" }, partial: { label: "Partly realized", tone: "warn" },
  not_realized: { label: "Not realized", tone: "bad" }, inconclusive: { label: "Could not be measured", tone: "neutral" },
};

/** The measured result, always next to the forecast and never mixed with it. */
function VerifiedImpact({ v }: { v: ImpactVerification }) {
  const t = useT();
  const o = OUTCOME[v.outcome];
  return (
    <div className="rounded-lg bg-slate-50 p-3 text-xs ring-1 ring-line">
      <p className="flex items-center gap-2 font-semibold"><Badge tone={o.tone}>{o.label}</Badge>
        {v.realization_pct !== null && <span>{v.realization_pct.toFixed(0)}% of the expected carbon reduction</span>}</p>
      {v.adjusted_carbon_kg_month !== null ? (
        <>
          <p className="mt-1">
            {t("vf.measured_vs_control", { kg: v.adjusted_carbon_kg_month.toFixed(1), expected: v.expected_carbon_kg_month.toFixed(1) })}
            {v.adjusted_cost_month !== null && <> · {t(v.adjusted_cost_month >= 0 ? "vf.cost_saving" : "vf.cost_increase", { amount: Math.abs(v.adjusted_cost_month).toFixed(2) })}
              {v.expected_cost_month !== null && <> ({t("vf.expected", { amount: v.expected_cost_month.toFixed(2) })})</>}</>}
          </p>
          <p className="mt-1 text-muted">{t("vf.control", { growth: `${(v.control_growth_pct ?? 0) > 0 ? "+" : ""}${(v.control_growth_pct ?? 0).toFixed(1)}`, days: v.control_days })}
            {v.measured_carbon_kg_month !== null && <> {t("vf.raw", { kg: v.measured_carbon_kg_month.toFixed(1) })}</>}</p>
        </>
      ) : v.measured_carbon_kg_month !== null && (
        <p className="mt-1">
          Measured {v.measured_carbon_kg_month.toFixed(1)} kg CO₂e/month (expected {v.expected_carbon_kg_month.toFixed(1)})
          {v.measured_cost_month !== null && <> · cost {v.measured_cost_month >= 0 ? "saving" : "increase"} {Math.abs(v.measured_cost_month).toFixed(2)}/month
            {v.expected_cost_month !== null && <> (expected {v.expected_cost_month.toFixed(2)})</>}</>}
        </p>
      )}
      <p className="mt-1 text-muted">{v.note} Based on {v.baseline_days} days before and {v.after_days} days after.</p>
    </div>
  );
}

export function RecommendationsView({ canDecide, canPlan }: { canDecide: boolean; canPlan: boolean }) {
  const t = useT();
  const [tab, setTab] = useState<"moves" | "commitments">("moves");
  const [status, setStatus] = useState<(typeof FILTERS)[number]>("open");
  const q = useRecommendations(status);
  const pg = usePaged(q.data ?? [], 10);
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <h1 className="text-2xl font-bold tracking-tight">Recommendations</h1>
        <CloudSwitcher />
      </div>
      <Segmented label="Recommendations" value={tab} onChange={setTab} options={(["moves", "commitments"] as const).map((k) => ({ value: k, label: t(`cm.tab.${k}`) }))} />
      {tab === "commitments" ? <div className="form-swap"><CommitmentsPanel /></div> : <>
      <Segmented size="sm" label="Status" value={status} onChange={setStatus} options={FILTERS.map((f) => ({ value: f, label: <span className="capitalize">{f}</span> }))} className="max-w-full overflow-x-auto" />
      <Card>
        {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ?
          <EmptyState icon={Lightbulb} text="No recommendations here yet. They appear after a cloud account is synced and carbon is calculated. Moves to a cleaner region are only proposed into regions you allow: set the allowed regions in a project's policy." action={{ href: "/settings", label: "Open project policy" }} /> : (
            <ul className="divide-y divide-line/70">{pg.slice.map((r) => <Row key={r.id} r={r} canDecide={canDecide} canPlan={canPlan} />)}</ul>
          )}
        <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
      </Card>
      </>}
    </div>
  );
}
