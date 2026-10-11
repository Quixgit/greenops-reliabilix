"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { ProviderMark } from "@/components/connectors/provider-mark";
import type { AnyIcon } from "@/components/ui/icons";
import { IconCard, IconKey, IconShield, IconBell as Bell, IconRefresh, IconCheckCircle as CheckCircle2, IconCloud as Cloud, IconClock, IconCpu, IconDatabase as Database, IconGauge as Gauge, IconPlug as Plug, IconServer as Server, IconWarning as AlertTriangle } from "@/components/ui/icons";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useOperatorConnections, useOperatorDeliveries, useOperatorErrors, useOperatorJobs, useOperatorMe, useOperatorStats, useOperatorSystem,
} from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { changed } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { BUILD_VERSION } from "@/lib/version";
import { relative } from "./jobs-meta";
import { PageTitle, TextTile, Tile } from "./parts";
import { cn } from "@/lib/utils";
import { Btn, EmptyPanel, Meter, Pill, RowList, SearchBox, Segmented, Select, type PillTone } from "./ui";

const ORDER = { missing: 0, stub: 1, ready: 2 } as const;
function useCanAct() { return useOperatorMe().data?.role === "admin"; }
const Err = ({ e }: { e: Error | null }) => (e ? <p role="alert" className="rise mt-3 rounded-xl bg-rose-50 px-3.5 py-2.5 text-sm text-[#c2566a] ring-1 ring-rose-200">{e.message}</p> : null);
const ago = (iso: string | null | undefined) => {
  if (!iso) return "never";
  const m = (Date.now() - new Date(iso).getTime()) / 60_000;
  return m < 1 ? "just now" : m < 60 ? `${Math.round(m)} min ago` : m < 2880 ? `${Math.round(m / 60)} h ago` : `${Math.round(m / 1440)} days ago`;
};

// ---- Errors -------------------------------------------------------------------------------------------------------------

const SEVERITY_TONE: Record<string, PillTone> = { warning: "warn", error: "bad", critical: "bad" };

