"use client";

import { useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconWallet } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { usePoolChoice, useSetPool, useK8sCost, type K8sClusterCost } from "@/hooks/use-k8s-cost";
import { useT } from "@/i18n/provider";
import { formatCostFine as formatCost } from "@/lib/formatters";
import { cn } from "@/lib/utils";

function Kpi({ label, value, hint, tone }: { label: string; value: string; hint?: string; tone?: string }) {
  return <div className="rounded-xl bg-slate-50 px-3 py-2.5"><p className="text-[12px] font-medium text-slate-600">{label}</p><p className={cn("mt-0.5 text-xl font-bold tabular-nums", tone)}>{value}</p>{hint && <p className="text-[11px] leading-snug text-muted">{hint}</p>}</div>;
}

/** Choose which billed services pay for the cluster: the cost report is only as right as this. */
function PoolEditor({ c, currency, canEdit }: { c: K8sClusterCost; currency: string; canEdit: boolean }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const choice = usePoolChoice(c.project_id, c.cluster, open);
  const save = useSetPool();
  const [picked, setPicked] = useState<string[] | null>(null);
  const current = picked ?? choice.data?.services ?? c.pool_services;
  const toggle = (n: string) => setPicked(current.includes(n) ? current.filter((x) => x !== n) : [...current, n]);
  return (
    <div className="rounded-xl border border-line px-3 py-2.5 text-sm">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className={cn("min-w-0 text-[13px]", c.pool_assumed ? "text-[#9a6b12]" : "text-slate-700")}>
          {c.pool_assumed ? t("k8c.pool_assumed") : t("k8c.pool_services", { services: c.pool_services.join(", ") })}
        </p>
        {canEdit && <button type="button" onClick={() => setOpen(!open)} className="text-xs font-semibold text-primary hover:underline">{open ? t("k8c.close") : t("k8c.choose")}</button>}
      </div>
      {open && (
        <div className="mt-2 space-y-2">
          <p className="text-xs text-muted">{t("k8c.pool_hint")}</p>
          {choice.isLoading ? <Skeleton className="h-16" /> : (
            <ul className="grid gap-1 sm:grid-cols-2">
              {(choice.data?.candidates ?? []).map((s) => (
                <li key={s.name}><label className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1 hover:bg-slate-50">
                  <input type="checkbox" checked={current.includes(s.name)} onChange={() => toggle(s.name)} />
                  <span className="min-w-0 flex-1 truncate text-[13px]" title={s.name}>{s.name}</span>
                  <span className="text-xs tabular-nums text-muted">{formatCost(s.cost, currency)}</span>
                </label></li>
              ))}
            </ul>
          )}
          <div className="flex items-center gap-2">
            <button type="button" disabled={save.isPending} onClick={() => save.mutate({ projectId: c.project_id, cluster: c.cluster, services: current }, { onSuccess: () => { setOpen(false); setPicked(null); } })} className="rounded-xl btn-leaf px-3.5 py-1.5 text-xs font-semibold text-white disabled:opacity-60">{t("k8c.save")}</button>
            <button type="button" onClick={() => setPicked([])} className="text-xs text-muted hover:underline">{t("k8c.reset")}</button>
            {save.isError && <span role="alert" className="text-xs text-[#a13a4b]">{save.error.message}</span>}
          </div>
        </div>
      )}
    </div>
  );
}

