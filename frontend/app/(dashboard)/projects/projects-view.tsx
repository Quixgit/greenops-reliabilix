"use client";

import { PageHead, PageTabs } from "@/components/dx/page";
import { Tile } from "@/components/dashboard/tile";
import type { GlyphName, Tone } from "@/components/ui/glyphs";
import { CloudSwitcher } from "@/components/dashboard/cloud-switcher";
import { useMutation, useQueries, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { z } from "zod";
import { BudgetsPanel } from "@/components/forms/budgets-panel";
import { PolicyPanel } from "@/components/forms/policy-panel";
import { ProjectWizard } from "@/components/forms/project-wizard";
import { ShowbackPanel } from "@/components/forms/showback-panel";
import { AllocationReportCard, AllocationRulesCard, ProjectStructure, TeamsCard } from "@/components/forms/structure-panels";
import { UnitEconomicsPanel } from "@/components/forms/unit-economics";
import { WorkUnits } from "@/components/forms/work-units";
import { Drawer } from "@/components/connectors/drawer";
import { Card } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconCloud, IconFolder, IconGlobe, IconPlus, IconShield, IconTrash, IconWallet } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { useAllocation, useBudgets, useConnections, useFinopsSummary, useProjects, useTeams } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api, apiVoid } from "@/lib/api/client";
import type { Project } from "@/lib/api/schemas";
import { formatCost, nf } from "@/lib/formatters";
import { share } from "@/lib/overview";
import { cn } from "@/lib/utils";

const policy = z.object({
  allowed_regions: z.array(z.string()), ci_carbon_block_kg: z.number().nullable(), ci_cost_block: z.number().nullable(),
  alert_carbon_monthly_kg: z.number().nullable().optional(), auto_apply_types: z.array(z.string()).default([]),
});
type Policy = z.infer<typeof policy>;
const TABS = ["projects", "teams", "showback", "unit", "budgets"] as const;
const PROVIDER: Record<string, string> = { aws: "AWS", azure: "Azure", gcp: "Google Cloud" };

function Stat({ index, glyph, tone, label, value, hint, info }: { index: number; glyph: GlyphName; tone: Tone; label: string; value: React.ReactNode; hint?: string; info: string }) {
  return <Tile index={index} icon={glyph} tone={tone} label={label} info={info} value={value} foot={hint} />;
}

function Chip({ on, children, title }: { on: boolean; children: React.ReactNode; title?: string }) {
  return <span title={title} className={cn("dx-tag", on ? "is-sage" : "is-line")}>{children}</span>;
}

const ROW = "minmax(180px,1.6fr) minmax(130px,1fr) minmax(140px,1.2fr) minmax(200px,1.6fr) 44px";