export function ErrorsView() {
  const qc = useQueryClient();
  const canAct = useCanAct();
  const [status, setStatus] = useState("open");
  const [service, setService] = useState("all");
  const [q, setQ] = useState("");
  const open = useOperatorErrors("open", "all");
  const list = useOperatorErrors(status, service);
  const act = useMutation({
    mutationFn: (v: { id: string; action: "resolve" | "ignore" | "reopen" }) => api(`operator/errors/${v.id}/${v.action}`, changed, undefined, { method: "POST" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["op-errors"] }),
  });
  const services = useMemo(() => Array.from(new Set(["api", "worker", "notifications", ...(open.data?.map((e) => e.service) ?? []), ...(list.data?.map((e) => e.service) ?? [])])), [open.data, list.data]);
  const rows = useMemo(() => {
    const n = q.trim().toLowerCase();
    return (list.data ?? []).filter((e) => !n || `${e.message} ${e.component} ${e.tenant_name ?? ""}`.toLowerCase().includes(n));
  }, [list.data, q]);
  const od = open.data;
  const occurrences = od?.reduce((n, e) => n + e.occurrences, 0);
  const newest = od?.map((e) => e.last_seen).sort().at(-1);
  return (
    <div className="space-y-6">
      <PageTitle title="Errors" sub="Errors of the api, the worker and the integrations they call, grouped by where they happen. Resolve a group when it is fixed; it opens again by itself if it recurs." />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Tile index={0} label="Open groups" value={od?.length} icon={AlertTriangle} color="peach" tone={od?.length ? "bad" : undefined} />
        <Tile index={1} label="Occurrences in open groups" value={occurrences} icon={Gauge} color="butter" />
        <Tile index={2} label="Services affected" value={od ? new Set(od.map((e) => e.service)).size : undefined} icon={Server} color="lavender" />
        <TextTile index={3} label="Latest error" text={newest ? ago(newest) : "—"} icon={IconClock} color="sky" note={newest ? shortTime(newest) : "none recorded"} />
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <SearchBox value={q} onChange={setQ} placeholder="Search message, place or organization…" className="w-full sm:w-80" />
        <Segmented label="Status" value={status} onChange={setStatus} options={[{ value: "open", label: "Open", count: od?.length }, { value: "resolved", label: "Resolved" }, { value: "ignored", label: "Ignored" }, { value: "all", label: "All" }]} />
        <Select label="Service" value={service} onChange={setService} options={[{ value: "all", label: "All services" }, ...services.map((s) => ({ value: s, label: s }))]} />
      </div>
      {list.isLoading ? <Skeleton className="h-40" /> : list.isError ? <ErrorState /> : (
        <RowList rows={rows} rowKey={(e) => e.id} pageSize={8}
          empty={status === "open" && !q ? <EmptyPanel title="No open errors" text="The platform is quiet. Errors of the api, the worker and their integrations appear here, grouped, the moment they happen." /> : <EmptyPanel tone="quiet" icon={AlertTriangle} title="No errors in this view" text="Change the status, the service or the search." />}
          row={(e) => (
            <div className="group flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-3 py-3.5 transition-colors hover:bg-slate-50">
              <AlertTriangle className={cn("size-10 shrink-0", e.status !== "open" ? "text-slate-300" : e.severity === "warning" ? "text-[#d9a441]" : "text-[#d9707f]")} strokeWidth={1.4} />
              <div className="min-w-0 flex-1 basis-80">
                <p className="flex flex-wrap items-center gap-2 font-semibold">{e.service}<code className="text-xs font-normal text-muted">{e.component}</code>
                  <Pill tone={e.status === "open" ? SEVERITY_TONE[e.severity] : e.status === "resolved" ? "good" : "neutral"}>{e.status === "open" ? e.severity : e.status}</Pill></p>
                <p className="mt-1 line-clamp-2 break-words rounded-lg bg-slate-50 px-3 py-2 font-mono text-xs text-slate-700" title={e.message}>{e.message}</p>
                <p className="mt-1.5 text-xs text-muted"><b className="tabular-nums text-ink">{e.occurrences.toLocaleString()}</b> time{e.occurrences === 1 ? "" : "s"} · last {relative(e.last_seen)} ({shortTime(e.last_seen)}) · first {shortTime(e.first_seen)}
                  {" · "}{e.tenant_name ? <>in “{e.tenant_name}”</> : "not tied to an organization"}{e.last_request_id ? <> · request <code>{e.last_request_id}</code></> : ""}{e.handled_by ? ` · ${e.status} by ${e.handled_by}` : ""}</p>
              </div>
              {canAct && (
                <div className="flex items-center gap-2">
                  {e.status === "open" ? (<>
                    <Btn size="sm" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "resolve" })}><CheckCircle2 className="size-3.5" />Resolve</Btn>
                    <Btn tone="quiet" size="sm" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "ignore" })}>Ignore</Btn>
                  </>) : <Btn tone="quiet" size="sm" disabled={act.isPending} onClick={() => act.mutate({ id: e.id, action: "reopen" })}>Reopen</Btn>}
                </div>
              )}
            </div>
          )} />
      )}
      <Err e={act.error} />
    </div>
  );
}

// ---- Notifications ------------------------------------------------------------------------------------------------------

const DELIVERY_TONE: Record<string, PillTone> = { sent: "good", failed: "bad", pending: "info", skipped: "neutral" };

