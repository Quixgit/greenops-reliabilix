"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconBell as Bell, IconCloud as Cloud, IconUsers as Users, IconWarning as AlertTriangle } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, useState } from "react";
import { Badge, statusTone } from "@/components/ui/badge";
import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import {
  useOperatorConnections, useOperatorDeliveries, useOperatorErrors, useOperatorMe, useOperatorOrgUsage, useOperatorPlans, useOperatorStats, useOperatorSubscription, useOperatorSystem, useOperatorTenants,
} from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { changed, settingsView, type OperatorTenant } from "@/lib/api/schemas";
import { useQuery } from "@tanstack/react-query";
import { shortTime } from "@/lib/format-time";
import { PageTitle, selectCls, th } from "./parts";
import { Check_, DataTable, Pill, type Column } from "./ui";

const select = selectCls;

function useCanAct() { return useOperatorMe().data?.role === "admin"; }

type OrgSort = "created" | "name" | "members" | "attention";

/** What each organization used in the last 30 days and the estimated cost of serving it (counts times unit costs). */
function OrgUsageCard() {
  const q = useOperatorOrgUsage(30);
  const n = (v: number) => v.toLocaleString();
  return (
    <Card interactive={false}>
      <CardHeader title="Resource use and cost of serving (last 30 days)" />
      <p className="-mt-2 mb-3 text-sm text-muted">Counts of records and runs per organization, and an estimate of what serving it costs: the counts times the unit costs set under Platform settings → Cost of serving an organization. It is an estimate for pricing and limits, not an accounting figure.</p>
      {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.items.length === 0 ? <EmptyState icon={Users} text="No organizations yet." /> : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead><tr><th className={th}>Organization</th><th className={th}>Plan</th><th className={th}>Usage rows</th><th className={th}>Calculations</th><th className={th}>Sync runs</th><th className={th}>Deliveries</th><th className={th}>Reports</th><th className={th}>K8s samples</th><th className={th}>Estimated cost</th></tr></thead>
            <tbody className="divide-y divide-line/70">
              {q.data!.items.map((u) => (
                <tr key={u.tenant_id} className="hover:bg-white/60">
                  <td className="py-2 pr-3 font-medium"><Link href={`/operator/organizations/${u.tenant_id}`} className="text-primary hover:underline">{u.name}</Link></td>
                  <td className="pr-3"><Badge>{u.plan}</Badge></td>
                  <td className="pr-3 tabular-nums">{n(u.usage_rows)}</td><td className="pr-3 tabular-nums">{n(u.calculation_rows)}</td><td className="pr-3 tabular-nums">{n(u.sync_runs)}</td>
                  <td className="pr-3 tabular-nums">{n(u.deliveries)}</td><td className="pr-3 tabular-nums">{n(u.reports)}</td><td className="pr-3 tabular-nums">{n(u.k8s_samples)}</td>
                  <td className="font-semibold tabular-nums">${u.estimated_cost_usd.toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

const planTone = (p: string) => (p === "enterprise" ? "violet" : p === "business" ? "info" : p === "team" ? "good" : "neutral") as "violet" | "info" | "good" | "neutral";
const ORG_COLUMNS: Column<OperatorTenant>[] = [
  { key: "name", header: "Name", sort: (a, b) => a.name.localeCompare(b.name), render: (t) => t.name },
  { key: "plan", header: "Plan", sort: (a, b) => a.plan.localeCompare(b.plan), render: (t) => <Pill tone={planTone(t.plan)}>{t.plan}</Pill> },
  { key: "members", header: "Members", align: "right", sort: (a, b) => a.members - b.members, render: (t) => <span className="tabular-nums">{t.members}</span> },
  { key: "projects", header: "Projects", align: "right", hideBelow: "md", sort: (a, b) => a.projects - b.projects, render: (t) => <span className="tabular-nums">{t.projects}</span> },
  { key: "conns", header: "Connections", align: "right", sort: (a, b) => a.connections - b.connections,
    render: (t) => <span className="tabular-nums">{t.connections}{t.connections_in_error > 0 && <span className="ml-1.5 font-medium text-bad">· {t.connections_in_error} error</span>}</span> },
  { key: "dest", header: "Destinations", align: "right", hideBelow: "lg", render: (t) => <span className="tabular-nums">{t.destinations}</span> },
  { key: "failed", header: "Failed (7 d)", align: "right", hideBelow: "md", sort: (a, b) => a.failed_deliveries_7d - b.failed_deliveries_7d,
    render: (t) => (t.failed_deliveries_7d > 0 ? <Pill tone="warn">{t.failed_deliveries_7d}</Pill> : <span className="text-muted">0</span>) },
  { key: "created", header: "Created", hideBelow: "lg", sort: (a, b) => a.created_at.localeCompare(b.created_at), render: (t) => <span className="whitespace-nowrap text-muted">{shortTime(t.created_at)}</span> },
];

export function OrganizationsView() {
  const tenants = useOperatorTenants();
  const [sort, setSort] = useState<OrgSort>("created");
  const [onlyAttention, setOnlyAttention] = useState(false);
  const rows = useMemo(() => {
    const needs = (t: OperatorTenant) => t.connections_in_error + t.failed_deliveries_7d;
    const list = (tenants.data ?? []).filter((t) => !onlyAttention || needs(t) > 0);
    const by: Record<OrgSort, (a: OperatorTenant, b: OperatorTenant) => number> = {
      created: (a, b) => b.created_at.localeCompare(a.created_at),
      name: (a, b) => a.name.localeCompare(b.name),
      members: (a, b) => b.members - a.members,
      attention: (a, b) => needs(b) - needs(a),
    };
    return [...list].sort(by[sort]);
  }, [tenants.data, sort, onlyAttention]);
  return (
    <div className="space-y-5">
      <PageTitle title="Organizations" sub="Counts and statuses only. Open an organization for its limits, module switches and change history." />
      <Card interactive={false}>
        <CardHeader title={`All organizations${tenants.data ? ` (${rows.length})` : ""}`} action={
          <div className="flex flex-wrap items-center gap-3">
            <Check_ label="Needs attention" checked={onlyAttention} onChange={setOnlyAttention}>Needs attention</Check_>
            <select aria-label="Sort" value={sort} onChange={(e) => setSort(e.target.value as OrgSort)} className={selectCls}>
              <option value="created">Newest first</option><option value="name">Name</option><option value="members">Most members</option><option value="attention">Most problems</option>
            </select>
          </div>} />
        {tenants.isLoading ? <Skeleton className="h-32" /> : tenants.isError ? <ErrorState /> : rows.length === 0 ? <EmptyState icon={Users} text={onlyAttention ? "No organization has problems right now." : "No organizations yet."} /> : (
          <DataTable rows={rows} rowKey={(t) => t.id} href={(t) => `/operator/organizations/${t.id}`} columns={ORG_COLUMNS} />
        )}
      </Card>
      <OrgUsageCard />
    </div>
  );
}

const SEVERITY_TONE = { warning: "warn", error: "bad", critical: "bad" } as const;

export function ErrorsView() {
  const qc = useQueryClient();
  const [status, setStatus] = useState("open");
  const [service, setService] = useState("all");
  const canAct = useCanAct();
  const q = useOperatorErrors(status, service);
  const act = useMutation({
    mutationFn: (v: { id: string; action: "resolve" | "ignore" | "reopen" }) => api(`operator/errors/${v.id}/${v.action}`, changed, undefined, { method: "POST" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["op-errors"] }),
  });
  const services = Array.from(new Set(["api", "worker", "notifications", ...(q.data?.map((e) => e.service) ?? [])]));
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-bold tracking-tight">Errors</h1>
        <div className="flex gap-2">
          <select aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)} className={select}><option value="open">Open</option><option value="resolved">Resolved</option><option value="ignored">Ignored</option><option value="all">All</option></select>
          <select aria-label="Service" value={service} onChange={(e) => setService(e.target.value)} className={select}><option value="all">All services</option>{services.map((s) => <option key={s} value={s}>{s}</option>)}</select>
        </div>
      </div>
      <Card>
        {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : (
          <DataTable rowKey={(e) => e.id} rows={q.data!} initialSort={{ key: "last", dir: "desc" }}
            empty="No errors in this view. Errors of the api, the worker and the integrations they call are listed here, grouped."
            columns={[
              { key: "sev", header: "Severity", sort: (a, b) => a.severity.localeCompare(b.severity), render: (e) => <Pill tone={SEVERITY_TONE[e.severity]}>{e.severity}</Pill> },
              { key: "where", header: "Where", sort: (a, b) => a.service.localeCompare(b.service),
                render: (e) => <span><span className="font-medium">{e.service}</span><code className="block text-xs text-muted">{e.component}</code></span> },
              { key: "msg", header: "Message", render: (e) => (
                <span className="block max-w-xl">
                  <span className="line-clamp-2 break-words font-mono text-xs text-slate-700">{e.message}</span>
                  <span className="mt-0.5 block text-xs text-muted">{e.tenant_name ? `Last seen in “${e.tenant_name}”` : "Not tied to an organization"}{e.last_request_id ? ` · request ${e.last_request_id}` : ""}{e.handled_by ? ` · ${e.status} by ${e.handled_by}` : ""}</span>
                </span>) },
              { key: "n", header: "×", align: "right", sort: (a, b) => a.occurrences - b.occurrences, render: (e) => <span className="tabular-nums">{e.occurrences.toLocaleString()}</span> },
              { key: "last", header: "Last seen", hideBelow: "md", sort: (a, b) => a.last_seen.localeCompare(b.last_seen), render: (e) => <span className="whitespace-nowrap text-muted" title={`first ${shortTime(e.first_seen)}`}>{shortTime(e.last_seen)}</span> },
              { key: "st", header: "Status", render: (e) => <Pill tone={e.status === "open" ? "bad" : e.status === "resolved" ? "good" : "neutral"}>{e.status}</Pill> },
              { key: "act", header: "", align: "right", render: (e) => !canAct ? null : (
                <span className="inline-flex gap-1.5">
                  {e.status === "open" ? (<>
                    <button type="button" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "resolve" })} className="rounded-lg px-2.5 py-1 text-xs font-medium text-primary ring-1 ring-line hover:bg-white">Resolve</button>
                    <button type="button" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "ignore" })} className="rounded-lg px-2.5 py-1 text-xs font-medium ring-1 ring-line hover:bg-white">Ignore</button>
                  </>) : <button type="button" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "reopen" })} className="rounded-lg px-2.5 py-1 text-xs font-medium ring-1 ring-line hover:bg-white">Reopen</button>}
                </span>) },
            ]} />
        )}
        {act.isError && <p role="alert" className="mt-2 text-sm text-bad">{act.error.message}</p>}
      </Card>
    </div>
  );
}

