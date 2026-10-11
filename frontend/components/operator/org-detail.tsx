"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { IconArrowLeft as ArrowLeft, IconClock, IconHistory, IconReceipt, IconShield, IconSliders, IconWarning } from "@/components/ui/icons";
import Link from "next/link";
import { useEffect, useState } from "react";
import { SettingsEditor, SettingsHistory } from "@/components/settings/settings-editor";
import { Card, CardHeader } from "@/components/ui/card";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { DateField } from "@/components/ui/date-field";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { useOperatorMe, useOperatorPlans, useOperatorSubscription, useOperatorTenants } from "@/hooks/use-admin";
import { useErasure, useOrgOverview } from "@/hooks/use-operator-extra";
import { api } from "@/lib/api/client";
import { changed, settingsView } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { describe } from "./audit";
import { Tile } from "./parts";
import { Avatar, Btn, inputCls, Pill, Select } from "./ui";

const planTone = (p: string) => (p === "enterprise" ? "violet" : p === "business" ? "info" : p === "team" ? "good" : "neutral") as "violet" | "info" | "good" | "neutral";
const Err = ({ e }: { e: Error | null }) => (e ? <p role="alert" className="rise mt-3 rounded-xl bg-rose-50 px-3.5 py-2.5 text-sm text-[#c2566a] ring-1 ring-rose-200">{e.message}</p> : null);

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
    <Card index={1} interactive={false}>
      <CardHeader title="Access" icon={<IconShield className="size-4 text-leaf-deep" />} action={<Pill tone={suspended ? "bad" : "good"} pulse={suspended}>{suspended ? "Suspended" : "Active"}</Pill>} />
      <p className="text-sm text-muted">
        {suspended ? "People in this organization can read and export their data but cannot change anything." : "Suspending keeps all data. People can still read and export it; every change is refused until you restore access."}
      </p>
      {suspended && current && <p className="mt-2 rounded-xl bg-amber-50 px-3.5 py-2.5 text-sm text-slate-700 ring-1 ring-amber-100">Shown to them: {current}</p>}
      {canAct ? (
        <div className="mt-4 flex flex-wrap items-center gap-2">
          {suspended ? (
            <Btn disabled={set.isPending} onClick={() => set.mutate({ "organization.suspended": false })}>Restore access</Btn>
          ) : (<>
            <input value={message} onChange={(e) => setMessage(e.target.value)} maxLength={300} aria-label="Message shown to the organization" placeholder={current || "Reason shown to the organization"} className={`${inputCls} sm:max-w-md`} />
            <ConfirmButton label="Suspend organization" confirmLabel="Suspend now" disabled={set.isPending}
              onConfirm={() => set.mutate({ "organization.suspended": true, ...(message.trim() ? { "organization.suspended_message": message.trim() } : {}) })} />
          </>)}
        </div>
      ) : <p className="mt-3 text-xs text-muted">Only an operator with the admin role can change this.</p>}
      <Err e={set.error} />
    </Card>
  );
}

const STATUSES = [{ value: "active", label: "Active" }, { value: "trialing", label: "Trial" }, { value: "past_due", label: "Past due" }, { value: "canceled", label: "Canceled" }];