export function NotificationsOpsView() {
  const qc = useQueryClient();
  const canAct = useCanAct();
  const [status, setStatus] = useState("failed");
  const [q, setQ] = useState("");
  const stats = useOperatorStats(24);
  const list = useOperatorDeliveries(status);
  const refresh = () => { void qc.invalidateQueries({ queryKey: ["op-deliveries"] }); void qc.invalidateQueries({ queryKey: ["op-stats"] }); };
  const retry = useMutation({ mutationFn: (v: { tenant: string; id: string }) => api(`operator/tenants/${v.tenant}/deliveries/${v.id}/retry`, changed, undefined, { method: "POST" }), onSuccess: refresh });
  const pause = useMutation({ mutationFn: (v: { tenant: string; id: string }) => api(`operator/tenants/${v.tenant}/destinations/${v.id}/disable`, changed, undefined, { method: "POST" }), onSuccess: refresh });
  const total = (st: string) => stats.data?.filter((s) => s.status === st).reduce((n, s) => n + s.count, 0);
  const byKind = useMemo(() => {
    const m = new Map<string, { sent: number; failed: number }>();
    for (const s of stats.data ?? []) { const r = m.get(s.kind) ?? { sent: 0, failed: 0 }; if (s.status === "sent") r.sent += s.count; if (s.status === "failed") r.failed += s.count; m.set(s.kind, r); }
    return [...m.entries()];
  }, [stats.data]);
  const rows = useMemo(() => {
    const n = q.trim().toLowerCase();
    return (list.data ?? []).filter((d) => !n || `${d.tenant_name} ${d.destination_name} ${d.event_type} ${d.last_error ?? ""}`.toLowerCase().includes(n));
  }, [list.data, q]);
  return (
    <div className="space-y-6">
      <PageTitle title="Notifications" sub="Delivery of e-mail, webhook and Slack messages across organizations. Retry and pause are recorded in the organization’s own audit log." />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Tile index={0} label="Sent, 24 h" value={stats.isLoading ? undefined : total("sent") ?? 0} icon={CheckCircle2} color="sage" />
        <Tile index={1} label="Failed, 24 h" value={stats.isLoading ? undefined : total("failed") ?? 0} icon={AlertTriangle} color="peach" tone={total("failed") ? "bad" : undefined} />
        <Tile index={2} label="Waiting" value={stats.isLoading ? undefined : total("pending") ?? 0} icon={IconClock} color="sky" />
        <Tile index={3} label="Skipped" value={stats.isLoading ? undefined : total("skipped") ?? 0} icon={Bell} color="lavender" note="no destination or switched off" />
      </div>
      {byKind.length > 0 && (
        <div className="flex flex-wrap gap-2.5">
          {byKind.map(([kind, v]) => (
            <span key={kind} className="rise inline-flex items-center gap-2.5 rounded-xl bg-white px-3.5 py-2 text-sm ring-1 ring-line">
              <ProviderMark id={kind} size="sm" /><span className="font-medium capitalize">{kind}</span>
              <span className="text-xs text-muted tabular-nums">{v.sent} sent</span>{v.failed > 0 && <Pill tone="bad">{v.failed} failed</Pill>}
            </span>
          ))}
        </div>
      )}
      <div className="flex flex-wrap items-center gap-3">
        <SearchBox value={q} onChange={setQ} placeholder="Search organization, destination or error…" className="w-full sm:w-80" />
        <Segmented label="Status" value={status} onChange={setStatus} options={[{ value: "failed", label: "Failed", count: total("failed") }, { value: "pending", label: "Pending" }, { value: "sent", label: "Sent" }, { value: "skipped", label: "Skipped" }, { value: "all", label: "All" }]} />
      </div>
      {list.isLoading ? <Skeleton className="h-40" /> : list.isError ? <ErrorState /> : (
        <RowList rows={rows} rowKey={(d) => d.id} pageSize={8}
          empty={status === "failed" && !q ? <EmptyPanel title="Nothing failed" text="Every message of the last days reached its destination, or is still on its way." /> : <EmptyPanel tone="quiet" icon={Bell} title="Nothing in this view" text="Change the status or the search." />}
          row={(d) => (
            <div className="group flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-3 py-3.5 transition-colors hover:bg-slate-50">
              <ProviderMark id={d.kind} size="sm" />
              <div className="min-w-0 flex-1 basis-72">
                <p className="flex flex-wrap items-center gap-2 font-semibold"><Link href={`/operator/organizations/${d.tenant_id}`} className="hover:text-primary">{d.tenant_name}</Link>
                  <span className="text-sm font-normal text-muted">→ {d.destination_name}</span><Pill tone={DELIVERY_TONE[d.status] ?? "neutral"}>{d.status}</Pill></p>
                <p className="mt-0.5 text-xs text-muted"><code>{d.event_type}</code> · attempt {d.attempts} · {relative(d.created_at)} ({shortTime(d.created_at)})</p>
                {d.last_error && <p className="mt-1.5 line-clamp-2 break-words rounded-lg bg-slate-50 px-3 py-2 font-mono text-xs text-slate-700" title={d.last_error}>{d.last_error}</p>}
              </div>
              {canAct && d.status === "failed" && d.destination_id && (
                <div className="flex items-center gap-2">
                  <Btn tone="quiet" size="sm" disabled={retry.isPending} onClick={() => retry.mutate({ tenant: d.tenant_id, id: d.id })}><IconRefresh className="size-3.5" />Retry</Btn>
                  <Btn tone="quiet" size="sm" disabled={pause.isPending} onClick={() => pause.mutate({ tenant: d.tenant_id, id: d.destination_id! })}>Pause destination</Btn>
                </div>
              )}
            </div>
          )} />
      )}
      <Err e={retry.error ?? pause.error} />
    </div>
  );
}

// ---- Connections --------------------------------------------------------------------------------------------------------

