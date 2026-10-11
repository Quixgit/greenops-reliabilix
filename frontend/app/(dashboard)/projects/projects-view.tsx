"use client";

import { useQueries } from "@tanstack/react-query";
import { useMemo, useState, type CSSProperties } from "react";
import { z } from "zod";
import { BudgetsPanel } from "@/components/forms/budgets-panel";
import { PolicyPanel } from "@/components/forms/policy-panel";
import { ProjectWizard } from "@/components/forms/project-wizard";
import { AllocationReportCard, AllocationRulesCard, ProjectStructure, TeamsCard } from "@/components/forms/structure-panels";
import { WorkUnits } from "@/components/forms/work-units";
import { Drawer } from "@/components/connectors/drawer";
import { Card } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconCloud, IconFolder, IconGlobe, IconPlus, IconShield, IconWallet } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { useAllocation, useBudgets, useConnections, useFinopsSummary, useProjects, useTeams } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import type { Project } from "@/lib/api/schemas";
import { formatCost, nf } from "@/lib/formatters";
import { share } from "@/lib/overview";
import { cn } from "@/lib/utils";

const policy = z.object({
  allowed_regions: z.array(z.string()), ci_carbon_block_kg: z.number().nullable(), ci_cost_block: z.number().nullable(),
  alert_carbon_monthly_kg: z.number().nullable().optional(), auto_apply_types: z.array(z.string()).default([]),
});
type Policy = z.infer<typeof policy>;
const TABS = ["projects", "teams", "budgets"] as const;
const PROVIDER: Record<string, string> = { aws: "AWS", azure: "Azure", gcp: "Google Cloud" };

function Stat({ index, icon: Icon, label, value, hint, info }: { index: number; icon: typeof IconFolder; label: string; value: React.ReactNode; hint?: string; info: string }) {
  return (
    <Card index={index} className="flex items-center gap-4 p-4">
      <Icon className="size-11 shrink-0 text-leaf-deep" strokeWidth={1.4} />
      <div className="min-w-0">
        <p className="flex items-center gap-1 text-[13px] font-medium text-slate-600">{label}<Info>{info}</Info></p>
        <p className="mt-0.5 text-2xl font-bold leading-tight tracking-tight tabular-nums">{value}</p>
        {hint && <p className="truncate text-xs text-muted">{hint}</p>}
      </div>
    </Card>
  );
}

