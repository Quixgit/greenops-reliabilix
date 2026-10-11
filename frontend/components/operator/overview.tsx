"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Cpu, Power, UserPlus } from "lucide-react";
import { IconArrowRight as ArrowRight, IconBell as Bell, IconBolt as Zap, IconBuilding as Building2, IconCloud as Cloud, IconDatabase as Database, IconDatabase as Layers, IconFolder as FolderKanban, IconPlug as Plug, IconTrend as Activity, IconUsers as Users, IconWarning as AlertTriangle } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useOperatorApiUsage, useOperatorAudit, useOperatorConnections, useOperatorDataSources, useOperatorErrors, useOperatorJobs, useOperatorMe,
  useOperatorPlans, useOperatorStats, useOperatorSystem, useOperatorTenants,
} from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { settingsView, type OperatorTenant } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { describe } from "./audit";
import { PageTitle, Tile } from "./parts";
import { DataTable, Meter, Pill, Switch, type Column, type PillTone } from "./ui";

type Item = { key: string; text: string; href: string; icon: typeof Bell };

// The platform switches an operator reaches for most. Each one is the same setting as on Platform settings.
const SWITCHES: { key: string; title: string; hint: string; danger?: boolean }[] = [
  { key: "platform.maintenance_enabled", title: "Maintenance mode", hint: "Organizations can read but not change anything while you work on the platform.", danger: true },
  { key: "platform.signups_enabled", title: "Self-service sign-up", hint: "People without an organization can create one." },
  { key: "carbon.auto_load_history", title: "Load the past of a region by itself", hint: "Daily: fills gaps in grid data from every source that has the past, then recalculates." },
];