export function ConnectionsOpsView() {
  const q = useOperatorConnections();
  const d = q.data;
  return (
    <div className="space-y-6">
      <PageTitle title="Connections" sub="Customer cloud accounts whose last synchronization failed. A connection leaves this list by itself when it syncs again." />
      <div className="grid gap-3 sm:grid-cols-3">
        <Tile index={0} label="Failing connections" value={d?.length} icon={Cloud} color="peach" tone={d?.length ? "bad" : undefined} />
        <Tile index={1} label="Organizations affected" value={d ? new Set(d.map((c) => c.tenant_id)).size : undefined} icon={Plug} color="lavender" />
        <Tile index={2} label="Providers" value={d ? new Set(d.map((c) => c.provider)).size : undefined} icon={Database} color="sky" />
      </div>
      {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : (
        <RowList rows={[...d!].sort((a, b) => (a.last_sync_at ?? "").localeCompare(b.last_sync_at ?? ""))} rowKey={(c) => `${c.tenant_id}${c.provider}${c.account_ref}`} pageSize={8}
          empty={<EmptyPanel title="Every connection is healthy" text="Customer cloud accounts that failed to synchronize would be listed here, with the reason." />}
          row={(c) => (
            <div className="group flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-3 py-3.5 transition-colors hover:bg-slate-50">
              <ProviderMark id={c.provider} size="sm" />
              <div className="min-w-0 flex-1 basis-72">
                <p className="flex flex-wrap items-center gap-2 font-semibold"><Link href={`/operator/organizations/${c.tenant_id}`} className="hover:text-primary">{c.tenant_name}</Link>
                  <span className="text-sm font-normal uppercase text-muted">{c.provider}</span><code className="text-xs font-normal text-muted">{c.account_ref}</code><Pill tone="bad" pulse>{c.sync_status}</Pill></p>
                <p className="mt-0.5 text-xs text-muted">{c.last_sync_at ? <>last sync {relative(c.last_sync_at)} ({shortTime(c.last_sync_at)})</> : "never synchronized"}</p>
                {c.last_error && <p className="mt-1.5 line-clamp-2 break-words rounded-lg bg-slate-50 px-3 py-2 font-mono text-xs text-slate-700" title={c.last_error}>{c.last_error}</p>}
              </div>
              <Link href={`/operator/organizations/${c.tenant_id}`} className="text-xs font-semibold text-primary hover:underline">Open organization →</Link>
            </div>
          )} />
      )}
    </div>
  );
}

// ---- System -------------------------------------------------------------------------------------------------------------

const uptime = (iso: string) => {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
  return d > 0 ? `${d} d ${h} h` : h > 0 ? `${h} h ${m} min` : `${m} min`;
};

function Health({ ok, label, detail, icon: Icon, index, children }: { ok: boolean; label: string; detail: string; icon: AnyIcon; index: number; children?: React.ReactNode }) {
  return (
    <Card index={index} interactive={false} className="p-4">
      <div className="flex items-center gap-3.5">
        <Icon className={cn("size-10 shrink-0", ok ? "text-leaf-deep" : "text-[#d9707f]")} strokeWidth={1.4} />
        <div className="min-w-0 flex-1"><p className="text-sm font-medium text-slate-700">{label}</p><p className="font-semibold">{detail}</p></div>
        <span className={cn("size-3 shrink-0 rounded-full", ok ? "animate-pulse bg-leaf" : "bg-pastel-rose")} aria-label={ok ? "healthy" : "not healthy"} />
      </div>
      {children}
    </Card>
  );
}

/** The drawn mark or icon of an external service, found from its name. */
function ServiceMark({ name }: { name: string }) {
  const n = name.toLowerCase();
  const mark = n.includes("electricity") ? "electricitymaps" : n.includes("aws") ? "aws" : n.includes("gcp") ? "gcp" : n.includes("azure") ? "azure" : n.includes("watttime") ? "watttime" : n.includes("e-mail") ? "email" : "";
  if (mark) return <ProviderMark id={mark} size="sm" />;
  const Icon = n.includes("auth0") ? IconShield : n.includes("storage") ? Database : n.includes("payment") ? IconCard : n.includes("sentry") ? AlertTriangle : n.includes("tracing") ? IconCpu : IconKey;
  return <Icon className="size-10 shrink-0 text-[#5b8fdc]" strokeWidth={1.4} />;
}