export function NotificationsOpsView() {
  const qc = useQueryClient();
  const [status, setStatus] = useState("failed");
  const canAct = useCanAct();
  const stats = useOperatorStats(24);
  const q = useOperatorDeliveries(status);
  const refresh = () => { void qc.invalidateQueries({ queryKey: ["op-deliveries"] }); void qc.invalidateQueries({ queryKey: ["op-stats"] }); };
  const retry = useMutation({ mutationFn: (v: { tenant: string; id: string }) => api(`operator/tenants/${v.tenant}/deliveries/${v.id}/retry`, changed, undefined, { method: "POST" }), onSuccess: refresh });
  const pause = useMutation({ mutationFn: (v: { tenant: string; id: string }) => api(`operator/tenants/${v.tenant}/destinations/${v.id}/disable`, changed, undefined, { method: "POST" }), onSuccess: refresh });
  const err = retry.error ?? pause.error;
  return (
    <div className="space-y-5">
      <PageTitle title="Notifications" sub="Delivery of e-mail, webhook and Slack messages across organizations." />
      <Card>
        <CardHeader title="Last 24 hours" />
        {stats.isLoading ? <Skeleton className="h-12" /> : stats.isError ? <ErrorState /> : stats.data!.length === 0 ? <p className="text-sm text-muted">No deliveries in this window.</p> : (
          <div className="flex flex-wrap gap-2">{stats.data!.map((s) => <span key={s.kind + s.status} className="inline-flex items-center gap-2 rounded-xl bg-white/70 px-3 py-1.5 text-sm ring-1 ring-line">{s.kind}<Badge tone={statusTone(s.status)}>{s.status}</Badge><b>{s.count}</b></span>)}</div>
        )}
      </Card>
      <Card>
        <CardHeader title="Deliveries" action={<select aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)} className={select}><option value="failed">Failed</option><option value="pending">Pending</option><option value="sent">Sent</option><option value="skipped">Skipped</option><option value="all">All</option></select>} />
        {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={Bell} text="Nothing in this view." /> : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead><tr><th className={th}>When</th><th className={th}>Organization</th><th className={th}>Destination</th><th className={th}>Event</th><th className={th}>Status</th><th className={th}>Detail</th><th /></tr></thead>
              <tbody className="divide-y divide-line/70">
                {q.data!.map((d) => (
                  <tr key={d.id}>
                    <td className="whitespace-nowrap py-2 pr-3 text-muted" title={d.created_at}>{shortTime(d.created_at)}</td>
                    <td className="pr-3">{d.tenant_name}</td>
                    <td className="pr-3">{d.destination_name} <span className="text-xs text-muted">{d.kind}</span></td>
                    <td className="pr-3">{d.event_type}</td>
                    <td className="pr-3"><Badge tone={statusTone(d.status)}>{d.status}</Badge></td>
                    <td className="max-w-xs break-words text-xs text-muted">attempt {d.attempts}{d.last_error ? ` · ${d.last_error}` : ""}</td>
                    <td className="whitespace-nowrap text-right">
                      {canAct && d.status === "failed" && d.destination_id && (<>
                        <button type="button" disabled={retry.isPending} onClick={() => retry.mutate({ tenant: d.tenant_id, id: d.id })} className="mr-2 rounded-lg px-2.5 py-1 text-xs font-medium text-primary ring-1 ring-line hover:bg-white">Retry</button>
                        <button type="button" disabled={pause.isPending} onClick={() => pause.mutate({ tenant: d.tenant_id, id: d.destination_id! })} className="rounded-lg px-2.5 py-1 text-xs font-medium ring-1 ring-line hover:bg-white">Pause destination</button>
                      </>)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {err && <p role="alert" className="mt-2 text-sm text-bad">{err.message}</p>}
        <p className="mt-3 text-xs text-muted">Retry and pause are recorded in the organization’s own audit log.</p>
      </Card>
    </div>
  );
}

export function ConnectionsOpsView() {
  const q = useOperatorConnections();
  return (
    <div className="space-y-5">
      <PageTitle title="Connections" sub="Customer cloud accounts whose last synchronization failed." />
      <Card>
        {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={Cloud} text="Every cloud connection is healthy or still pending." /> : (
          <ul className="divide-y divide-line/70 text-sm">
            {q.data!.map((c) => (
              <li key={`${c.tenant_id}${c.provider}${c.account_ref}`} className="py-3">
                <p className="flex flex-wrap items-center gap-2 font-medium">{c.tenant_name}<Badge>{c.provider}</Badge><code className="text-xs font-normal">{c.account_ref}</code><Badge tone="bad">{c.sync_status}</Badge></p>
                <p className="mt-0.5 text-xs text-muted">Last sync {shortTime(c.last_sync_at)}{c.last_error ? ` · ${c.last_error}` : ""}</p>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}

export function SystemView() {
  const q = useOperatorSystem();
  return (
    <div className="space-y-5">
      <PageTitle title="System" sub="Which build is running and which database schema it expects." />
      {q.isLoading ? <Skeleton className="h-48" /> : q.isError ? <ErrorState /> : (
        <>
          <Card>
            <CardHeader title="Build" />
            <dl className="grid grid-cols-[160px_1fr] gap-y-2 text-sm">
              <dt className="text-muted">Version</dt><dd>{q.data!.version}</dd>
              <dt className="text-muted">Environment</dt><dd>{q.data!.env}</dd>
              <dt className="text-muted">Schema version</dt><dd>{q.data!.expected_migration}</dd>
            </dl>
          </Card>
          <Card>
            <CardHeader title="Services" />
            <p className="text-sm text-muted">Keys, addresses and the state of every external service (identity, mail, storage, grid data, billing exports) are managed on their own pages. <Link href="/operator/data-sources?tab=services" className="font-medium text-primary hover:underline">Open data sources and platform services →</Link></p>
          </Card>
        </>
      )}
    </div>
  );
}

export function PlatformSettingsView() {
  const me = useOperatorMe();
  const [tab, setTab] = useState<"settings" | "history">("settings");
  return (
    <div className="space-y-5">
      <PageTitle title="Platform settings" sub="Limits, module switches, security, retention. Every change is written to the history." />
      <PillTabs value={tab} onChange={(v) => setTab(v as "settings" | "history")} label="Platform settings sections" tabs={[{ value: "settings", label: "Settings" }, { value: "history", label: "Change history" }]} />
      {tab === "settings"
        ? <SettingsEditor mode="platform" path="operator/settings" historyPath="operator/settings/history" canEdit={me.data?.role === "admin"} />
        : <SettingsHistory path="operator/settings/history?limit=200" />}
    </div>
  );
}

/** Suspend or restore one organization. It is a locked, audited setting; the API gate refuses changes while it is on. */
function StatusPanel({ id, canAct }: { id: string; canAct: boolean }) {
  const qc = useQueryClient();
  const path = `operator/tenants/${id}/settings`;
  const view = useQuery({ queryKey: ["op-org-settings", id], queryFn: () => api(path, settingsView) });
  const [message, setMessage] = useState("");
  const item = (k: string) => view.data?.items.find((i) => i.key === k);
  const suspended = item("organization.suspended")?.value === true;
  const current = String(item("organization.suspended_message")?.value ?? "");
  const set = useMutation({
    mutationFn: (values: Record<string, unknown>) => api(path, settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values }) }),
    onSuccess: (data) => { qc.setQueryData(["op-org-settings", id], data); void qc.invalidateQueries({ queryKey: ["settings"] }); },
  });
  return (
    <Card interactive={false}>
      <CardHeader title="Status" action={<Badge tone={suspended ? "bad" : "good"}>{suspended ? "Suspended" : "Active"}</Badge>} />
      <p className="text-sm text-muted">
        {suspended ? "People in this organization can read and export their data but cannot change anything." : "Suspending keeps all data. People can still read and export it; every change is refused until you restore access."}
      </p>
      {canAct && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          {suspended ? (
            <button type="button" disabled={set.isPending} onClick={() => set.mutate({ "organization.suspended": false })} className="rounded-xl bg-emerald-600 px-3.5 py-2 text-sm font-semibold text-white disabled:opacity-60">Restore access</button>
          ) : (<>
            <input value={message} onChange={(e) => setMessage(e.target.value)} maxLength={300} aria-label="Message shown to the organization" placeholder={current || "Reason shown to the organization"} className={`${selectCls} w-full sm:w-96`} />
            <ConfirmButton label="Suspend organization" confirmLabel="Suspend now" disabled={set.isPending}
              onConfirm={() => set.mutate({ "organization.suspended": true, ...(message.trim() ? { "organization.suspended_message": message.trim() } : {}) })} />
          </>)}
        </div>
      )}
      {set.isError && <p role="alert" className="mt-2 text-sm text-bad">{set.error.message}</p>}
    </Card>
  );
}

/** Assign a plan: writes the plan's limits and modules for this organization, with a reason in its audit log. */
function PlanPanel({ id, current, canAct }: { id: string; current: string; canAct: boolean }) {
  const qc = useQueryClient();
  const plans = useOperatorPlans();
  const sub = useOperatorSubscription(id);
  const [plan, setPlan] = useState("");
  const [reason, setReason] = useState("");
  const [status, setStatus] = useState("active");
  const [until, setUntil] = useState("");
  const assign = useMutation({
    mutationFn: () => api(`operator/tenants/${id}/plan`, changed, undefined, {
      method: "PUT",
      body: JSON.stringify({ plan, reason, status, ...(status === "trialing" ? { trial_ends_at: until } : until ? { period_end: until } : {}) }),
    }),
    onSuccess: () => {
      setReason("");
      for (const k of ["op-tenants", "op-plans", "op-org-settings", "settings", "op-subscription", "plan"]) void qc.invalidateQueries({ queryKey: [k] });
    },
  });
  const chosen = plans.data?.find((p) => p.id === (plan || current));
  return (
    <Card interactive={false}>
      <CardHeader title="Plan" action={<Badge>{current}</Badge>} />
      {canAct ? (
        <div className="flex flex-wrap items-center gap-2">
          <select aria-label="Plan" value={plan || current} onChange={(e) => setPlan(e.target.value)} className={selectCls}>
            {plans.data?.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
          <select aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)} className={selectCls}>
            <option value="active">active</option><option value="trialing">trial</option><option value="past_due">past due</option><option value="canceled">canceled</option>
          </select>
          <label className="flex items-center gap-1.5 text-sm text-muted">{status === "trialing" ? "Trial ends" : "Paid until"}
            <input type="date" value={until} onChange={(e) => setUntil(e.target.value)} className={selectCls} aria-label="Date" /></label>
          <input value={reason} onChange={(e) => setReason(e.target.value)} maxLength={300} placeholder="Reason (required, kept in the audit log)" aria-label="Reason" className={`${selectCls} w-full sm:w-96`} />
          <button type="button" disabled={assign.isPending || reason.trim().length < 3 || !plan || (status === "trialing" && !until)} onClick={() => assign.mutate()}
            className="brand-gradient rounded-xl px-4 py-2 text-sm font-semibold text-white disabled:opacity-50">{assign.isPending ? "Assigning…" : "Assign plan"}</button>
        </div>
      ) : <p className="text-sm text-muted">Only an operator with the admin role can assign a plan.</p>}
      {sub.data && sub.data.status !== "none" && (
        <p className="mt-2 text-xs text-muted">
          Subscription: {sub.data.status}{sub.data.period_end ? `, until ${shortTime(sub.data.period_end)}` : ""} · source {sub.data.source} · set by {sub.data.updated_by.slice(0, 24)}.
          After the end date the scheduler returns the organization to Free.
        </p>
      )}
      {chosen && plan && plan !== current && (
        <p className="mt-2 text-xs text-muted">Will fix: {Object.entries(chosen.limits).map(([k, v]) => `${k.replace("limits.", "").replace(/_/g, " ")} ${v === 0 ? "unlimited" : v}`).join(", ")}; modules off: {Object.entries(chosen.modules).filter(([, v]) => !v).map(([k]) => k.replace("modules.", "")).join(", ") || "none"}.</p>
      )}
      {assign.isError && <p role="alert" className="mt-2 text-sm text-bad">{assign.error.message}</p>}
      {assign.isSuccess && <p role="status" className="mt-2 text-sm text-emerald-700">Plan assigned.</p>}
    </Card>
  );
}

export function OrganizationView({ id }: { id: string }) {
  const me = useOperatorMe();
  const tenants = useOperatorTenants();
  const org = tenants.data?.find((t) => t.id === id);
  const [tab, setTab] = useState<"settings" | "history">("settings");
  return (
    <div className="space-y-5">
      <div>
        <Link href="/operator/organizations" className="text-sm text-primary hover:underline">← Organizations</Link>
        <h1 className="mt-1 text-2xl font-bold tracking-tight">{org?.name ?? "Organization"}</h1>
        {org && (
          <p className="mt-1 flex flex-wrap items-center gap-2 text-sm text-muted">
            {org.members} members · {org.projects} projects · {org.connections} connections · {org.destinations} destinations
            {org.connections_in_error > 0 && <Badge tone="bad">{org.connections_in_error} connection error</Badge>}
          </p>
        )}
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <PlanPanel id={id} current={org?.plan ?? "free"} canAct={me.data?.role === "admin"} />
        <StatusPanel id={id} canAct={me.data?.role === "admin"} />
      </div>
      <PillTabs value={tab} onChange={(v) => setTab(v as "settings" | "history")} label="Organization sections" tabs={[{ value: "settings", label: "Settings and limits" }, { value: "history", label: "Change history" }]} />
      {tab === "settings"
        ? <SettingsEditor mode="operator-tenant" path={`operator/tenants/${id}/settings`} historyPath="" canEdit={me.data?.role === "admin"} />
        : <SettingsHistory path={`operator/settings/history?tenant_id=${id}&limit=200`} />}
    </div>
  );
}