/** Assign a plan: writes the plan's limits and modules for this organization, with a reason in its audit log. */
function PlanPanel({ id, current, canAct }: { id: string; current: string; canAct: boolean }) {
  const qc = useQueryClient();
  const plans = useOperatorPlans();
  const sub = useOperatorSubscription(id);
  const [plan, setPlan] = useState("");
  useEffect(() => { const p = new URLSearchParams(window.location.search).get("plan"); if (p) setPlan(p); }, []);
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
  const field = "text-xs font-semibold uppercase tracking-wide text-slate-400";
  return (
    <Card index={0} interactive={false}>
      <CardHeader title="Plan" icon={<IconReceipt className="size-4 text-leaf-deep" />} action={<Pill tone={planTone(current)}>{current}</Pill>} />
      {sub.data && sub.data.status !== "none" && (
        <div className="mb-4 grid grid-cols-3 gap-3 rounded-2xl bg-slate-50/70 p-3.5 text-sm ring-1 ring-line/60">
          <div><p className={field}>Status</p><p className="mt-1 font-medium capitalize">{sub.data.status.replace("_", " ")}</p></div>
          <div><p className={field}>Until</p><p className="mt-1 font-medium">{sub.data.period_end ? shortTime(sub.data.period_end) : "—"}</p></div>
          <div><p className={field}>Source</p><p className="mt-1 truncate font-medium" title={sub.data.updated_by}>{sub.data.source}</p></div>
        </div>
      )}
      {canAct ? (
        <div className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <div><p className={`${field} mb-1.5`}>Plan</p><Select label="Plan" className="w-full" value={plan || current} onChange={setPlan} options={(plans.data ?? []).map((p) => ({ value: p.id, label: p.name }))} /></div>
            <div><p className={`${field} mb-1.5`}>Status</p><Select label="Status" className="w-full" value={status} onChange={setStatus} options={STATUSES} /></div>
            <div><p className={`${field} mb-1.5`}>{status === "trialing" ? "Trial ends" : "Paid until"}</p><DateField value={until} onChange={setUntil} label="Date" /></div>
          </div>
          <input value={reason} onChange={(e) => setReason(e.target.value)} maxLength={300} placeholder="Reason (required, kept in the audit log)" aria-label="Reason" className={inputCls} />
          {chosen && plan && plan !== current && (
            <p className="rise rounded-xl bg-leaf-soft/60 px-3.5 py-2.5 text-xs text-slate-700">Will fix: {Object.entries(chosen.limits).map(([k, v]) => `${k.replace("limits.", "").replace(/_/g, " ")} ${v === 0 ? "unlimited" : v}`).join(", ")}; modules off: {Object.entries(chosen.modules).filter(([, v]) => !v).map(([k]) => k.replace("modules.", "")).join(", ") || "none"}.</p>
          )}
          <div className="flex flex-wrap items-center gap-3">
            <Btn disabled={assign.isPending || reason.trim().length < 3 || !plan || (status === "trialing" && !until)} onClick={() => assign.mutate()}>{assign.isPending ? "Assigning…" : "Assign plan"}</Btn>
            {assign.isSuccess && <span role="status" className="rise text-sm text-leaf-deep">Plan assigned.</span>}
            <span className="text-xs text-muted">After the end date the scheduler returns the organization to Free.</span>
          </div>
        </div>
      ) : <p className="text-sm text-muted">Only an operator with the admin role can assign a plan.</p>}
      <Err e={assign.error} />
    </Card>
  );
}

function Timeline({ id }: { id: string }) {
  const q = useOrgOverview(id);
  if (q.isLoading) return <Skeleton className="h-48" />;
  const tl = q.data?.timeline ?? [];
  return (
    <Card index={0} interactive={false}>
      <CardHeader title="What happened" icon={<IconClock className="size-4 text-leaf-deep" />} />
      {tl.length === 0 ? <EmptyState icon={IconHistory} text="No recorded actions yet." /> : (
        <ol className="relative max-h-[34rem] space-y-1 overflow-y-auto pr-1 before:absolute before:bottom-2 before:left-[15px] before:top-2 before:w-px before:bg-line">
          {tl.map((e, i) => (
            <li key={i} className="rise relative flex gap-4 rounded-xl py-2 pl-1 pr-2 transition-colors hover:bg-slate-50" style={{ "--i": Math.min(i, 10) } as React.CSSProperties}>
              <span className="relative mt-1 grid size-7 shrink-0 place-items-center rounded-full bg-white ring-1 ring-line"><span className="size-2.5 rounded-full bg-leaf" /></span>
              <div className="min-w-0 text-sm">
                <p className="flex flex-wrap items-center gap-x-2 gap-y-1"><code className="rounded-md bg-slate-100 px-1.5 py-0.5 text-xs">{e.action}</code><span className="break-all text-muted">{e.target}</span></p>
                <p className="mt-0.5 text-xs text-muted">{shortTime(e.at)} · {e.actor.length > 28 ? `${e.actor.slice(0, 28)}…` : e.actor}</p>
              </div>
            </li>
          ))}
        </ol>
      )}
    </Card>
  );
}

function ErasurePanel({ id, canAct }: { id: string; canAct: boolean }) {
  const q = useOrgOverview(id);
  const er = useErasure(id);
  const [reason, setReason] = useState("");
  const [grace, setGrace] = useState("14");
  const [typed, setTyped] = useState("");
  if (q.isLoading) return <Skeleton className="h-48" />;
  if (!q.data) return null;
  const o = q.data.overview;
  return (
    <Card index={0} interactive={false} className="ring-1 ring-rose-100">
      <CardHeader title="Erasure" icon={<IconWarning className="size-4 text-[#d9707f]" />} action={o.erasure_requested ? <Pill tone="bad" pulse>Scheduled</Pill> : <Pill>None</Pill>} />
      {o.erasure_requested ? (
        <>
          <p className="text-sm text-slate-700">Scheduled by <b>{o.erasure_requested.by.replace("operator:", "operator ")}</b>; the organization and everything in it is erased on <b>{shortTime(o.erasure_requested.execute_after)}</b> unless cancelled.</p>
          {canAct && <div className="mt-4"><ConfirmButton label="Cancel the erasure" confirmLabel="Cancel erasure" disabled={er.cancel.isPending} onConfirm={() => er.cancel.mutate("cancelled in the console")} /></div>}
        </>
      ) : canAct ? (
        <div className="space-y-3">
          <p className="text-sm text-muted">Schedules the erasure of the organization and all its data after a grace period; it can be cancelled until it runs. The reason is kept in the organization&apos;s audit log.</p>
          <input value={reason} onChange={(e) => setReason(e.target.value)} maxLength={300} placeholder="Reason (required)" aria-label="Reason" className={inputCls} />
          <div className="flex flex-wrap items-center gap-2">
            <Select label="Grace period" value={grace} onChange={setGrace} options={[7, 14, 30, 60, 90].map((d) => ({ value: String(d), label: `${d} days grace` }))} />
            <input value={typed} onChange={(e) => setTyped(e.target.value)} placeholder={`Type ${o.name} to confirm`} aria-label="Type the organization name" className={`${inputCls} min-w-0 flex-1`} />
          </div>
          <ConfirmButton label="Schedule erasure" confirmLabel="Erase after the grace period" disabled={er.request.isPending || reason.trim().length < 5 || typed.trim() !== o.name}
            onConfirm={() => er.request.mutate({ grace_days: Number(grace), reason: reason.trim() })} />
        </div>
      ) : <p className="text-sm text-muted">Only an operator with the admin role can schedule an erasure.</p>}
      <Err e={er.request.error ?? er.cancel.error} />
    </Card>
  );
}