function ClusterCostCard({ c, currency, cpuShare, canEdit, index }: { c: K8sClusterCost; currency: string; cpuShare: number; canEdit: boolean; index: number }) {
  const t = useT();
  const maxWl = Math.max(...c.workloads.map((w) => w.cost), 1e-9);
  const pct = (v: number) => (c.pool_cost > 0 ? `${((v / c.pool_cost) * 100).toFixed(0)}%` : "");
  return (
    <Card index={index} interactive={false} className="space-y-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-[15px] font-semibold">{c.cluster}</h3>
        <span className="text-xs text-muted">{t("k8c.days", { days: c.days })}</span>
      </div>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Kpi label={t("k8c.pool")} value={formatCost(c.pool_cost, currency)} hint={t("k8c.pool_hint_short")} />
        <Kpi label={t("k8c.allocated")} value={formatCost(c.allocated_cost, currency)} hint={pct(c.allocated_cost)} />
        {c.idle_cost === null
          ? <Kpi label={t("k8c.idle")} value="—" hint={t("k8c.idle_unknown")} />
          : <Kpi label={t("k8c.idle")} value={formatCost(c.idle_cost, currency)} hint={t("k8c.idle_hint", { pct: pct(c.idle_cost) })} tone={c.pool_cost > 0 && c.idle_cost / c.pool_cost > 0.4 ? "text-[#b7791f]" : undefined} />}
        <Kpi label={t("k8c.waste")} value={formatCost(c.waste_cost, currency)} hint={t("k8c.waste_hint")} tone={c.waste_cost > 0 ? "text-[#b7791f]" : undefined} />
      </div>
      <PoolEditor c={c} currency={currency} canEdit={canEdit} />
      {c.namespaces.length > 0 && (
        <div>
          <p className="mb-1.5 text-[13px] font-semibold text-slate-700">{t("k8c.namespaces")}</p>
          <div className="space-y-1">
            {c.namespaces.map((n, i) => (
              <div key={n.namespace} className="row-in grid items-center gap-x-3 sm:grid-cols-[9rem_minmax(0,1fr)_7rem]" style={{ "--i": i } as CSSProperties}>
                <span className="truncate text-sm" title={n.namespace}>{n.namespace}</span>
                <span className="h-2 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-[#7aa8e6]" style={{ width: `${Math.max(1, n.share_pct)}%` }} /></span>
                <span className="text-right text-sm tabular-nums">{formatCost(n.cost, currency)} <span className="text-[11px] text-muted">{n.share_pct.toFixed(0)}%</span></span>
              </div>
            ))}
          </div>
        </div>
      )}
      {c.workloads.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[34rem] text-left text-sm">
            <thead><tr className="text-[11px] uppercase tracking-wide text-muted"><th className="py-1.5 font-medium">{t("k8c.workload")}</th><th className="py-1.5 font-medium">{t("k8c.cost")}</th><th className="py-1.5 text-right font-medium">{t("k8c.cpu_use")}</th><th className="py-1.5 text-right font-medium">{t("k8c.mem_use")}</th><th className="py-1.5 text-right font-medium">{t("k8c.waste")}</th></tr></thead>
            <tbody>
              {c.workloads.slice(0, 15).map((w) => (
                <tr key={`${w.namespace}/${w.workload}`} className="border-t border-line/70">
                  <td className="max-w-[14rem] truncate py-2" title={`${w.namespace}/${w.workload}`}><span className="text-muted">{w.namespace}/</span>{w.workload}</td>
                  <td className="py-2 pr-3"><span className="flex items-center gap-2"><span className="h-1.5 w-20 overflow-hidden rounded-full bg-slate-100"><span className="block h-full rounded-full bg-[#7aa8e6]" style={{ width: `${Math.max(2, (w.cost / maxWl) * 100)}%` }} /></span><span className="tabular-nums">{formatCost(w.cost, currency)}</span></span></td>
                  <td className={cn("py-2 text-right tabular-nums", w.cpu_utilization_pct !== null && w.cpu_utilization_pct < 30 && "text-[#b7791f]")}>{w.cpu_utilization_pct === null ? "—" : `${w.cpu_utilization_pct.toFixed(0)}%`}</td>
                  <td className={cn("py-2 text-right tabular-nums", w.memory_utilization_pct !== null && w.memory_utilization_pct < 30 && "text-[#b7791f]")}>{w.memory_utilization_pct === null ? "—" : `${w.memory_utilization_pct.toFixed(0)}%`}</td>
                  <td className="py-2 text-right tabular-nums">{w.waste_cost > 0 ? formatCost(w.waste_cost, currency) : "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {c.workloads_without_usage > 0 && <p className="text-xs text-muted">{t("k8c.unmeasured", { count: c.workloads_without_usage })}</p>}
      {c.unallocated_cost > 0 && <p className="text-xs text-[#9a6b12]">{t("k8c.unallocated", { amount: formatCost(c.unallocated_cost, currency) })}</p>}
      <p className="text-[11px] leading-snug text-muted">{t("k8c.basis", { cpu: Math.round(cpuShare * 100), mem: Math.round((1 - cpuShare) * 100) })}</p>
    </Card>
  );
}

/** What each cluster costs and who in it uses that cost: the nodes' bill split by what workloads hold, with idle and waste. */
export function K8sCost({ cluster, canEdit }: { cluster: string; canEdit: boolean }) {
  const t = useT();
  const q = useK8sCost(cluster);
  return (
    <div className="space-y-4">
      <Card index={0} interactive={false}>
        <CardHeader icon={<IconWallet className="size-[18px] text-sky-500" />} title={t("k8c.title")} info={t("k8c.info")} />
        <p className="max-w-3xl text-sm text-muted">{t("k8c.lead")}</p>
      </Card>
      {q.isLoading ? <Skeleton className="h-60" /> : q.isError ? <ErrorState /> : !q.data || q.data.clusters.length === 0 ? (
        <Card interactive={false}><EmptyState icon={IconWallet} text={t("k8c.empty")} /></Card>
      ) : q.data.clusters.map((c, i) => <ClusterCostCard key={`${c.project_id}/${c.cluster}`} c={c} currency={q.data.currency} cpuShare={q.data.cpu_share} canEdit={canEdit} index={i + 1} />)}
    </div>
  );
}