export function SystemView() {
  const q = useOperatorSystem();
  const jobs = useOperatorJobs();
  const s = q.data;
  const queueOk = !!jobs.data && !jobs.isError;
  const waiting = jobs.data?.queues.reduce((n, x) => n + x.pending + x.retry, 0) ?? 0;
  const gaveUp = jobs.data?.queues.reduce((n, x) => n + x.archived, 0) ?? 0;
  const rt = s?.runtime, db = s?.database;
  const missing = s?.integrations.filter((i) => i.status === "missing" && !/^(planned|optional)/.test(i.note)) ?? [];
  return (
    <div className="space-y-6">
      <PageTitle title="System" sub="Which build is running, how the process and its database are doing, and which external services are set up." />
      {q.isLoading ? <Skeleton className="h-48" /> : q.isError || !s ? <ErrorState /> : (<>
        <div className="glass rise flex flex-wrap items-center gap-5 p-5">
          <span className="grid size-14 shrink-0 place-items-center rounded-2xl bg-leaf-soft text-leaf-deep ring-1 ring-leaf/20"><Server className="size-7" aria-hidden /></span>
          <div className="min-w-0 flex-1">
            <p className="text-xl font-bold tracking-tight">Reliabilix GreenOps Platform <span className="text-muted">v{BUILD_VERSION.number}</span></p>
            <p className="mt-0.5 text-sm text-muted">Release {s.version} · database schema {s.expected_migration}</p>
          </div>
          <div className="flex flex-wrap gap-2"><Pill tone={s.env === "production" ? "good" : "warn"}>{s.env}</Pill><Pill tone="violet">{BUILD_VERSION.stage}</Pill></div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <TextTile index={0} label="API running for" text={rt ? uptime(rt.started_at) : "—"} icon={IconClock} color="sage" note={rt ? `since ${shortTime(rt.started_at)}` : undefined} />
          <TextTile index={1} label="Database round trip" text={db ? `${db.ping_ms.toFixed(1)} ms` : "—"} icon={Database} color="sky" note={db && !db.ok ? "the database does not answer" : undefined} />
          <Tile index={2} label="Memory in use, MB" value={rt ? Math.round(rt.heap_mb) : null} icon={IconCpu} color="lavender" note={rt ? `${rt.goroutines} goroutines · ${rt.cpus} CPUs` : undefined} />
          <Tile index={3} label="Waiting jobs" value={jobs.isLoading ? undefined : waiting} icon={Gauge} color="butter" tone={gaveUp ? "warn" : undefined} note={gaveUp ? `${gaveUp} gave up` : "pending and retrying"} />
        </div>

        <div className="grid gap-4 md:grid-cols-3">
          <Health index={1} ok icon={Server} label="API" detail={rt ? `up ${uptime(rt.started_at)}` : "answering"} />
          <Health index={2} ok={!!db?.ok} icon={Database} label="Database" detail={db ? `${db.ping_ms.toFixed(1)} ms` : "unknown"}>
            {db && <div className="mt-3"><div className="mb-1 flex justify-between text-xs text-muted"><span>Connections in use</span><span className="tabular-nums">{db.in_use} of {db.max} · {db.connections} open</span></div><Meter value={db.in_use} max={db.max} label="Connections in use" /></div>}
          </Health>
          <Health index={3} ok={queueOk} icon={Gauge} label="Job queue (Redis)" detail={queueOk ? `${waiting} waiting` : "unreachable"}>
            {queueOk && <p className="mt-3 text-xs text-muted">{gaveUp ? <Link href="/operator/jobs" className="font-medium text-[#a8761f] hover:underline">{gaveUp} task{gaveUp === 1 ? "" : "s"} gave up →</Link> : "no task has given up"}</p>}
          </Health>
        </div>

        <div className="flex items-center justify-between gap-3">
          <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight"><Plug className="size-4 text-leaf-deep" />External services</h2>
          <Link href="/operator/data-sources?tab=services" className="text-xs font-semibold text-primary hover:underline">Open settings →</Link>
        </div>
        {missing.length > 0 && <p className="rounded-xl bg-amber-50 px-3.5 py-2.5 text-sm text-slate-700 ring-1 ring-amber-100">{missing.length} not set up: {missing.map((m) => m.name).join(", ")}.</p>}
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {[...s.integrations].sort((a, b) => ORDER[a.status] - ORDER[b.status]).map((i, n) => (
            <Card key={i.name} index={n} className="flex items-center gap-3.5 p-4">
              <ServiceMark name={i.name} />
              <div className="min-w-0 flex-1"><p className="truncate font-semibold" title={i.name}>{i.name}</p><p className="line-clamp-2 text-xs text-muted">{i.note || (i.status === "ready" ? "in use" : "")}</p></div>
              <Pill tone={i.status === "ready" ? "good" : i.status === "stub" ? "violet" : "warn"}>{i.status === "ready" ? "ready" : i.status === "stub" ? "stand-in" : "not set"}</Pill>
            </Card>
          ))}
        </div>
      </>)}
    </div>
  );
}