function Chip({ on, children, title }: { on: boolean; children: React.ReactNode; title?: string }) {
  return <span title={title} className={cn("inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium", on ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-500")}><span className={cn("size-1.5 rounded-full", on ? "bg-leaf" : "bg-slate-300")} />{children}</span>;
}

/** One project at a glance: what it costs, where it runs, what rules it has; a click opens everything about it. */
function ProjectCard({ p, index, total, pol, onOpen }: { p: Project; index: number; total: number; pol?: Policy; onOpen: () => void }) {
  const t = useT();
  const cur = useFinopsSummary().data?.currency || "USD";
  const alloc = useAllocation().data;
  const teams = useTeams().data ?? [];
  const budgets = useBudgets().data ?? [];
  const conns = (useConnections().data ?? []).filter((c) => c.project_id === p.id);
  const cost = alloc?.projects.find((x) => x.project_id === p.id)?.total ?? 0;
  const budget = budgets.find((b) => b.project_id === p.id);
  const team = teams.find((x) => x.id === p.team_id)?.name;
  const clouds = [...new Set(conns.map((c) => c.provider))];
  const sh = share(cost, total);
  return (
    <li className="rise" style={{ "--i": index } as CSSProperties}>
      <button type="button" onClick={onOpen}
        className="group flex h-full w-full flex-col gap-4 rounded-2xl border border-line bg-white p-5 text-left shadow-[0_1px_2px_rgba(16,24,40,0.04)] transition-all duration-300 hover:-translate-y-1 hover:border-leaf/50 hover:shadow-[0_14px_32px_-14px_rgba(47,143,120,0.35)]">
        <span className="flex items-start gap-3">
          <IconFolder className="size-10 shrink-0 text-leaf-deep transition-transform duration-500 group-hover:-rotate-6 group-hover:scale-110" strokeWidth={1.4} />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-base font-semibold tracking-tight">{p.name}</span>
            <span className="block truncate text-xs text-muted">{t("proj.per", { unit: p.functional_unit.replace(/_/g, " ") })}{team ? ` · ${team}` : ""}</span>
          </span>
        </span>
        <span>
          <span className="flex items-baseline justify-between"><span className="text-2xl font-bold tabular-nums tracking-tight">{formatCost(cost, cur)}</span><span className="text-xs text-muted">{nf(sh < 10 ? 1 : 0).format(sh)}% {t("proj.of_total")}</span></span>
          <span className="mt-2 block h-1.5 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-leaf" style={{ width: `${Math.max(3, sh)}%`, "--d": `${index * 70}ms` } as CSSProperties} /></span>
        </span>
        <span className="flex flex-wrap gap-1.5">
          {clouds.length > 0 ? clouds.map((c) => <Chip key={c} on>{PROVIDER[c] ?? c}</Chip>) : <Chip on={false}>{t("proj.no_cloud")}</Chip>}
          <Chip on={(pol?.allowed_regions.length ?? 0) > 0} title={t("proj.regions_tip")}>{pol ? (pol.allowed_regions.length > 0 ? t("proj.regions", { count: pol.allowed_regions.length }) : t("proj.no_regions")) : "…"}</Chip>
          <Chip on={pol?.ci_carbon_block_kg != null || pol?.ci_cost_block != null}>{t("proj.ci_gate")}</Chip>
          {budget && <Chip on={budget.state === "ok"} title={t("proj.budget_tip")}>{Math.round(budget.used_pct)}% {t("proj.budget")}</Chip>}
        </span>
      </button>
    </li>
  );
}

/** The projects of the organization: a summary, a card per project and a panel with everything about the chosen one. */
export function ProjectsView({ canCreate, canBudget }: { canCreate: boolean; canBudget: boolean }) {
  const t = useT();
  const q = useProjects();
  const cost = useFinopsSummary();
  const alloc = useAllocation();
  const budgets = useBudgets();
  const [tab, setTab] = useState<(typeof TABS)[number]>("projects");
  const [openId, setOpenId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [detailTab, setDetailTab] = useState<"overview" | "policy" | "units">("overview");
  const cur = cost.data?.currency || "USD";
  const projects = q.data ?? [];
  const policies = useQueries({ queries: projects.map((p) => ({ queryKey: ["policy", p.id], queryFn: () => api(`projects/${p.id}/policy`, policy) })) });
  const polOf = (id: string) => policies[projects.findIndex((p) => p.id === id)]?.data;
  const total = useMemo(() => (alloc.data?.projects ?? []).reduce((n, x) => n + x.total, 0), [alloc.data]);
  const withRegions = policies.filter((x) => (x.data?.allowed_regions.length ?? 0) > 0).length;
  const overBudget = (budgets.data ?? []).filter((b) => b.state !== "ok").length;
  const open = projects.find((p) => p.id === openId) ?? null;
  const conns = (useConnections().data ?? []).filter((c) => c.project_id === openId);

  if (adding) {
    return <ProjectWizard onClose={() => setAdding(false)} onOpen={(id) => { setAdding(false); setTab("projects"); setOpenId(id); setDetailTab("overview"); }} />;
  }

  return (
    <div className="page-enter space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("proj.title")}</h1>
          <p className="mt-1 text-sm text-muted">{t("proj.subtitle")}</p>
        </div>
        {canCreate && (
          <button type="button" onClick={() => setAdding(true)} className="inline-flex items-center gap-2 rounded-xl btn-leaf px-4 py-2.5 text-sm font-semibold text-white ">
            <IconPlus className="size-4" strokeWidth={2.2} />{t("proj.new")}
          </button>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat index={0} icon={IconFolder} label={t("proj.stat.projects")} info={t("proj.info.projects")} value={q.isLoading ? <Skeleton className="h-7 w-10" /> : <CountUp value={projects.length} format={(n) => String(Math.round(n))} />} />
        <Stat index={1} icon={IconWallet} label={t("proj.stat.cost")} info={t("proj.info.cost")} value={alloc.isLoading ? <Skeleton className="h-7 w-24" /> : <CountUp value={total} format={(n) => formatCost(n, cur)} />} hint={t("proj.stat.cost_hint")} />
        <Stat index={2} icon={IconGlobe} label={t("proj.stat.regions")} info={t("proj.info.regions")} value={`${withRegions}/${projects.length}`} hint={t("proj.stat.regions_hint")} />
        <Stat index={3} icon={IconShield} label={t("proj.stat.budgets")} info={t("proj.info.budgets")} value={budgets.isError ? "—" : `${overBudget}`} hint={overBudget ? t("proj.stat.over") : t("proj.stat.ok")} />
      </div>

      <div role="tablist" className="flex gap-1 border-b border-line">
        {TABS.map((k) => (
          <button key={k} role="tab" type="button" aria-selected={tab === k} onClick={() => setTab(k)}
            className={cn("relative px-4 py-2.5 text-sm font-medium transition-colors", tab === k ? "text-leaf-deep" : "text-slate-500 hover:text-ink")}>
            {t(`proj.tab.${k}`)}
            <span aria-hidden className={cn("absolute inset-x-3 -bottom-px h-0.5 origin-left rounded-full bg-leaf-deep transition-transform duration-300", tab === k ? "scale-x-100" : "scale-x-0")} />
          </button>
        ))}
      </div>

      {tab === "projects" && (
        q.isLoading ? <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-44" />)}</div>
          : q.isError ? <ErrorState />
          : projects.length === 0 ? <Card interactive={false}><EmptyState icon={IconFolder} text={t("proj.empty")} /></Card>
          : (
            <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {projects.map((p, i) => <ProjectCard key={p.id} p={p} index={i} total={total} pol={polOf(p.id)} onOpen={() => { setOpenId(p.id); setDetailTab("overview"); }} />)}
            </ul>
          )
      )}
      {tab === "teams" && (
        <div className="space-y-5"><AllocationReportCard /><div className="grid gap-5 lg:grid-cols-2"><TeamsCard canEdit={canCreate} /><AllocationRulesCard canEdit={canCreate} /></div></div>
      )}
      {tab === "budgets" && <BudgetsPanel canEdit={canBudget} currency={cur} />}

      <Drawer open={!!open} onClose={() => setOpenId(null)} title={open?.name ?? ""} closeLabel={t("common.cancel")}>
        {open && (
          <div className="space-y-5">
            <div role="tablist" className="flex gap-1 rounded-xl bg-slate-100 p-1 text-sm">
              {(["overview", "policy", "units"] as const).map((k) => (
                <button key={k} role="tab" type="button" aria-selected={detailTab === k} onClick={() => setDetailTab(k)}
                  className={cn("flex-1 rounded-lg px-3 py-1.5 font-medium transition-all", detailTab === k ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{t(`proj.detail.${k}`)}</button>
              ))}
            </div>
            <div key={detailTab} className="form-swap">
              {detailTab === "overview" && (
                <div className="space-y-5">
                  <ProjectStructure projectId={open.id} teamId={open.team_id} canEdit={canCreate} />
                  <div>
                    <p className="mb-2 flex items-center gap-2 text-sm font-semibold"><IconCloud className="size-5 text-leaf-deep" />{t("proj.connections")}</p>
                    {conns.length === 0 ? <p className="rounded-xl bg-slate-50 px-3 py-3 text-sm text-muted ring-1 ring-line/70">{t("proj.no_connections")}</p> : (
                      <ul className="text-sm">{conns.map((c) => (
                        <li key={c.id} className="flex items-center justify-between border-t border-line/70 py-2.5">
                          <span className="font-medium">{PROVIDER[c.provider] ?? c.provider} <span className="font-normal text-muted">{c.account_ref}</span></span>
                          <span className="text-xs text-muted">{c.last_sync_at ? t("proj.synced") : t("proj.not_synced")}</span>
                        </li>
                      ))}</ul>
                    )}
                  </div>
                </div>
              )}
              {detailTab === "policy" && <PolicyPanel projectId={open.id} canEdit={canCreate} />}
              {detailTab === "units" && <WorkUnits projectId={open.id} unit={open.functional_unit} canEdit={canCreate} />}
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
