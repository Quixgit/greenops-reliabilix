"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Bell, Cloud, Users } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Badge, statusTone } from "@/components/ui/badge";
import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import {
  useOperatorConnections, useOperatorDeliveries, useOperatorErrors, useOperatorMe, useOperatorStats, useOperatorSystem, useOperatorTenants,
} from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { changed } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { cn } from "@/lib/utils";

const th = "py-1.5 pr-3 text-left text-xs font-medium text-muted";
const select = "rounded-lg bg-white/80 px-2.5 py-1.5 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-primary";

function Stat({ label, value, tone }: { label: string; value: string | number; tone?: "bad" | "warn" }) {
  return (
    <Card className="p-4">
      <p className="text-xs font-medium text-muted">{label}</p>
      <p className={cn("mt-1 text-2xl font-bold tracking-tight", tone === "bad" && "text-bad", tone === "warn" && "text-amber-600")}>{value}</p>
    </Card>
  );
}

function useCanAct() { return useOperatorMe().data?.role === "admin"; }

export function OrganizationsView() {
  const tenants = useOperatorTenants();
  const errors = useOperatorErrors("open", "all");
  const stats = useOperatorStats(24);
  const unhealthy = useOperatorConnections();
  const failed24 = stats.data?.filter((s) => s.status === "failed").reduce((n, s) => n + s.count, 0);
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Organizations</h1>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Stat label="Organizations" value={tenants.data?.length ?? "…"} />
        <Stat label="Open errors" value={errors.data?.length ?? "…"} tone={errors.data?.length ? "bad" : undefined} />
        <Stat label="Failed notifications (24 h)" value={failed24 ?? "…"} tone={failed24 ? "warn" : undefined} />
        <Stat label="Connections needing attention" value={unhealthy.data?.length ?? "…"} tone={unhealthy.data?.length ? "warn" : undefined} />
      </div>
      <Card>
        <CardHeader title="All organizations" />
        {tenants.isLoading ? <Skeleton className="h-32" /> : tenants.isError ? <ErrorState /> : tenants.data!.length === 0 ? <EmptyState icon={Users} text="No organizations yet." /> : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead><tr><th className={th}>Name</th><th className={th}>Plan</th><th className={th}>Members</th><th className={th}>Projects</th><th className={th}>Connections</th><th className={th}>Destinations</th><th className={th}>Failed (7 d)</th><th className={th}>Created</th></tr></thead>
              <tbody className="divide-y divide-line/70">
                {tenants.data!.map((t) => (
                  <tr key={t.id}>
                    <td className="py-2 pr-3 font-medium" title={t.id}><Link href={`/operator/organizations/${t.id}`} className="text-primary hover:underline">{t.name}</Link></td>
                    <td className="pr-3"><Badge>{t.plan}</Badge></td>
                    <td className="pr-3">{t.members}</td><td className="pr-3">{t.projects}</td>
                    <td className="pr-3">{t.connections}{t.connections_in_error > 0 && <Badge tone="bad" className="ml-2">{t.connections_in_error} error</Badge>}</td>
                    <td className="pr-3">{t.destinations}</td>
                    <td className="pr-3">{t.failed_deliveries_7d > 0 ? <Badge tone="warn">{t.failed_deliveries_7d}</Badge> : 0}</td>
                    <td className="whitespace-nowrap text-muted">{shortTime(t.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      <p className="text-xs text-muted">Only counts and statuses are shown here. Usage data, reports, payloads and destination addresses of organizations are never available to operators.</p>
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
        {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={AlertTriangle} text="No errors in this view. Errors of the api, the worker and the integrations they call are listed here, grouped." /> : (
          <ul className="divide-y divide-line/70 text-sm">
            {q.data!.map((e) => (
              <li key={e.id} className="space-y-1.5 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge tone={SEVERITY_TONE[e.severity]}>{e.severity}</Badge><Badge>{e.service}</Badge>
                  <code className="text-xs">{e.component}</code>
                  <span className="text-xs text-muted">× {e.occurrences.toLocaleString()} · last {shortTime(e.last_seen)} · first {shortTime(e.first_seen)}</span>
                  {e.status !== "open" && <Badge tone="neutral">{e.status}</Badge>}
                </div>
                <p className="break-words font-mono text-xs text-slate-700">{e.message}</p>
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
                  <span>{e.tenant_name ? `Last seen in “${e.tenant_name}”` : "Not tied to an organization"}{e.last_request_id ? ` · request ${e.last_request_id}` : ""}{e.handled_by ? ` · ${e.status} by ${e.handled_by}` : ""}</span>
                  {canAct && (
                    <span className="flex gap-2">
                      {e.status === "open" ? (<>
                        <button type="button" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "resolve" })} className="rounded-lg px-2.5 py-1 font-medium text-primary ring-1 ring-line hover:bg-white">Resolve</button>
                        <button type="button" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "ignore" })} className="rounded-lg px-2.5 py-1 font-medium ring-1 ring-line hover:bg-white">Ignore</button>
                      </>) : <button type="button" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "reopen" })} className="rounded-lg px-2.5 py-1 font-medium ring-1 ring-line hover:bg-white">Reopen</button>}
                    </span>
                  )}
                </div>
              </li>
            ))}
          </ul>
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
      <h1 className="text-2xl font-bold tracking-tight">Notifications</h1>
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
      <h1 className="text-2xl font-bold tracking-tight">Connections needing attention</h1>
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
      <h1 className="text-2xl font-bold tracking-tight">System</h1>
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
            <CardHeader title="Integrations" />
            <p className="mb-3 text-xs text-muted"><b>stub</b> means a development stand-in is running instead of the real service; <b>missing</b> means the feature is unavailable until it is set up.</p>
            <ul className="divide-y divide-line/70 text-sm">
              {q.data!.integrations.map((i) => (
                <li key={i.name} className="flex flex-wrap items-center justify-between gap-2 py-2.5">
                  <span className="font-medium">{i.name}</span>
                  <span className="flex items-center gap-3"><span className="text-xs text-muted">{i.note}</span><Badge tone={statusTone(i.status)}>{i.status}</Badge></span>
                </li>
              ))}
            </ul>
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
      <h1 className="text-2xl font-bold tracking-tight">Platform settings</h1>
      <PillTabs value={tab} onChange={(v) => setTab(v as "settings" | "history")} label="Platform settings sections" tabs={[{ value: "settings", label: "Settings" }, { value: "history", label: "Change history" }]} />
      {tab === "settings"
        ? <SettingsEditor mode="platform" path="operator/settings" historyPath="operator/settings/history" canEdit={me.data?.role === "admin"} />
        : <SettingsHistory path="operator/settings/history?limit=200" />}
    </div>
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
        <Link href="/operator" className="text-sm text-primary hover:underline">← Organizations</Link>
        <h1 className="mt-1 text-2xl font-bold tracking-tight">{org?.name ?? "Organization"}</h1>
        {org && (
          <p className="mt-1 flex flex-wrap items-center gap-2 text-sm text-muted">
            <Badge>{org.plan}</Badge>{org.members} members · {org.projects} projects · {org.connections} connections · {org.destinations} destinations
            {org.connections_in_error > 0 && <Badge tone="bad">{org.connections_in_error} connection error</Badge>}
          </p>
        )}
      </div>
      <PillTabs value={tab} onChange={(v) => setTab(v as "settings" | "history")} label="Organization sections" tabs={[{ value: "settings", label: "Settings and limits" }, { value: "history", label: "Change history" }]} />
      {tab === "settings"
        ? <SettingsEditor mode="operator-tenant" path={`operator/tenants/${id}/settings`} historyPath="" canEdit={me.data?.role === "admin"} />
        : <SettingsHistory path={`operator/settings/history?tenant_id=${id}&limit=200`} />}
    </div>
  );
}
