"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import { IconBell as Bell, IconBell as BellRing, IconBranch as GitBranch, IconBuilding as Building2, IconChart as BarChart3, IconChevronRight as ChevronRight, IconClipboard as ClipboardList, IconClock as Clock, IconCloud as Cloud, IconGauge as Minimize2, IconGlobe as Globe2, IconKey as KeyRound, IconLeaf as Leaf, IconLock as Lock, IconMail as Mail, IconPlug as Webhook, IconSend as Hash, IconShield as ShieldCheck, IconSliders as Settings, IconSliders as Settings2, IconTarget as Target, IconUsers as Users } from "@/components/ui/icons";
import Link from "next/link";
import { useEffect, useState, type CSSProperties, type ReactNode } from "react";
import { z } from "zod";
import { Skeleton } from "@/components/ui/skeleton";
import { useApiKeys, useDestinations, useMembers, useNotificationsEnabled, useRoles, useTenant } from "@/hooks/use-admin";
import { useAlerts, useBudgets, useCarbonSummary, useConnections, useGridSources, useK8sClusters, useProjects, useRegions } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { apiVoid } from "@/lib/api/client";
import { budget, settingsView, tenantInfo } from "@/lib/api/schemas";
import { connectorSummary } from "@/lib/connectors";
import { formatCarbon, formatCost, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { BrandLogo } from "./brand-logos";
import type { IconType } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";

const policy = z.object({
  allowed_regions: z.array(z.string()), ci_carbon_block_kg: z.number().nullable(), ci_cost_block: z.number().nullable(),
  auto_apply_types: z.array(z.string()).default([]),
});
const GREEN = "text-teal-700";

/** The card of the design: an outlined green icon, a bold title with its one-line purpose, then rows. */
export function Panel({ index = 0, icon: Icon, title, hint, info, children, foot, action, className }: {
  index?: number; icon: LucideIcon | IconType; title: string; hint?: string; info?: ReactNode; children: ReactNode; foot?: ReactNode; action?: ReactNode; className?: string;
}) {
  return (
    <section className={cn("rise lift flex flex-col rounded-xl border border-line bg-white p-5 shadow-[0_1px_2px_rgba(16,24,40,0.04)]", className)} style={{ "--i": index } as CSSProperties}>
      <div className="flex items-start gap-3">
        <Icon className={cn("mt-0.5 size-7 shrink-0", GREEN)} strokeWidth={1.5} />
        <div className="min-w-0 flex-1">
          <h3 className="flex items-center gap-1.5 text-[14px] font-semibold leading-tight text-ink">{title}{info && <Info>{info}</Info>}</h3>
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
      <span className={ok === true ? "text-teal-700" : "text-muted"}>{children}</span>
    </span>
  );
}

const field = "w-full rounded-lg border border-line bg-white px-3 py-2 text-sm outline-none transition-shadow focus:border-teal-600 focus:ring-2 focus:ring-teal-600/20 disabled:bg-slate-50 disabled:text-slate-500";

/** A row whose value is edited in place: click it, change the number, save. The editor opens under the row. */
function EditableRow({ icon, label, display, onSave, initial, unit, hint, min, max, canEdit, saving, error, onOpen }: {
  icon: LucideIcon; label: string; display: ReactNode; initial: number; unit?: string; hint?: string; min?: number; max?: number;
  canEdit: boolean; saving?: boolean; error?: string | null; onSave: (v: number) => Promise<unknown> | void; onOpen?: () => void;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [v, setV] = useState(String(initial));
  useEffect(() => { if (!open) setV(String(initial)); }, [initial, open]);
  const n = Number(v.replace(",", "."));
  const valid = v.trim() !== "" && Number.isFinite(n) && (min === undefined || n >= min) && (max === undefined || n <= max);
  return (
    <div className="border-t border-line/70">
      <button type="button" disabled={!canEdit} onClick={() => { setOpen((o) => !o); onOpen?.(); }} aria-expanded={open}
        className="group flex w-full items-center gap-3 py-2.5 text-left transition-colors hover:bg-slate-50/70 disabled:cursor-default disabled:hover:bg-transparent">
        {(() => { const Icon = icon; return <Icon className="size-4 shrink-0 text-slate-500" strokeWidth={1.6} />; })()}
        <span className="min-w-0 flex-1 truncate text-[13px] text-slate-700">{label}</span>
        <span className="shrink-0 text-xs tabular-nums text-muted">{display}</span>
        {canEdit && <ChevronRight className={cn("size-4 shrink-0 text-slate-400 transition-transform duration-300", open && "rotate-90")} />}
      </button>
      <div className="expand" data-open={open}>
        <div>
          <div className="pb-3">
            {hint && <p className="mb-2 text-xs text-muted">{hint}</p>}
            <div className="flex items-center gap-2">
              <input value={v} onChange={(e) => setV(e.target.value)} inputMode="decimal" aria-label={label}
                className="w-32 rounded-lg border border-line bg-white px-3 py-1.5 text-sm outline-none focus:border-teal-600 focus:ring-2 focus:ring-teal-600/20" />
              {unit && <span className="text-xs text-muted">{unit}</span>}
              <button type="button" disabled={!valid || saving}
                onClick={async () => { await onSave(n); setOpen(false); }}
                className="ml-auto rounded-lg bg-teal-700 px-3 py-1.5 text-xs font-semibold text-white transition-all hover:bg-teal-800 active:scale-[0.98] disabled:opacity-50">{saving ? t("settings.org.saving") : t("settings.org.save")}</button>
            </div>
            {error && <p role="alert" className="mt-1 text-xs text-bad">{error}</p>}
          </div>
        </div>
      </div>
    </div>
  );
}

/** The sources of grid carbon intensity as the platform has them: how many regions each covers and how fresh it is. */
const GRID_NAME: Record<string, { name: string; desc: string; fresh: number }> = {
  electricitymaps: { name: "Electricity Maps", desc: "Real-time grid intensity worldwide", fresh: 36 },
  eia: { name: "EIA", desc: "United States, from the generation mix", fresh: 72 },
  energy_charts: { name: "Energy-Charts", desc: "Europe, from the generation mix", fresh: 72 },
  uk_carbon_intensity: { name: "UK Carbon Intensity", desc: "Great Britain, measured and forecast", fresh: 36 },
  ember: { name: "Ember", desc: "Monthly generation data, worldwide", fresh: 24 * 50 },
};

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
  const gridSrc = useGridSources();
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

  // ---- policy rows edited in place
  const patchSetting = useMutation({
    mutationFn: (v: { key: string; value: number }) => api("settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values: { [v.key]: v.value } }) }),
    onSuccess: (view) => qc.setQueryData(["settings", "settings"], view),
  });
  const monthlyBudget = useMutation({
    mutationFn: async (amount: number) => {
      const cur = (budgets.data ?? []).find((b) => b.project_id === null && b.period === "monthly");
      if (cur) await apiVoid(`finops/budgets/${cur.id}`, { method: "DELETE" });
      return api("finops/budgets", budget, undefined, {
        method: "POST", body: JSON.stringify({ project_id: null, amount, currency: cur?.currency ?? (currency ? String(currency.value) === "auto" ? "USD" : String(currency.value) : "USD"), period: "monthly" }),
      });
    },
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["budgets"] }); void qc.invalidateQueries({ queryKey: ["alerts"] }); },
  });

  // ---- channels: shown as a state, never as a second switch (the switch is on the Notifications tab)
  const notifOn = useNotificationsEnabled();
  const channel = (kind: string) => {
    const list = (dests.data ?? []).filter((d) => d.kind === kind);
    return { list, on: list.some((d) => d.enabled) };
  };

  const budgetTop = (budgets.data ?? []).find((b) => b.project_id === null && b.period === "monthly") ?? (budgets.data ?? [])[0];
  const carbonLimit = setting("alerts.carbon_monthly_kg");
  const warn = setting("alerts.warn_pct");
  const withRegions = (pols.data ?? []).filter((p) => p.allowed_regions.length > 0).length;
  const autoOn = (pols.data ?? []).filter((p) => p.auto_apply_types.includes("time_shift")).length;
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
      <Link key={id} href={connected ? "/connectors" : id === "kubernetes" ? "/kubernetes" : `/connectors?connect=${id}`} className="group flex items-center gap-3 border-t border-line/70 py-2.5 transition-colors hover:bg-slate-50/70">
        <BrandLogo id={id} className="size-9 shrink-0" />
        <span className="min-w-0 flex-1">
          <span className="block text-[13px] font-medium text-ink">{name}</span>
          <span className="block truncate text-[11px] text-muted">
            {!connected ? (id === "kubernetes" ? t("settings.org.k8s_hint") : t("settings.org.connect_hint", { name }))
              : id === "kubernetes" ? `${t("connectors.k8s.clusters", { count: s.count })}${s.lastAt ? ` · ${timeAgo(s.lastAt)}` : ""}`
              : `${t("settings.org.accounts", { count: list.length })}${s.lastAt ? ` · ${t("settings.org.last_sync", { ago: timeAgo(s.lastAt) })}` : ""}`}
          </span>
        </span>
        <Status ok={connected ? s.state === "synced" : null}>{connected ? t(`connection.state.${s.state}`) : t("connection.state.none")}</Status>
        <ChevronRight className="size-4 text-slate-400 transition-transform duration-200 group-hover:translate-x-0.5" />
      </Link>
    );
  };

  const sourceRow = (provider: string, lastAt: string, regions: number) => {
    const meta = GRID_NAME[provider] ?? { name: provider, desc: "", fresh: 72 };
    const live = Date.now() - Date.parse(lastAt) <= meta.fresh * 3600_000;
    const share = origins.filter((o) => o.source === provider).reduce((n, o) => n + o.share_pct, 0);
    return (
      <div key={provider} className="flex items-center gap-3 border-t border-line/70 py-2.5">
        <span aria-hidden className="grid size-9 shrink-0 place-items-center rounded-lg bg-teal-50 text-xs font-bold text-teal-800">{meta.name.slice(0, 2).toUpperCase()}</span>
        <span className="min-w-0 flex-1">
          <span className="block text-[13px] font-medium text-ink">{meta.name}</span>
          <span className="block truncate text-[11px] text-muted">
            {t("settings.org.regions", { count: regions })} · {timeAgo(lastAt)}{share > 0 ? ` · ${t("settings.org.in_use", { pct: Math.round(share) })}` : ""}
          </span>
        </span>
        <Status ok={live}>{live ? t("settings.hub.live") : t("settings.hub.stale")}</Status>
      </div>
    );
  };

  const sw = (kind: string, label: string, icon: LucideIcon) => {
    const c = channel(kind);
    const state = dests.isLoading || notifOn === undefined ? "…" : c.list.length === 0 ? t("settings.hub.not_set") : notifOn === false ? t("settings.notif.all_off") : c.on ? t("settings.hub.on") : t("settings.hub.paused");
    return <Row key={kind} icon={icon} label={label} value={state} onClick={() => go("notifications")} />;
  };

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Panel index={0} icon={Building2} title={t("settings.org.details")} hint={t("settings.org.details_hint")} info={t("info.org_details")}>
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
            {save.isSuccess && <span role="status" className="text-xs text-teal-700">{t("settings.org.saved")}</span>}
            {save.isError && <span role="alert" className="text-xs text-bad">{save.error.message}</span>}
            <button type="button" onClick={() => save.mutate()} disabled={!canEdit || (!nameChanged && !currencyChanged) || nm.trim().length < 2 || save.isPending}
              className="rounded-lg bg-teal-700 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-all hover:bg-teal-800 active:scale-[0.98] disabled:opacity-50">
              {save.isPending ? t("settings.org.saving") : t("settings.org.save")}
            </button>
          </div>
        </div>
      </Panel>

      <Panel index={1} icon={Cloud} title={t("settings.org.clouds")} hint={t("settings.org.clouds_hint")} info={t("info.org_clouds")} foot={<FootLink href="/connectors">{t("settings.org.all_connectors")}</FootLink>}>
        {cloud("aws", "AWS")}{cloud("azure", "Azure")}{cloud("gcp", "GCP")}{cloud("kubernetes", "Kubernetes")}
      </Panel>

      <Panel index={2} icon={Leaf} title={t("settings.org.sources")} hint={t("settings.org.sources_hint")} info={t("info.org_sources")} foot={<FootLink href="/carbon">{t("settings.org.all_sources")}</FootLink>}>
        {gridSrc.isLoading ? <Skeleton className="my-3 h-24" /> : (gridSrc.data ?? []).length === 0
          ? <p className="border-t border-line/70 py-3 text-xs text-muted">{t("settings.org.no_sources")}</p>
          : (gridSrc.data ?? []).map((g) => sourceRow(g.provider, g.last_at, g.regions))}
        <div className="flex items-center gap-3 border-t border-line/70 py-2.5 opacity-70">
          <BrandLogo id="watttime" className="size-9 shrink-0" />
          <span className="min-w-0 flex-1"><span className="block text-[13px] font-medium text-ink">WattTime</span><span className="block truncate text-[11px] text-muted">{t("settings.org.watttime")}</span></span>
          <Status ok={null}>{t("settings.org.planned")}</Status>
        </div>
      </Panel>

      <Panel index={3} icon={ShieldCheck} title={t("settings.org.policy")} hint={t("settings.org.policy_hint")} info={t("info.org_policy")} foot={<FootLink onClick={() => go("policies")}>{t("settings.hub.manage_policies")}</FootLink>}>
        <EditableRow icon={BarChart3} label={t("settings.org.monthly_budget")} canEdit={canEdit} unit={budgetTop?.currency ?? (cur && cur !== "auto" ? cur : "USD")} min={1}
          initial={budgetTop?.amount ?? 0} saving={monthlyBudget.isPending} error={monthlyBudget.isError ? monthlyBudget.error.message : null}
          hint={t("settings.org.monthly_budget_hint")}
          display={budgets.isLoading ? "…" : budgetTop ? formatCost(budgetTop.amount, budgetTop.currency) : t("settings.org.not_set")}
          onSave={(v) => monthlyBudget.mutateAsync(v)} />
        <EditableRow icon={Target} label={t("settings.org.carbon_budget")} canEdit={canEdit} unit="kg CO₂e" min={0} max={carbonLimit?.max}
          initial={typeof carbonLimit?.value === "number" ? carbonLimit.value : 0} saving={patchSetting.isPending} error={patchSetting.isError ? patchSetting.error.message : null}
          hint={t("settings.org.carbon_budget_hint")}
          display={settings.isLoading ? "…" : typeof carbonLimit?.value === "number" && carbonLimit.value > 0 ? formatCarbon(carbonLimit.value) : t("settings.org.no_limit")}
          onSave={(v) => patchSetting.mutateAsync({ key: "alerts.carbon_monthly_kg", value: Math.round(v) })} />
        <EditableRow icon={BellRing} label={t("settings.org.alert_thresholds")} canEdit={canEdit} unit="%" min={warn?.min ?? 1} max={warn?.max ?? 100}
          initial={typeof warn?.value === "number" ? warn.value : 80} saving={patchSetting.isPending} error={patchSetting.isError ? patchSetting.error.message : null}
          hint={t("settings.org.alert_thresholds_hint")}
          display={settings.isLoading ? "…" : warn ? `${String(warn.value)}%` : t("settings.org.configured")}
          onSave={(v) => patchSetting.mutateAsync({ key: "alerts.warn_pct", value: Math.round(v) })} />
        <Row icon={Settings2} label={t("settings.org.optimization")} onClick={() => go("policies")}
          value={pols.isLoading ? "…" : t("settings.org.of_projects", { n: withRegions, total: nProj })} />
      </Panel>

      <Panel index={4} icon={Settings} title={t("settings.org.automation")} hint={t("settings.org.automation_hint")} info={t("info.org_automation")} foot={<FootLink onClick={() => go("automation")}>{t("settings.org.configure_automation")}</FootLink>}>
        <Row icon={Globe2} label={t("settings.auto.region_moves")} onClick={() => go("automation")} value={t("settings.auto.needs_person")} />
        <Row icon={Minimize2} label={t("settings.auto.rightsizing")} onClick={() => go("automation")} value={t("settings.auto.needs_person")} />
        <Row icon={Clock} label={t("settings.org.time_shift")} onClick={() => go("policies")}
          value={pols.isLoading ? "…" : autoOn > 0 ? t("settings.org.auto_in", { n: autoOn, total: nProj }) : t("settings.auto.approval")} />
        <Row icon={GitBranch} label={t("settings.org.ci_gate")} onClick={() => go("policies")} value={pols.isLoading ? "…" : t("settings.org.of_projects", { n: gates, total: nProj })} />
      </Panel>

      <Panel index={5} icon={Bell} title={t("settings.hub.notifications")} hint={t("settings.hub.notifications_hint")} info={t("info.org_notifications")} foot={<FootLink onClick={() => go("notifications")}>{t("settings.hub.manage_notifications")}</FootLink>}>
        {sw("email", t("settings.org.email"), Mail)}
        {sw("slack", t("settings.org.slack"), Hash)}
        {sw("webhook", t("settings.org.webhook"), Webhook)}
        <Row icon={BellRing} label={t("settings.org.alerts_open")} href="/alerts" value={alerts.isLoading ? "…" : t("settings.org.alerts_count", { count: alerts.data?.length ?? 0 })} />
      </Panel>

      <Panel index={6} icon={Lock} title={t("settings.hub.security")} hint={t("settings.org.security_hint")} info={t("info.org_security")} className="lg:col-span-3"
        foot={<div className="flex flex-wrap items-center justify-between gap-2"><span className="text-xs text-muted">{t("settings.org.provider_note")}</span>{canManage && <FootLink onClick={() => go("security")}>{t("settings.org.all_security")}</FootLink>}</div>}>
        <div className="grid gap-x-8 md:grid-cols-3 md:divide-x md:divide-line/70 [&>div]:md:pl-8 first:[&>div]:md:pl-0">
          <div>
            <Row icon={Users} label={t("settings.hub.members")} onClick={canManage ? () => go("users") : undefined} value={members.isLoading ? "…" : t("settings.org.members_count", { count: members.data?.length ?? 0 })} />
            <Row icon={ShieldCheck} label={t("settings.org.roles")} onClick={canManage ? () => go("users") : undefined} value={roles.isLoading ? "…" : t("settings.org.roles_count", { builtin: roles.data?.builtin.length ?? 0, custom: customRoles })} />
          </div>
          <div>
            <Row icon={KeyRound} label={t("settings.org.keys")} onClick={canManage ? () => go("security") : undefined} value={keys.isLoading ? "…" : t("settings.org.keys_count", { count: activeKeys })} />
            <Row icon={ClipboardList} label={t("settings.org.audit")} onClick={canManage ? () => go("security") : undefined} value={t("settings.org.enabled")} />
          </div>
          <div>
            <Row icon={Clock} label={t("settings.org.invite_lifetime")} onClick={canManage ? () => go("data") : undefined}
              value={settings.isLoading ? "…" : t("settings.org.days", { count: Number(setting("security.invitation_expiry_days")?.value ?? 7) })} />
            <Row icon={Mail} label={t("settings.org.invite_domains")} onClick={canManage ? () => go("data") : undefined}
              value={settings.isLoading ? "…" : ((setting("security.invitation_allowed_domains")?.value as string[] | undefined)?.length ?? 0) > 0
                ? ((setting("security.invitation_allowed_domains")?.value as string[]).join(", ")) : t("settings.org.any_domain")} />
          </div>
        </div>
      </Panel>
    </div>
  );
}
