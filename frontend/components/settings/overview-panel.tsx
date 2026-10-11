"use client";

import { useQueries } from "@tanstack/react-query";
import { ArrowUpRight, Bell, Building2, Cloud, Leaf, ShieldCheck, SlidersHorizontal, Workflow, type LucideIcon } from "lucide-react";
import Link from "next/link";
import type { CSSProperties, ReactNode } from "react";
import { z } from "zod";
import { Skeleton } from "@/components/ui/skeleton";
import { useApiKeys, useDestinations, useMembers, usePlan, useTenant } from "@/hooks/use-admin";
import { useBudgets, useConnections, useK8sClusters, useMethodologyVersions, useProjects, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { connectorSummary, type ConnectorState } from "@/lib/connectors";
import { timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const policy = z.object({
  allowed_regions: z.array(z.string()), alert_carbon_monthly_kg: z.number().nullable().optional(),
  ci_carbon_block_kg: z.number().nullable(), ci_cost_block: z.number().nullable(), auto_apply_types: z.array(z.string()).default([]),
});

type Tone = "good" | "warn" | "bad" | "mute";
const DOT: Record<Tone, string> = { good: "bg-emerald-500", warn: "bg-amber-500", bad: "bg-red-500", mute: "bg-slate-300" };
const TEXT: Record<Tone, string> = { good: "text-emerald-700", warn: "text-amber-700", bad: "text-red-700", mute: "text-muted" };
const TILE: Record<string, string> = {
  emerald: "from-emerald-400 to-teal-600", blue: "from-sky-400 to-blue-600", violet: "from-violet-400 to-indigo-600",
  amber: "from-amber-400 to-orange-500", rose: "from-rose-400 to-pink-600", slate: "from-slate-500 to-slate-700",
};

function Status({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium", TEXT[tone])}>
      <span className="relative flex size-1.5">
        {tone === "good" && <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-60" />}
        <span className={cn("relative inline-flex size-1.5 rounded-full", DOT[tone])} />
      </span>
      {children}
    </span>
  );
}

function Line({ label, children, onClick }: { label: string; children: ReactNode; onClick?: () => void }) {
  const inner = (
    <>
      <span className="min-w-0 truncate text-sm text-slate-600">{label}</span>
      <span className="ml-3 flex shrink-0 items-center gap-1.5 text-sm font-medium tabular-nums">{children}</span>
    </>
  );
  return onClick ? (
    <button type="button" onClick={onClick} className="-mx-2 flex w-[calc(100%+1rem)] items-center justify-between rounded-lg px-2 py-2 text-left transition-colors hover:bg-slate-50">{inner}</button>
  ) : <div className="flex items-center justify-between py-2">{inner}</div>;
}

function Hub({ index, icon: Icon, tile, title, hint, children, action, className }: {
  index: number; icon: LucideIcon; tile: keyof typeof TILE; title: string; hint: string; children: ReactNode;
  action?: { label: string; onClick?: () => void; href?: string }; className?: string;
}) {
  const cls = "mt-4 inline-flex items-center gap-1 text-sm font-semibold text-primary transition-transform hover:translate-x-0.5";
  return (
    <section className={cn("glass connector-card rise flex flex-col p-5", className)} style={{ "--i": index } as CSSProperties}>
      <div className="flex items-start gap-3">
        <span data-mark className={cn("flex size-11 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br text-white shadow-sm ring-1 ring-black/5 transition-transform duration-300", TILE[tile])}>
          <Icon className="size-5" strokeWidth={1.9} />
        </span>
        <div className="min-w-0">
          <h3 className="text-[15px] font-semibold leading-tight tracking-tight">{title}</h3>
          <p className="mt-0.5 text-xs text-muted">{hint}</p>
        </div>
      </div>
      <div className="mt-3 flex-1 divide-y divide-line/60">{children}</div>
      {action && (action.href
        ? <Link href={action.href} className={cls}>{action.label}<ArrowUpRight className="size-4" /></Link>
        : <button type="button" onClick={action.onClick} className={cn(cls, "self-start")}>{action.label}<ArrowUpRight className="size-4" /></button>)}
    </section>
  );
}

const STATE_TONE: Record<ConnectorState, Tone> = { synced: "good", waiting: "warn", failing: "bad", attention: "bad", available: "mute" };

/** The settings hub: every area at a glance, from the facts the platform already has; each card opens its section. */
export function OverviewPanel({ go, canManage, canSettings }: { go: (section: string) => void; canManage: boolean; canSettings: boolean }) {
  const t = useT();
  const tenant = useTenant();
  const plan = usePlan();
  const projects = useProjects();
  const members = useMembers();
  const keys = useApiKeys();
  const budgets = useBudgets();
  const conns = useConnections();
  const clusters = useK8sClusters();
  const dests = useDestinations();
  const grid = useRegions();
  const methodology = useMethodologyVersions();
  const policies = useQueries({
    queries: (projects.data ?? []).map((p) => ({ queryKey: ["policy", p.id], queryFn: () => api(`projects/${p.id}/policy`, policy) })),
  }).map((q) => q.data).filter((p): p is z.infer<typeof policy> => !!p);

  const facts = { connections: conns.data ?? [], clusters: clusters.data ?? [], destinations: dests.data ?? [], gridReadings: (grid.data ?? []).map((r) => r.at) };
  const stateLabel = (s: ConnectorState) => (s === "available" ? t("connection.state.none") : t(`connection.state.${s}`));
  const nProjects = projects.data?.length ?? 0;
  const withRegions = policies.filter((p) => p.allowed_regions.length > 0).length;
  const withLimit = policies.filter((p) => p.alert_carbon_monthly_kg != null).length;
  const withGate = policies.filter((p) => p.ci_carbon_block_kg != null || p.ci_cost_block != null).length;
  const autoOn = policies.filter((p) => p.auto_apply_types.length > 0).length;
  const budgetList = budgets.data ?? [];
  const budgetBad = budgetList.filter((b) => b.state !== "ok").length;
  const activeKeys = (keys.data ?? []).filter((k) => !k.revoked_at).length;
  const grd = connectorSummary("electricity-maps", facts);
  const methodVersion = methodology.data?.find((m) => m.active)?.version;
  const load = (done: boolean, node: ReactNode) => (done ? node : <Skeleton className="h-4 w-16" />);

  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      <Hub index={0} icon={Building2} tile="blue" title={t("settings.hub.org")} hint={t("settings.hub.org_hint")}
        action={canSettings ? { label: t("settings.hub.open_general"), onClick: () => go("organization") } : undefined}>
        <Line label={t("settings.hub.name")}>{load(!tenant.isLoading, <span className="max-w-[12rem] truncate">{tenant.data?.name ?? "—"}</span>)}</Line>
        <Line label={t("settings.hub.plan")} onClick={canSettings ? () => go("plan") : undefined}>
          {load(!plan.isLoading, <>{plan.data?.name ?? "—"}{plan.data && plan.data.status !== "none" && <Status tone={plan.data.status === "active" || plan.data.status === "trialing" ? "good" : "warn"}>{plan.data.status}</Status>}</>)}
        </Line>
        <Line label={t("settings.hub.projects")} onClick={() => go("projects")}>{load(!projects.isLoading, nProjects)}</Line>
        <Line label={t("settings.hub.since")}>{load(!tenant.isLoading, tenant.data ? timeAgo(tenant.data.created_at) : "—")}</Line>
      </Hub>

      <Hub index={1} icon={Cloud} tile="emerald" title={t("settings.hub.clouds")} hint={t("settings.hub.clouds_hint")} action={{ label: t("settings.hub.all_connectors"), href: "/connectors" }}>
        {(["aws", "gcp", "azure", "kubernetes"] as const).map((id) => {
          const s = connectorSummary(id, facts);
          return (
            <Line key={id} label={{ aws: "AWS", gcp: "Google Cloud", azure: "Azure", kubernetes: "Kubernetes" }[id]}>
              {load(!conns.isLoading, <>{s.count > 0 && <span className="text-xs font-normal text-muted">×{s.count}</span>}<Status tone={STATE_TONE[s.state]}>{stateLabel(s.state)}</Status></>)}
            </Line>
          );
        })}
      </Hub>

      <Hub index={2} icon={Leaf} tile="emerald" title={t("settings.hub.carbon")} hint={t("settings.hub.carbon_hint")}>
        <Line label="Electricity Maps">
          {load(!grid.isLoading, <><Status tone={STATE_TONE[grd.state]}>{grd.state === "synced" ? t("settings.hub.live") : grd.state === "waiting" ? t("settings.hub.stale") : t("connection.state.none")}</Status>
            {grd.lastAt && <span className="text-xs font-normal text-muted">{timeAgo(grd.lastAt)}</span>}</>)}
        </Line>
        <Line label={t("settings.hub.regions")}>{load(!grid.isLoading, grd.count)}</Line>
        <Line label={t("settings.hub.methodology")}>{load(!methodology.isLoading, methodVersion ?? "—")}</Line>
      </Hub>

      <Hub index={3} icon={SlidersHorizontal} tile="violet" title={t("settings.hub.policy")} hint={t("settings.hub.policy_hint")}
        action={{ label: t("settings.hub.manage_policies"), onClick: () => go("projects") }}>
        <Line label={t("settings.hub.budgets")}>
          {load(!budgets.isLoading, budgets.isError ? <span className="text-muted">—</span> : <>{budgetList.length}{budgetBad > 0 && <Status tone="bad">{t("settings.hub.over", { count: budgetBad })}</Status>}</>)}
        </Line>
        <Line label={t("settings.hub.residency")}>{load(policies.length === nProjects, <>{withRegions}/{nProjects}</>)}</Line>
        <Line label={t("settings.hub.carbon_limit")}>{load(policies.length === nProjects, <>{withLimit}/{nProjects}</>)}</Line>
        <Line label={t("settings.hub.ci_gate")}>{load(policies.length === nProjects, <>{withGate}/{nProjects}</>)}</Line>
      </Hub>

      <Hub index={4} icon={Workflow} tile="amber" title={t("settings.hub.automation")} hint={t("settings.hub.automation_hint")}
        action={{ label: t("settings.hub.manage_policies"), onClick: () => go("projects") }}>
        <Line label={t("settings.hub.auto_apply")}>{load(policies.length === nProjects, <Status tone={autoOn > 0 ? "warn" : "good"}>{t("settings.hub.projects_count", { count: autoOn })}</Status>)}</Line>
        <Line label={t("settings.hub.approval")}>{load(policies.length === nProjects, <Status tone="good">{t("settings.hub.projects_count", { count: nProjects - autoOn })}</Status>)}</Line>
        <p className="py-2 text-xs text-muted">{t("settings.hub.automation_note")}</p>
      </Hub>

      <Hub index={5} icon={Bell} tile="rose" title={t("settings.hub.notifications")} hint={t("settings.hub.notifications_hint")}
        action={{ label: t("settings.hub.manage_notifications"), onClick: () => go("notifications") }}>
        {(["email", "slack", "webhook"] as const).map((k) => {
          const s = connectorSummary(k, facts);
          return (
            <Line key={k} label={{ email: "E-mail", slack: "Slack", webhook: t("settings.hub.webhook") }[k]}>
              {load(!dests.isLoading, <Status tone={STATE_TONE[s.state]}>{s.state === "synced" ? t("settings.hub.on") : s.state === "waiting" ? t("settings.hub.paused") : t("settings.hub.not_set")}</Status>)}
            </Line>
          );
        })}
      </Hub>

      {canManage && (
        <Hub index={6} icon={ShieldCheck} tile="slate" title={t("settings.hub.security")} hint={t("settings.hub.security_hint")} className="md:col-span-2 xl:col-span-3">
          <div className="grid gap-x-8 sm:grid-cols-3 sm:divide-x sm:divide-line/60 [&>*]:sm:pl-6 first:[&>*]:sm:pl-0">
            <Line label={t("settings.hub.members")} onClick={() => go("members")}>{load(!members.isLoading, members.data?.length ?? "—")}</Line>
            <Line label={t("settings.hub.api_keys")} onClick={() => go("keys")}>{load(!keys.isLoading, activeKeys)}</Line>
            <Line label={t("settings.hub.history")} onClick={() => go("history")}><ArrowUpRight className="size-4 text-muted" /></Line>
          </div>
        </Hub>
      )}
    </div>
  );
}