function PlatformControls({ canAct }: { canAct: boolean }) {
  const qc = useQueryClient();
  const key = ["settings", "operator/settings"];
  const q = useQuery({ queryKey: key, queryFn: () => api("operator/settings", settingsView), staleTime: 0 });
  const [confirm, setConfirm] = useState<string | null>(null);
  const save = useMutation({
    mutationFn: (v: Record<string, unknown>) => api("operator/settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values: v }) }),
    onSuccess: (view) => { qc.setQueryData(key, view); void qc.invalidateQueries({ queryKey: ["settings-history"] }); setConfirm(null); },
  });
  const value = (k: string) => q.data?.items.find((i) => i.key === k)?.value === true;
  return (
    <Card index={4} interactive={false}>
      <CardHeader title="Platform controls" icon={<Power className="size-4 text-emerald-600" />}
        action={<Link href="/operator/settings" className="text-xs font-semibold text-primary hover:underline">All settings →</Link>} />
      {q.isLoading ? <Skeleton className="h-32" /> : (
        <ul className="divide-y divide-line/70">
          {SWITCHES.map((s) => {
            const on = value(s.key);
            const risky = s.danger && !on; // turning maintenance ON locks every organization out of changes
            return (
              <li key={s.key} className="flex items-center gap-4 py-3">
                <div className="min-w-0 flex-1">
                  <p className="flex items-center gap-2 text-sm font-medium">{s.title}{s.danger && on && <Pill tone="warn" pulse>active</Pill>}</p>
                  <p className="text-xs text-muted">{s.hint}</p>
                  {confirm === s.key && (
                    <p role="alert" className="mt-2 flex flex-wrap items-center gap-2 rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-900 ring-1 ring-amber-200">
                      Every organization will be unable to make changes until you turn this off.
                      <button type="button" onClick={() => save.mutate({ [s.key]: true })} className="rounded-lg bg-amber-500 px-2.5 py-1 font-semibold text-white">Turn on</button>
                      <button type="button" onClick={() => setConfirm(null)} className="font-medium underline">Cancel</button>
                    </p>
                  )}
                </div>
                <Switch label={s.title} checked={on} disabled={!canAct || save.isPending}
                  onChange={(v) => (risky && v ? setConfirm(s.key) : save.mutate({ [s.key]: v }))} />
              </li>
            );
          })}
        </ul>
      )}
      {!canAct && <p className="mt-2 text-xs text-muted">Read-only for your role.</p>}
      {save.isError && <p role="alert" className="mt-2 text-xs text-bad">{(save.error as Error).message}</p>}
    </Card>
  );
}

const planTone = (p: string): PillTone => (p === "enterprise" ? "violet" : p === "business" ? "info" : p === "team" ? "good" : "neutral");

/** One screen for "what needs me now": metrics, switches, and the tables that explain them. Counts and statuses only. */
export function OverviewView() {
  const me = useOperatorMe();
  const tenants = useOperatorTenants();
  const errors = useOperatorErrors("open", "all");
  const stats = useOperatorStats(24);
  const conns = useOperatorConnections();
  const system = useOperatorSystem();
  const sources = useOperatorDataSources();
  const audit = useOperatorAudit(8);
  const jobs = useOperatorJobs();
  const usage = useOperatorApiUsage();
  const plans = useOperatorPlans();

  const failed24 = stats.data?.filter((s) => s.status === "failed").reduce((n, s) => n + s.count, 0);
  const sum = (f: (t: OperatorTenant) => number) => (tenants.data ? tenants.data.reduce((n, t) => n + f(t), 0) : undefined);
  const backlog = jobs.data?.queues.reduce((n, q) => n + q.pending + q.retry, 0);
  const failedJobs = jobs.data?.queues.reduce((n, q) => n + q.failed_today, 0);

  const attention = useMemo(() => {
    const out: Item[] = [];
    if (errors.data?.length) out.push({ key: "err", text: `${errors.data.length} open error group${errors.data.length === 1 ? "" : "s"}`, href: "/operator/errors", icon: AlertTriangle });
    if (conns.data?.length) out.push({ key: "conn", text: `${conns.data.length} cloud connection${conns.data.length === 1 ? "" : "s"} failing to sync`, href: "/operator/connections", icon: Cloud });
    if (failed24) out.push({ key: "notif", text: `${failed24} failed notification${failed24 === 1 ? "" : "s"} in 24 hours`, href: "/operator/notifications", icon: Bell });
    const missing = system.data?.integrations.filter((i) => i.status === "missing" && !/^(planned|optional)/.test(i.note)) ?? [];
    if (missing.length) out.push({ key: "int", text: `${missing.length} integration${missing.length === 1 ? "" : "s"} not set up: ${missing.map((m) => m.name).join(", ")}`, href: "/operator/data-sources", icon: Plug });
    const bad = sources.data?.filter((s) => s.state === "attention") ?? [];
    if (bad.length) out.push({ key: "src", text: `${bad.length} data source${bad.length === 1 ? "" : "s"} with failing connections`, href: "/operator/data-sources", icon: Database });
    return out;
  }, [errors.data, conns.data, failed24, system.data, sources.data]);
  const loading = [errors, stats, conns, system, sources].some((q) => q.isLoading);

  const orgCols: Column<OperatorTenant>[] = [
    { key: "name", header: "Organization", sort: (a, b) => a.name.localeCompare(b.name), render: (t) => t.name },
    { key: "plan", header: "Plan", sort: (a, b) => a.plan.localeCompare(b.plan), render: (t) => <Pill tone={planTone(t.plan)}>{t.plan}</Pill> },
    { key: "members", header: "Members", align: "right", sort: (a, b) => a.members - b.members, render: (t) => <span className="tabular-nums">{t.members}</span> },
    { key: "projects", header: "Projects", align: "right", hideBelow: "md", sort: (a, b) => a.projects - b.projects, render: (t) => <span className="tabular-nums">{t.projects}</span> },
    { key: "conns", header: "Connections", align: "right", sort: (a, b) => a.connections - b.connections,
      render: (t) => <span className="tabular-nums">{t.connections}{t.connections_in_error > 0 && <span className="ml-1.5 text-bad">· {t.connections_in_error} failing</span>}</span> },
    { key: "deliv", header: "Failed deliveries 7d", align: "right", hideBelow: "lg", sort: (a, b) => a.failed_deliveries_7d - b.failed_deliveries_7d,
      render: (t) => <span className={t.failed_deliveries_7d ? "font-medium text-amber-700 tabular-nums" : "tabular-nums text-muted"}>{t.failed_deliveries_7d}</span> },
    { key: "state", header: "State", sort: (a, b) => (b.connections_in_error + b.failed_deliveries_7d) - (a.connections_in_error + a.failed_deliveries_7d),
      render: (t) => (t.connections_in_error + t.failed_deliveries_7d > 0 ? <Pill tone="warn">needs a look</Pill> : <Pill tone="good">healthy</Pill>) },
  ];

  return (
    <div className="space-y-6">
      <PageTitle title="Overview" sub="The state of the platform and the people who run it. Only counts and statuses: usage data, reports, payloads and addresses of organizations are never available here." />

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Tile index={0} label="Organizations" value={tenants.data?.length} icon={Building2} color="sage" />
        <Tile index={1} label="Members" value={sum((t) => t.members)} icon={Users} color="lavender" />
        <Tile index={2} label="Projects" value={sum((t) => t.projects)} icon={FolderKanban} color="sky" />
        <Tile index={3} label="Cloud connections" value={sum((t) => t.connections)} icon={Cloud} color="sky" note={conns.data?.length ? `${conns.data.length} failing` : undefined} tone={conns.data?.length ? "warn" : undefined} />
        <Tile index={4} label="Open errors" value={errors.data?.length} icon={AlertTriangle} color="peach" tone={errors.data?.length ? "bad" : undefined} />
        <Tile index={5} label="Failed notifications (24 h)" value={failed24} icon={Bell} color="butter" tone={failed24 ? "warn" : undefined} />
        <Tile index={6} label="Queue backlog" value={backlog} icon={Layers} color="lavender" note="pending and retrying jobs" tone={backlog && backlog > 50 ? "warn" : undefined} />
        <Tile index={7} label="Failed jobs today" value={failedJobs} icon={Zap} color="peach" tone={failedJobs ? "bad" : undefined} />
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <PlatformControls canAct={me.data?.role === "admin"} />
        <Card index={5} className="xl:col-span-2" interactive={false}>
          <CardHeader title="Needs attention" />
          {loading ? <Skeleton className="h-28" /> : attention.length === 0 ? (
            <div className="flex items-center gap-3 rounded-2xl bg-emerald-50 px-4 py-5">
              <CheckCircle2 className="size-6 text-emerald-600" aria-hidden />
              <p className="text-sm font-medium">Nothing needs attention: no open errors, failing connections, failed deliveries or missing integrations.</p>
            </div>
          ) : (
            <ul className="space-y-2">
              {attention.map((a, i) => (
                <li key={a.key} className="rise" style={{ "--i": i } as CSSProperties}>
                  <Link href={a.href} className="group flex items-center gap-3 rounded-2xl bg-white px-4 py-3 text-sm ring-1 ring-line transition hover:shadow-sm">
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-xl bg-orange-50"><a.icon className="size-4 text-orange-600" aria-hidden /></span>
                    <span className="font-medium">{a.text}</span>
                    <ArrowRight className="ml-auto size-4 text-muted transition group-hover:translate-x-0.5" aria-hidden />
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>

      <Card index={6} interactive={false}>
        <CardHeader title="Organizations" icon={<Building2 className="size-4 text-emerald-600" />}
          action={<Link href="/operator/organizations" className="text-xs font-semibold text-primary hover:underline">All organizations →</Link>} />
        {tenants.isLoading ? <Skeleton className="h-40" /> : (
          <DataTable columns={orgCols} rows={tenants.data ?? []} rowKey={(t) => t.id} href={(t) => `/operator/organizations/${t.id}`}
            initialSort={{ key: "state", dir: "desc" }} empty="No organizations yet." />
        )}
      </Card>

      <div className="grid gap-4 xl:grid-cols-2">
        <Card index={7} interactive={false}>
          <CardHeader title="Queues" icon={<Cpu className="size-4 text-violet-500" />} action={<Link href="/operator/jobs" className="text-xs font-semibold text-primary hover:underline">Jobs →</Link>} />
          {jobs.isLoading ? <Skeleton className="h-32" /> : (
            <DataTable dense rowKey={(q) => q.name} rows={jobs.data?.queues ?? []} empty="No queues reported."
              columns={[
                { key: "name", header: "Queue", render: (q) => q.name },
                { key: "pending", header: "Pending", align: "right", sort: (a, b) => a.pending - b.pending, render: (q) => <span className="tabular-nums">{q.pending}</span> },
                { key: "active", header: "Active", align: "right", render: (q) => <span className="tabular-nums">{q.active}</span> },
                { key: "retry", header: "Retry", align: "right", render: (q) => <span className={q.retry ? "font-medium text-amber-700 tabular-nums" : "tabular-nums text-muted"}>{q.retry}</span> },
                { key: "failed", header: "Failed today", align: "right", sort: (a, b) => a.failed_today - b.failed_today, render: (q) => <span className={q.failed_today ? "font-medium text-bad tabular-nums" : "tabular-nums text-muted"}>{q.failed_today}</span> },
                { key: "st", header: "State", render: (q) => (q.paused ? <Pill tone="warn">paused</Pill> : <Pill tone="good" pulse={q.active > 0}>running</Pill>) },
              ]} />
          )}
        </Card>

        <Card index={8} interactive={false}>
          <CardHeader title="Paid API usage today" icon={<Activity className="size-4 text-sky-500" />} action={<Link href="/operator/data-sources" className="text-xs font-semibold text-primary hover:underline">Sources →</Link>} />
          {usage.isLoading ? <Skeleton className="h-32" /> : (() => {
            const today = new Date().toISOString().slice(0, 10);
            const rows = (usage.data?.items ?? []).filter((i) => i.day.slice(0, 10) === today);
            return (
              <DataTable dense rowKey={(r) => r.provider} rows={rows} empty="No calls to outside providers yet today."
                columns={[
                  { key: "p", header: "Provider", render: (r) => r.provider },
                  { key: "r", header: "Requests", align: "right", sort: (a, b) => a.requests - b.requests, render: (r) => <span className="tabular-nums">{r.requests}</span> },
                  { key: "e", header: "Errors", align: "right", render: (r) => <span className={r.errors ? "font-medium text-bad tabular-nums" : "tabular-nums text-muted"}>{r.errors}</span> },
                  { key: "c", header: "Daily cap", render: (r) => <Meter value={r.requests} max={usage.data?.daily_caps[r.provider] ?? 0} label="Share of the daily request cap" /> },
                ]} />
            );
          })()}
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card index={9} interactive={false} className="xl:col-span-2">
          <CardHeader title="Data sources" icon={<Database className="size-4 text-emerald-600" />} action={<Link href="/operator/data-sources" className="text-xs font-semibold text-primary hover:underline">Manage →</Link>} />
          {sources.isLoading ? <Skeleton className="h-40" /> : (
            <DataTable dense rowKey={(s) => s.id} rows={sources.data ?? []} href={(s) => `/operator/data-sources/${s.id}`}
              initialSort={{ key: "state", dir: "asc" }}
              columns={[
                { key: "name", header: "Source", render: (s) => s.name },
                { key: "cat", header: "Category", hideBelow: "md", render: (s) => <span className="capitalize text-muted">{s.category.replace(/_/g, " ")}</span> },
                { key: "state", header: "State", sort: (a, b) => a.state.localeCompare(b.state),
                  render: (s) => <Pill tone={s.state === "ready" ? "good" : s.state === "attention" ? "bad" : s.state === "planned" ? "violet" : "neutral"}>{s.state.replace(/_/g, " ")}</Pill> },
                { key: "conn", header: "Connections", align: "right", hideBelow: "lg", render: (s) => <span className="tabular-nums">{s.connections ?? "—"}</span> },
                { key: "sync", header: "Last sync", hideBelow: "lg", render: (s) => <span className="text-muted">{s.last_sync_at ? shortTime(s.last_sync_at) : "—"}</span> },
              ]} />
          )}
        </Card>

        <Card index={10} interactive={false}>
          <CardHeader title="Plans" icon={<Layers className="size-4 text-violet-500" />} action={<Link href="/operator/plans" className="text-xs font-semibold text-primary hover:underline">Catalog →</Link>} />
          {plans.isLoading ? <Skeleton className="h-32" /> : (
            <ul className="space-y-2.5">
              {(plans.data ?? []).map((p) => {
                const total = (plans.data ?? []).reduce((n, x) => n + x.organizations, 0) || 1;
                return (
                  <li key={p.id} className="flex items-center gap-3 text-sm">
                    <span className="w-24 shrink-0"><Pill tone={planTone(p.id)}>{p.name}</Pill></span>
                    <Meter value={p.organizations} max={total} label={`${p.organizations} organizations`} />
                    <span className="w-6 text-right tabular-nums">{p.organizations}</span>
                  </li>
                );
              })}
            </ul>
          )}
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card index={11} interactive={false} className="xl:col-span-2">
          <CardHeader title="Integrations" icon={<Plug className="size-4 text-sky-500" />} action={<Link href="/operator/integrations" className="text-xs font-semibold text-primary hover:underline">Set up →</Link>} />
          {system.isLoading ? <Skeleton className="h-28" /> : system.data && (
            <>
              <div className="mb-3 flex flex-wrap items-center gap-x-6 gap-y-1 text-sm">
                <span><span className="text-muted">Version</span> <b>{system.data.version}</b></span>
                <span><span className="text-muted">Environment</span> <b>{system.data.env}</b></span>
                <span><span className="text-muted">Schema</span> <b>{system.data.expected_migration}</b></span>
              </div>
              <DataTable dense rowKey={(i) => i.name} rows={system.data.integrations}
                columns={[
                  { key: "n", header: "Integration", render: (i) => i.name },
                  { key: "s", header: "Status", sort: (a, b) => a.status.localeCompare(b.status), render: (i) => <Pill tone={i.status === "ready" ? "good" : i.status === "stub" ? "violet" : "warn"}>{i.status === "stub" ? "stand-in" : i.status}</Pill> },
                  { key: "note", header: "Note", hideBelow: "md", render: (i) => <span className="text-xs text-muted">{i.note}</span> },
                ]} />
            </>
          )}
        </Card>

        <Card index={12} interactive={false}>
          <CardHeader title="Recent operator activity" icon={<UserPlus className="size-4 text-amber-500" />} action={<Link href="/operator/audit" className="text-xs font-semibold text-primary hover:underline">All →</Link>} />
          {audit.isLoading ? <Skeleton className="h-28" /> : (
            <ul className="space-y-2.5 text-sm">
              {(audit.data ?? []).map((e) => {
                const d = describe(e.action);
                return (
                  <li key={e.id} className="flex items-start gap-2">
                    <span className={`mt-1.5 size-2 shrink-0 rounded-full ${d.write ? "bg-amber-400" : "bg-slate-300"}`} aria-hidden />
                    <span className="min-w-0"><span className="font-medium">{d.area}</span> <span className="text-muted">· {d.write ? "change" : "read"} · {e.operator_email || e.operator_sub}</span>
                      <span className="block text-xs text-muted">{shortTime(e.at)}</span></span>
                  </li>
                );
              })}
            </ul>
          )}
        </Card>
      </div>
    </div>
  );
}