/** One project in the table: what it costs, where it runs, what rules it has; a click on the row opens everything about it. */
function ProjectRow({ p, total, pol, onOpen, canDelete }: { p: Project; total: number; pol?: Policy; onOpen: () => void; canDelete: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const [confirm, setConfirm] = useState(false);
  const del = useMutation({
    mutationFn: () => apiVoid(`projects/${p.id}`, { method: "DELETE" }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["projects"] }); void qc.invalidateQueries({ queryKey: ["allocation"] }); void qc.invalidateQueries({ queryKey: ["budgets"] });
    },
  });
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
  if (confirm) {
    return (
      <li className="dx-row" style={{ background: "var(--dx-rose)" }} role="alert">
        <div style={{ gridColumn: "1 / -1", display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
          <b style={{ color: "var(--dx-rose-ink)" }}>{t("proj.delete_ask", { name: p.name })}</b>
          <span style={{ color: "var(--dx-ink-2)", fontSize: 12.5, flex: 1, minWidth: 200 }}>{del.isError ? del.error.message : t("proj.delete_hint")}</span>
          <button type="button" className="dx-btn is-sm" onClick={() => setConfirm(false)}>{t("common.cancel")}</button>
          <button type="button" className="dx-btn is-sm is-primary" disabled={del.isPending} onClick={() => del.mutate()}>{t("proj.delete")}</button>
        </div>
      </li>
    );
  }
  return (
    <li className="dx-row is-click" style={{ gridTemplateColumns: ROW }} onClick={onOpen}>
      <div style={{ minWidth: 0 }}>
        <button type="button" className="dx-link-btn" onClick={(e) => { e.stopPropagation(); onOpen(); }}>{p.name}</button>
        <div style={{ color: "var(--dx-muted)", fontSize: 12 }}>{t("proj.per", { unit: p.functional_unit.replace(/_/g, " ") })}{team ? ` · ${team}` : ""}</div>
      </div>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
        {clouds.length > 0 ? clouds.map((c) => <Chip key={c} on>{PROVIDER[c] ?? c}</Chip>) : <Chip on={false}>{t("proj.no_cloud")}</Chip>}
      </div>
      <div>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}><b className="tabular-nums">{formatCost(cost, cur)}</b><span style={{ color: "var(--dx-muted)", fontSize: 12 }}>{nf(sh < 10 ? 1 : 0).format(sh)}%</span></div>
        <div className="dx-meter-line"><i style={{ width: `${Math.max(3, sh)}%` }} /></div>
      </div>
      <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
        <Chip on={(pol?.allowed_regions.length ?? 0) > 0} title={t("proj.regions_tip")}>{pol ? (pol.allowed_regions.length > 0 ? t("proj.regions", { count: pol.allowed_regions.length }) : t("proj.no_regions")) : "…"}</Chip>
        <Chip on={pol?.ci_carbon_block_kg != null || pol?.ci_cost_block != null}>{t("proj.ci_gate")}</Chip>
        {budget && <Chip on={budget.state === "ok"} title={t("proj.budget_tip")}>{Math.round(budget.used_pct)}% {t("proj.budget")}</Chip>}
      </div>
      <div style={{ textAlign: "right" }}>
        {canDelete && <button type="button" className="dx-iconbtn" aria-label={t("proj.delete")} title={t("proj.delete")} onClick={(e) => { e.stopPropagation(); del.reset(); setConfirm(true); }}><IconTrash className="size-[18px]" /></button>}
      </div>
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
  useEffect(() => {
    const h = window.location.hash.slice(1);
    if ((TABS as readonly string[]).includes(h)) setTab(h as (typeof TABS)[number]);
  }, []);
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
  // The Overview's "Report what your work produces" step links here with ?units: open the first project on its work-units tab, so
  // the person lands on the form to fill in instead of on a list.
  const unitsDeepLink = useRef(false);
  useEffect(() => {
    if (unitsDeepLink.current || projects.length === 0 || typeof window === "undefined") return;
    if (!new URLSearchParams(window.location.search).has("units")) return;
    unitsDeepLink.current = true;
    setTab("projects");
    setOpenId(projects[0].id);
    setDetailTab("units");
  }, [projects]);
  const open = projects.find((p) => p.id === openId) ?? null;
  const conns = (useConnections().data ?? []).filter((c) => c.project_id === openId);

  return (
    <div className="page-enter">
      <PageHead title={t("proj.title")} sub={t("proj.subtitle")} actions={<>
        <CloudSwitcher />
        {canCreate && !adding && <button type="button" className="dx-btn is-primary" onClick={() => setAdding(true)}><IconPlus className="size-4" />{t("proj.new")}</button>}
      </>} />

      {adding && (
        <div style={{ marginBottom: 20 }}>
          <ProjectWizard onClose={() => setAdding(false)} onOpen={(id) => { setAdding(false); setTab("projects"); setOpenId(id); setDetailTab("overview"); }} />
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" style={{ marginBottom: 20 }}>
        <Stat index={0} glyph="list" tone="green" label={t("proj.stat.projects")} info={t("proj.info.projects")} value={q.isLoading ? <Skeleton className="h-7 w-10" /> : <CountUp value={projects.length} format={(n) => String(Math.round(n))} />} />
        <Stat index={1} glyph="wallet" tone="blue" label={t("proj.stat.cost")} info={t("proj.info.cost")} value={alloc.isLoading ? <Skeleton className="h-7 w-24" /> : <CountUp value={total} format={(n) => formatCost(n, cur)} />} hint={t("proj.stat.cost_hint")} />
        <Stat index={2} glyph="pin" tone="sky" label={t("proj.stat.regions")} info={t("proj.info.regions")} value={`${withRegions}/${projects.length}`} hint={t("proj.stat.regions_hint")} />
        <Stat index={3} glyph="shield" tone="violet" label={t("proj.stat.budgets")} info={t("proj.info.budgets")} value={budgets.isError ? "—" : `${overBudget}`} hint={overBudget ? t("proj.stat.over") : t("proj.stat.ok")} />
      </div>

      <PageTabs label={t("proj.title")} value={tab} onChange={(k) => { setTab(k as (typeof TABS)[number]); window.history.replaceState(null, "", `#${k}`); }}
        tabs={TABS.map((k) => ({ id: k, label: t(`proj.tab.${k}`), count: k === "projects" ? projects.length : null }))} />
      <div className="dxp-body">
      {tab === "projects" && (
        q.isLoading ? <div className="grid gap-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-14" />)}</div>
          : q.isError ? <ErrorState />
          : projects.length === 0 ? <Card interactive={false}><EmptyState icon={IconFolder} text={t("proj.empty")} /></Card>
          : (
            <div className="dx-panel">
              <div className="dx-row dx-head-row" style={{ gridTemplateColumns: ROW }}>
                <span>{t("dx.proj.col.project", undefined, "Project")}</span><span>{t("dx.proj.col.clouds", undefined, "Clouds")}</span><span>{t("dx.proj.col.cost", undefined, "Cost")}</span><span>{t("dx.proj.col.rules", undefined, "Rules")}</span><span />
              </div>
              <ul className="dx-rows">
                {projects.map((p) => <ProjectRow key={p.id} p={p} total={total} pol={polOf(p.id)} canDelete={canCreate} onOpen={() => { setOpenId(p.id); setDetailTab("overview"); }} />)}
              </ul>
            </div>
          )
      )}
      {tab === "teams" && (
        <div className="space-y-5"><AllocationReportCard /><div className="grid gap-5 lg:grid-cols-2"><TeamsCard canEdit={canCreate} /><AllocationRulesCard canEdit={canCreate} /></div></div>
      )}
      {tab === "showback" && <ShowbackPanel />}
      {tab === "unit" && <UnitEconomicsPanel />}
      {tab === "budgets" && <BudgetsPanel canEdit={canBudget} currency={cur} />}
      </div>

      <Drawer open={!!open} onClose={() => setOpenId(null)} title={open?.name ?? ""} closeLabel={t("common.cancel")}>
        {open && (
          <div className="space-y-5">
            <div role="tablist" className="dx-seg-inline">
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