export function OrganizationView({ id }: { id: string }) {
  const me = useOperatorMe();
  const tenants = useOperatorTenants();
  const overview = useOrgOverview(id);
  const org = tenants.data?.find((t) => t.id === id);
  const canAct = me.data?.role === "admin";
  const [tab, setTab] = useState("overview");
  const o = overview.data?.overview;
  const fail = org ? org.connections_in_error + org.failed_deliveries_7d : 0;

  return (
    <div className="space-y-6">
      <Link href="/operator/organizations" className="inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-sm font-medium text-slate-500 transition hover:bg-slate-100 hover:text-ink">
        <ArrowLeft className="size-3.5" aria-hidden />Organizations
      </Link>

      <div className="glass rise flex flex-wrap items-center gap-5 p-5">
        <Avatar name={org?.name ?? "O"} size="lg" />
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-2xl font-bold tracking-tight">{org?.name ?? "Organization"}</h1>
          <p className="mt-0.5 text-sm text-muted">{o ? `Created ${shortTime(o.created_at)} · last activity ${o.last_activity ? shortTime(o.last_activity) : "never"}` : " "}</p>
        </div>
        {org && <div className="flex flex-wrap gap-2">
          <Pill tone={planTone(org.plan)}>{org.plan}</Pill>
          {fail > 0 ? <Pill tone="warn" pulse>needs a look</Pill> : <Pill tone="good">healthy</Pill>}
          {o?.erasure_requested && <Pill tone="bad" pulse>erasure scheduled</Pill>}
        </div>}
      </div>

      <div className="grid gap-3 sm:grid-cols-4 xl:grid-cols-8">
        {([["Members", o?.members], ["Open invitations", o?.invitations_open], ["API keys", o?.api_keys], ["Projects", o?.projects], ["Cloud connections", o?.connections], ["Reports", o?.reports], ["Destinations", o?.destinations], ["Gate checks", o?.gate_checks]] as [string, number | undefined][]).map(([label, v], i) => (
          <Tile key={label} index={i} label={label} value={v} tone={label === "Cloud connections" && o?.connections_failing ? "warn" : undefined} note={label === "Cloud connections" && o?.connections_failing ? `${o.connections_failing} failing` : undefined} />
        ))}
      </div>

      <PillTabs value={tab} onChange={setTab} label="Organization sections" tabs={[
        { value: "overview", label: "Overview" }, { value: "plan", label: "Plan and access" }, { value: "settings", label: "Settings and limits" },
        { value: "history", label: "Change history" }, { value: "erasure", label: "Erasure" },
      ]} />

      <div key={tab} className="page-enter">
        {tab === "overview" && <Timeline id={id} />}
        {tab === "plan" && <div className="grid gap-5 xl:grid-cols-2"><PlanPanel id={id} current={org?.plan ?? "free"} canAct={canAct} /><StatusPanel id={id} canAct={canAct} /></div>}
        {tab === "settings" && <div className="flex items-center gap-2 pb-3 text-sm text-muted"><IconSliders className="size-4" aria-hidden />Values set here apply to this organization only; the rest inherits the platform defaults.</div>}
        {tab === "settings" && <SettingsEditor mode="operator-tenant" path={`operator/tenants/${id}/settings`} historyPath="" canEdit={canAct} />}
        {tab === "history" && <SettingsHistory path={`operator/settings/history?tenant_id=${id}&limit=200`} />}
        {tab === "erasure" && <div className="max-w-2xl"><ErasurePanel id={id} canAct={canAct} /></div>}
      </div>
    </div>
  );
}
