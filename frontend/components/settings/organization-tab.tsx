"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  BarChart3, Bell, BellRing, Building2, ChevronRight, ClipboardList, Cloud, Clock, GitBranch, Globe2, Hash, KeyRound, Leaf, Lock, Mail,
  Minimize2, Settings, Settings2, ShieldCheck, Target, Users, Webhook, type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { z } from "zod";
import { Switch } from "@/components/operator/ui";
import { Skeleton } from "@/components/ui/skeleton";
import { useApiKeys, useDestinations, useMembers, useRoles, useTenant } from "@/hooks/use-admin";
import { useAlerts, useBudgets, useCarbonSummary, useConnections, useK8sClusters, useProjects, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { settingsView, tenantInfo } from "@/lib/api/schemas";
import { connectorSummary } from "@/lib/connectors";
import { formatCarbon, formatCost, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { BrandLogo } from "./brand-logos";

const policy = z.object({
  allowed_regions: z.array(z.string()), ci_carbon_block_kg: z.number().nullable(), ci_cost_block: z.number().nullable(),
  auto_apply_types: z.array(z.string()).default([]),
});
const GREEN = "text-emerald-600";

/** The card of the design: an outlined green icon, a bold title with its one-line purpose, then rows. */
export function Panel({ index = 0, icon: Icon, title, hint, children, foot, action, className }: {
  index?: number; icon: LucideIcon; title: string; hint?: string; children: ReactNode; foot?: ReactNode; action?: ReactNode; className?: string;
}) {
  return (
    <section className={cn("rise lift flex flex-col rounded-xl border border-line bg-white p-5 shadow-[0_1px_2px_rgba(16,24,40,0.04)]", className)} style={{ "--i": index } as CSSProperties}>
      <div className="flex items-start gap-3">
        <Icon className={cn("mt-0.5 size-7 shrink-0", GREEN)} strokeWidth={1.5} />
        <div className="min-w-0 flex-1">
          <h3 className="text-[14px] font-semibold leading-tight text-ink">{title}</h3>
          {hint && <p className="mt-0.5 text-xs leading-snug text-muted">{hint}</p>}
        </div>
        {action}
      </div>
      <div className="mt-3 flex-1">{children}</div>
      {foot && <div className="mt-3">{foot}</div>}
    </section>
  );
}

export function FootLink({ children, href, onClick }: { children: ReactNode; href?: string; onClick?: () => void }) {
  const cls = cn("group inline-flex items-center gap-1 text-xs font-semibold hover:underline", GREEN);
  const arrow = <span aria-hidden className="transition-transform duration-200 group-hover:translate-x-0.5">→</span>;
  return href ? <Link href={href} className={cls}>{children}{arrow}</Link> : <button type="button" onClick={onClick} className={cls}>{children}{arrow}</button>;
}

/** An icon, a label, a value on the right and a chevron when the row opens something. */
export function Row({ icon: Icon, label, value, onClick, href, toggle, last }: {
  icon: LucideIcon; label: string; value?: ReactNode; onClick?: () => void; href?: string; toggle?: ReactNode; last?: boolean;
}) {
  const inner = (
    <>
      <Icon className="size-4 shrink-0 text-slate-500" strokeWidth={1.6} />
      <span className="min-w-0 flex-1 truncate text-[13px] text-slate-700">{label}</span>
      {value !== undefined && <span className="shrink-0 text-xs tabular-nums text-muted">{value}</span>}
      {toggle}
      {(onClick || href) && <ChevronRight className="size-4 shrink-0 text-slate-400 transition-transform duration-200 group-hover:translate-x-0.5" />}
    </>
  );
  const base = cn("group flex w-full items-center gap-3 border-t border-line/70 py-2.5 text-left", last && "")
  if (href) return <Link href={href} className={cn(base, "transition-colors hover:bg-slate-50/70")}>{inner}</Link>;
  if (onClick) return <button type="button" onClick={onClick} className={cn(base, "transition-colors hover:bg-slate-50/70")}>{inner}</button>;
  return <div className={base}>{inner}</div>;
}

export function Status({ ok, children }: { ok: boolean | null; children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-xs">
      <span className={cn("size-1.5 rounded-full", ok === true ? "bg-emerald-500" : "bg-slate-300")} />
      <span className={ok === true ? "text-emerald-600" : "text-muted"}>{children}</span>
    </span>
  );
}

const field = "w-full rounded-lg border border-line bg-white px-3 py-2 text-sm outline-none transition-shadow focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/20 disabled:bg-slate-50 disabled:text-slate-500";

/** The Organization tab: every area of the organization in the order and shape of the design, filled from real data. */
export function OrganizationTab({ go, canManage, canEdit }: { go: (tab: string) => void; canManage: boolean; canEdit: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const tenant = useTenant();
  const conns = useConnections();
  const clusters = useK8sClusters();
  const dests = useDestinations();
  const grid = useRegions();
  const summary = useCarbonSummary();
  const projects = useProjects();
  const budgets = useBudgets();
  const alerts = useAlerts("new");
  const members = useMembers();
  const keys = useApiKeys();
  const roles = useRoles();
  const settings = useQuery({ queryKey: ["settings", "settings"], queryFn: () => api("settings", settingsView) });
  const policies = (projects.data ?? []).map((p) => p.id);
  const pols = useQuery({
    queryKey: ["policies-all", policies.join(",")],
    enabled: policies.length > 0,
    queryFn: async () => Promise.all(policies.map((id) => api(`projects/${id}/policy`, policy))),
  });

  const facts = { connections: conns.data ?? [], clusters: clusters.data ?? [], destinations: dests.data ?? [], gridReadings: (grid.data ?? []).map((r) => r.at) };
  const setting = (k: string) => settings.data?.items.find((i) => i.key === k);

  // ---- Organization details: the name is read-only (no rename in the API); the reporting currency is a real setting
  const currency = setting("finops.reporting_currency");
  const [cur, setCur] = useState("");
  const [nm, setNm] = useState("");
  useEffect(() => { if (currency) setCur(String(currency.value)); }, [currency]);
  useEffect(() => { if (tenant.data) setNm(tenant.data.name); }, [tenant.data]);
  const nameChanged = !!tenant.data && nm.trim() !== tenant.data.name;
  const currencyChanged = !!currency && cur !== String(currency.value);
  const save = useMutation({
    mutationFn: async () => {
      if (nameChanged) await api("tenant", tenantInfo, undefined, { method: "PATCH", body: JSON.stringify({ name: nm.trim() }) });
      if (currencyChanged) return api("settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values: { "finops.reporting_currency": cur } }) });
      return null;
    },
    onSuccess: (v) => {
      if (v) qc.setQueryData(["settings", "settings"], v);
      void qc.invalidateQueries({ queryKey: ["tenant"] });
      void qc.invalidateQueries({ queryKey: ["finops-summary"] });
    },
  });

  // ---- channels: the switch enables or disables every destination of that kind
  const toggle = useMutation({
    mutationFn: async (v: { kind: string; on: boolean }) => {
      const list = (dests.data ?? []).filter((d) => d.kind === v.kind);
      await Promise.all(list.map((d) => api(`notifications/destinations/${d.id}/${v.on ? "enable" : "disable"}`, z.unknown(), undefined, { method: "POST", body: "{}" })));
    },
    onSettled: () => { void qc.invalidateQueries({ queryKey: ["notif-destinations"] }); },
  });
  const channel = (kind: string) => {
    const list = (dests.data ?? []).filter((d) => d.kind === kind);
    return { list, on: list.some((d) => d.enabled) };
  };

  const budgetTop = (budgets.data ?? []).find((b) => b.project_id === null && b.period === "monthly") ?? (budgets.data ?? [])[0];
  const carbonLimit = setting("alerts.carbon_monthly_kg");
  const warn = setting("alerts.warn_pct");
  const withRegions = (pols.data ?? []).filter((p) => p.allowed_regions.length > 0).length;
  const autoOn = (pols.data ?? []).filter((p) => p.auto_apply_types.length > 0).length;
  const gates = (pols.data ?? []).filter((p) => p.ci_carbon_block_kg != null || p.ci_cost_block != null).length;
  const nProj = projects.data?.length ?? 0;
  const origins = summary.data?.intensity_origins ?? [];
  const used = (name: string) => origins.some((o) => o.source.toLowerCase().includes(name));
  const grd = connectorSummary("electricity-maps", facts);
  const k8s = connectorSummary("kubernetes", facts);
  const activeKeys = (keys.data ?? []).filter((k) => !k.revoked_at).length;
  const customRoles = roles.data?.custom.length ?? 0;

  const cloud = (id: "aws" | "azure" | "gcp" | "kubernetes", name: string) => {
    const s = connectorSummary(id, facts);
    const list = (conns.data ?? []).filter((c) => c.provider === id);
    const connected = s.state !== "available";
    return (
      <Link key={id} href="/connectors" className="group flex items-center gap-3 border-t border-line/70 py-2.5 transition-colors hover:bg-slate-50/70">
        <BrandLogo id={id} className="size-9 shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block text-[13px] font-medium text-ink">{name}</span>
          <span className="block truncate text-[11px] text-muted">
            {!connected ? t("settings.org.connect_hint", { name })
              : id === "kubernetes" ? `${t("connectors.k8s.clusters", { count: s.count })}${s.lastAt ? ` · ${timeAgo(s.lastAt)}` : ""}`
              : `${t("settings.org.accounts", { count: list.length })}${s.lastAt ? ` · ${t("settings.org.last_sync", { ago: timeAgo(s.lastAt) })}` : ""}`}
          </span>
        </span>
        <Status ok={connected ? s.state === "synced" : null}>{connected ? t(`connection.state.${s.state}`) : t("connection.state.none")}</Status>
        <ChevronRight className="size-4 text-slate-400 transition-transform duration-200 group-hover:translate-x-0.5" />
      </Link>
    );
  };

  const source = (id: "watttime" | "ember" | "entsoe", name: string, desc: string, connected: boolean | null) => (
    <div key={id} className="flex items-center gap-3 border-t border-line/70 py-2.5">
      <BrandLogo id={id} className="size-9 shrink-0" />
      <span className="min-w-0 flex-1">
        <span className="block text-[13px] font-medium text-ink">{name}</span>
        <span className="block truncate text-[11px] text-muted">{desc}</span>
      </span>
      <Status ok={connected}>{connected === null ? t("settings.org.planned") : connected ? t("connection.state.synced") : t("connection.state.none")}</Status>
    </div>
  );

  const sw = (kind: string, label: string, icon: LucideIcon) => {
    const c = channel(kind);
    return (
      <Row key={kind} icon={icon} label={label}
        value={dests.isLoading ? "…" : c.list.length === 0 ? t("settings.hub.not_set") : c.on ? t("settings.hub.on") : t("settings.hub.paused")}
        toggle={<Switch label={label} size="sm" checked={c.on} disabled={!canEdit || c.list.length === 0 || toggle.isPending} onChange={(on) => toggle.mutate({ kind, on })} />} />
    );
  };

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Panel index={0} icon={Building2} title={t("settings.org.details")} hint={t("settings.org.details_hint")}>
        <div className="space-y-3">
          <label className="block">
            <span className="mb-1 block text-xs text-slate-600">{t("settings.org.name")}</span>
            {tenant.isLoading ? <Skeleton className="h-9" /> : <input className={field} value={nm} maxLength={100} disabled={!canManage} onChange={(e) => setNm(e.target.value)} />}
          </label>
          <div className="grid grid-cols-2 gap-3">
            <label className="block">
              <span className="mb-1 block text-xs text-slate-600">{t("settings.org.created")}</span>
              <input className={field} readOnly value={tenant.data ? new Date(tenant.data.created_at).toLocaleDateString() : ""} />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs text-slate-600">{t("settings.org.currency")}</span>
              <select className={field} value={cur} disabled={!canEdit || !currency?.editable || save.isPending} onChange={(e) => setCur(e.target.value)}>
                {(currency?.enum ?? [cur]).map((o) => <option key={o} value={o}>{o}</option>)}
              </select>
            </label>
          </div>
          <div className="flex items-center justify-end gap-3 pt-1">
            {save.isSuccess && <span role="status" className="text-xs text-emerald-600">{t("settings.org.saved")}</span>}
            {save.isError && <span role="alert" className="text-xs text-bad">{save.error.message}</span>}
            <button type="button" onClick={() => save.mutate()} disabled={!canEdit || (!nameChanged && !currencyChanged) || nm.trim().length < 2 || save.isPending}
              className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-all hover:bg-emerald-700 active:scale-[0.98] disabled:opacity-50">
              {save.isPending ? t("settings.org.saving") : t("settings.org.save")}
            </button>
          </div>
        </div>
      </Panel>

      <Panel index={1} icon={Cloud} title={t("settings.org.clouds")} hint={t("settings.org.clouds_hint")} foot={<FootLink href="/connectors">{t("settings.org.all_connectors")}</FootLink>}>
        {cloud("aws", "AWS")}{cloud("azure", "Azure")}{cloud("gcp", "GCP")}{cloud("kubernetes", "Kubernetes")}
      </Panel>

      <Panel index={2} icon={Leaf} title={t("settings.org.sources")} hint={t("settings.org.sources_hint")} foot={<FootLink href="/carbon">{t("settings.org.all_sources")}</FootLink>}>
        {source("watttime", "WattTime", t("settings.org.watttime"), null)}
        {source("ember", "Ember", t("settings.org.ember"), used("ember") || grd.state !== "available" ? true : false)}
        {source("entsoe", "ENTSO-E", t("settings.org.entsoe"), used("entso") || used("energy-charts") || used("energy_charts") ? true : false)}
      </Panel>

      <Panel index={3} icon={ShieldCheck} title={t("settings.org.policy")} hint={t("settings.org.policy_hint")} foot={<FootLink onClick={() => go("policies")}>{t("settings.hub.manage_policies")}</FootLink>}>
        <Row icon={BarChart3} label={t("settings.org.monthly_budget")} href="/projects"
          value={budgets.isLoading ? "…" : budgetTop ? formatCost(budgetTop.amount, budgetTop.currency) : t("settings.org.not_set")} />
        <Row icon={Target} label={t("settings.org.carbon_budget")} onClick={() => go("data")}
          value={settings.isLoading ? "…" : typeof carbonLimit?.value === "number" && carbonLimit.value > 0 ? formatCarbon(carbonLimit.value) : t("settings.org.no_limit")} />
        <Row icon={BellRing} label={t("settings.org.alert_thresholds")} onClick={() => go("data")}
          value={settings.isLoading ? "…" : warn ? `${String(warn.value)}%` : t("settings.org.configured")} />
        <Row icon={Settings2} label={t("settings.org.optimization")} onClick={() => go("policies")}
          value={pols.isLoading ? "…" : t("settings.org.of_projects", { n: withRegions, total: nProj })} />
      </Panel>

      <Panel index={4} icon={Settings} title={t("settings.org.automation")} hint={t("settings.org.automation_hint")} foot={<FootLink onClick={() => go("automation")}>{t("settings.org.configure_automation")}</FootLink>}>
        <Row icon={Globe2} label={t("settings.auto.region_moves")} onClick={() => go("automation")} value={t("settings.auto.needs_person")} />
        <Row icon={Minimize2} label={t("settings.auto.rightsizing")} onClick={() => go("automation")}
          value={pols.isLoading ? "…" : autoOn > 0 ? t("settings.hub.projects_count", { count: autoOn }) : t("settings.auto.approval")} />
        <Row icon={GitBranch} label={t("settings.org.ci_gate")} onClick={() => go("policies")} value={pols.isLoading ? "…" : t("settings.org.of_projects", { n: gates, total: nProj })} />
        <Row icon={ShieldCheck} label={t("settings.org.approval_required")} value={t("settings.org.always")} />
      </Panel>

      <Panel index={5} icon={Bell} title={t("settings.hub.notifications")} hint={t("settings.hub.notifications_hint")} foot={<FootLink onClick={() => go("notifications")}>{t("settings.hub.manage_notifications")}</FootLink>}>
        {sw("email", t("settings.org.email"), Mail)}
        {sw("slack", t("settings.org.slack"), Hash)}
        {sw("webhook", t("settings.org.webhook"), Webhook)}
        <Row icon={BellRing} label={t("settings.org.alerts_open")} href="/alerts" value={alerts.isLoading ? "…" : t("settings.org.alerts_count", { count: alerts.data?.length ?? 0 })} />
      </Panel>

      <Panel index={6} icon={Lock} title={t("settings.hub.security")} hint={t("settings.org.security_hint")} className="lg:col-span-3"
        foot={canManage ? <div className="text-right"><FootLink onClick={() => go("security")}>{t("settings.org.all_security")}</FootLink></div> : undefined}>
        <div className="grid gap-x-8 md:grid-cols-3 md:divide-x md:divide-line/70 [&>div]:md:pl-8 first:[&>div]:md:pl-0">
          <div>
            <Row icon={Users} label={t("settings.hub.members")} onClick={canManage ? () => go("users") : undefined} value={members.isLoading ? "…" : t("settings.org.members_count", { count: members.data?.length ?? 0 })} />
            <Row icon={ShieldCheck} label={t("settings.org.roles")} onClick={canManage ? () => go("users") : undefined} value={roles.isLoading ? "…" : customRoles > 0 ? t("settings.org.custom_roles", { count: customRoles }) : t("settings.org.configured")} />
          </div>
          <div>
            <Row icon={KeyRound} label={t("settings.org.keys")} onClick={canManage ? () => go("security") : undefined} value={keys.isLoading ? "…" : t("settings.org.keys_count", { count: activeKeys })} />
            <Row icon={ClipboardList} label={t("settings.org.audit")} onClick={canManage ? () => go("security") : undefined} value={t("settings.org.enabled")} />
          </div>
          <div>
            <Row icon={ShieldCheck} label={t("settings.org.mfa")} value={t("settings.org.by_provider")} />
            <Row icon={Clock} label={t("settings.org.session")} value={t("settings.org.by_provider")} />
          </div>
        </div>
      </Panel>
    </div>
  );
}
