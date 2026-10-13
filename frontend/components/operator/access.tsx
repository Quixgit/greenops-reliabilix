"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState, type CSSProperties } from "react";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { CountUp } from "@/components/ui/count-up";
import { ErrorState } from "@/components/ui/empty-state";
import { IconCheck, IconClock, IconEye, IconLock, IconMinus, IconShield as ShieldCheck, IconUserPlus as UserPlus, IconUsers } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorMe, useOperatorRecords } from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { operatorRecords, type OperatorRecord } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { cn } from "@/lib/utils";
import { PageTitle } from "./parts";
import { Avatar, Btn, inputCls, Pill, Segmented } from "./ui";

const ROLES = [{ value: "viewer", label: "Viewer" }, { value: "admin", label: "Admin" }];

/** What each role may do, in the order a person asks it. The server enforces it; this is only the explanation. */
const CAPABILITIES: { text: string; viewer: boolean }[] = [
  { text: "See organizations, usage and platform metrics", viewer: true },
  { text: "Read the audit log, errors, jobs and notification deliveries", viewer: true },
  { text: "Change platform settings and the source policy", viewer: false },
  { text: "Set keys and addresses of data sources and services", viewer: false },
  { text: "Assign plans, suspend or restore organizations", viewer: false },
  { text: "Retry or remove failed jobs and deliveries, handle errors", viewer: false },
  { text: "Send announcements to organizations", viewer: false },
  { text: "Add and withdraw operators, change their role", viewer: false },
];

const ago = (iso: string | null) => {
  if (!iso) return "never signed in";
  const h = (Date.now() - new Date(iso).getTime()) / 3_600_000;
  if (h < 1) return "just now";
  if (h < 48) return `${Math.round(h)} h ago`;
  return `${Math.round(h / 24)} days ago`;
};

const Err = ({ e }: { e: Error | null }) => (e ? <p role="alert" className="rise mt-3 rounded-xl bg-rose-50 px-3.5 py-2.5 text-sm text-[#a8504b] ring-1 ring-rose-200">{e.message}</p> : null);

function Stat({ icon: Icon, label, value, note, tone, index }: { icon: typeof IconUsers; label: string; value: number | undefined; note?: string; tone: string; index: number }) {
  return (
    <div className="rise flex items-center gap-3.5 rounded-2xl bg-white/70 p-4 ring-1 ring-white/80 backdrop-blur" style={{ "--i": index + 1 } as CSSProperties}>
      <span className={cn("grid size-11 shrink-0 place-items-center rounded-xl", tone)}><Icon className="size-5" strokeWidth={1.7} aria-hidden /></span>
      <div className="min-w-0">
        <p className="text-xs font-medium text-muted">{label}</p>
        <p className="text-2xl font-bold leading-tight tracking-tight tabular-nums">{value === undefined ? <span className="text-slate-300">—</span> : <CountUp value={value} format={(n) => Math.round(n).toLocaleString()} />}</p>
        {note && <p className="text-[11px] text-muted">{note}</p>}
      </div>
    </div>
  );
}

function OperatorRow({ o, index, self, lastAdmin, maxActions, canAct, busy, onRole, onWithdraw }: {
  o: OperatorRecord; index: number; self: boolean; lastAdmin: boolean; maxActions: number; canAct: boolean; busy: boolean;
  onRole: (role: string) => void; onWithdraw: () => void;
}) {
  const name = o.email || o.subject;
  const share = maxActions > 0 ? Math.max(o.actions_30d > 0 ? 6 : 0, Math.round((o.actions_30d / maxActions) * 100)) : 0;
  const editable = canAct && !self && !!o.email && !lastAdmin;
  return (
    <li className="rise group flex flex-wrap items-center gap-x-5 gap-y-3 rounded-2xl bg-white p-4 ring-1 ring-line transition-all duration-300 hover:-translate-y-0.5 hover:shadow-md" style={{ "--i": Math.min(index, 10) } as CSSProperties}>
      <Avatar name={name} size="lg" />
      <div className="min-w-0 flex-1 basis-56">
        <p className="flex flex-wrap items-center gap-x-2 gap-y-1 font-semibold">
          <span className="truncate">{name}</span>
          {self && <Pill tone="info">you</Pill>}
          {lastAdmin && <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-[#77621a] ring-1 ring-amber-100" title="The platform always keeps at least one admin"><IconLock className="size-3" aria-hidden />last admin</span>}
        </p>
        <p className="mt-0.5 flex flex-wrap items-center gap-x-3 text-xs text-muted">
          {o.email && <span className="truncate">{o.subject}</span>}
          <span className="inline-flex items-center gap-1"><IconClock className="size-3" aria-hidden />added {shortTime(o.created_at)}</span>
          <span>active {ago(o.last_seen_at)}</span>
        </p>
      </div>
      <div className="w-36 shrink-0" title={`${o.actions_30d.toLocaleString()} actions in 30 days`}>
        <p className="flex items-baseline justify-between text-xs text-muted"><span>30 days</span><span className="font-semibold tabular-nums text-ink">{o.actions_30d.toLocaleString()}</span></p>
        <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-slate-100"><div className="bar-x h-full rounded-full bg-leaf/70 transition-all duration-700" style={{ width: `${share}%` }} /></div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {editable
          ? <Segmented label={`Role of ${name}`} value={o.role} onChange={onRole} options={ROLES} />
          : <Pill tone={o.role === "admin" ? "good" : "neutral"}>{o.role}</Pill>}
        {canAct && !self && !lastAdmin && <ConfirmButton label="Withdraw" confirmLabel="Withdraw access" disabled={busy} onConfirm={onWithdraw} />}
      </div>
    </li>
  );
}

/** Operators: who may use the console, with which role. Assigned and withdrawn here; the database enforces the rules. */
export function AccessView() {
  const q = useOperatorRecords();
  const me = useOperatorMe();
  const qc = useQueryClient();
  const canAct = me.data?.role === "admin";
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("viewer");
  const done = (data: unknown) => { qc.setQueryData(["op-operators"], data); void qc.invalidateQueries({ queryKey: ["op-audit"] }); };
  const assign = useMutation({
    mutationFn: (v: { email: string; role: string }) => api("operator/operators", operatorRecords, undefined, { method: "PUT", body: JSON.stringify(v) }),
    onSuccess: (data) => { done(data); setEmail(""); },
  });
  const withdraw = useMutation({
    mutationFn: (subject: string) => api("operator/operators/withdraw", operatorRecords, undefined, { method: "POST", body: JSON.stringify({ subject }) }),
    onSuccess: done,
  });
  const d = q.data;
  const admins = d?.filter((o) => o.role === "admin").length ?? 0;
  const active30 = d?.filter((o) => o.actions_30d > 0).length ?? 0;
  const maxActions = useMemo(() => Math.max(0, ...(d ?? []).map((o) => o.actions_30d)), [d]);
  const sorted = useMemo(() => [...(d ?? [])].sort((a, b) => Number(b.role === "admin") - Number(a.role === "admin") || b.actions_30d - a.actions_30d), [d]);

  return (
    <div className="space-y-6">
      <PageTitle title="Access & security" sub="Who may use this console and with which role. A person must have signed in to the platform before they can be made an operator. Every change is written to the audit log." />

      <section aria-label="Summary" className="rise relative overflow-hidden rounded-3xl bg-gradient-to-br from-leaf-soft/70 via-sky-50/60 to-violet-50/50 p-5 ring-1 ring-white/70">
        <span aria-hidden className="pointer-events-none absolute -right-10 -top-12 size-52 rounded-full bg-white/50 blur-3xl" />
        <div className="grid gap-3 sm:grid-cols-3">
          <Stat index={0} icon={IconUsers} label="Operators" value={d?.length} tone="bg-sky-100/70 text-sky-700" />
          <Stat index={1} icon={ShieldCheck} label="Admins" value={d ? admins : undefined} note="the platform always keeps at least one" tone="bg-leaf-soft text-leaf-deep" />
          <Stat index={2} icon={IconEye} label="Active in 30 days" value={d ? active30 : undefined} tone="bg-violet-100/70 text-violet-700" />
        </div>
      </section>

      <section className="space-y-3" aria-labelledby="ops">
        <h2 id="ops" className="flex items-center gap-2 text-[15px] font-semibold tracking-tight"><ShieldCheck className="size-4 text-leaf-deep" aria-hidden />Operators</h2>
        {q.isLoading ? <Skeleton className="h-28" /> : q.isError ? <ErrorState /> : sorted.length === 0 ? <p className="text-sm text-muted">No operators.</p> : (
          <ul className="space-y-3">
            {sorted.map((o, i) => (
              <OperatorRow key={o.subject} o={o} index={i} self={o.subject === me.data?.subject} lastAdmin={o.role === "admin" && admins <= 1}
                maxActions={maxActions} canAct={!!canAct} busy={assign.isPending || withdraw.isPending}
                onRole={(r) => assign.mutate({ email: o.email, role: r })} onWithdraw={() => withdraw.mutate(o.subject)} />
            ))}
          </ul>
        )}
        <Err e={assign.error ?? withdraw.error} />
      </section>

      <div className="grid gap-5 xl:grid-cols-5">
        {canAct && (
          <section className="glass rise h-fit p-5 xl:col-span-2" style={{ "--i": 2 } as CSSProperties}>
            <h2 className="mb-1 flex items-center gap-2 text-[15px] font-semibold tracking-tight"><UserPlus className="size-4 text-leaf-deep" aria-hidden />Add an operator</h2>
            <p className="mb-4 text-xs text-muted">The person must have signed in to the platform once, so their account exists.</p>
            <form className="space-y-3.5" onSubmit={(e) => { e.preventDefault(); if (email.trim()) assign.mutate({ email: email.trim(), role }); }}>
              <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="name@company.com" aria-label="E-mail" autoComplete="off" className={inputCls} />
              <div className="flex flex-wrap items-center gap-3">
                <Segmented label="Role" value={role} onChange={setRole} options={ROLES} />
                <Btn type="submit" disabled={assign.isPending || !email.trim()}>{assign.isPending ? "Adding…" : "Add operator"}</Btn>
              </div>
              <p className="rounded-xl bg-slate-50 px-3 py-2 text-xs text-slate-600 ring-1 ring-line/70">
                {role === "admin" ? "An admin can change the platform and its keys. Give it to as few people as you can." : "A viewer can read everything and change nothing: the safe default."}
              </p>
            </form>
          </section>
        )}

        <section className={cn("glass rise p-5", canAct ? "xl:col-span-3" : "xl:col-span-5")} style={{ "--i": 3 } as CSSProperties}>
          <h2 className="mb-4 flex items-center gap-2 text-[15px] font-semibold tracking-tight"><IconLock className="size-4 text-leaf-deep" aria-hidden />What each role can do</h2>
          <div className="thin-scroll overflow-x-auto">
            <table className="w-full min-w-[420px] border-separate border-spacing-0 text-sm">
              <thead>
                <tr className="text-xs uppercase tracking-wide text-slate-500">
                  <th className="pb-2 text-left font-semibold">Capability</th>
                  <th className="w-20 pb-2 text-center font-semibold"><Pill>viewer</Pill></th>
                  <th className="w-20 pb-2 text-center font-semibold"><Pill tone="good">admin</Pill></th>
                </tr>
              </thead>
              <tbody>
                {CAPABILITIES.map((c, i) => (
                  <tr key={c.text} className="rise" style={{ "--i": i } as CSSProperties}>
                    <td className="border-t border-line/60 py-2.5 pr-3">{c.text}</td>
                    <td className="border-t border-line/60 py-2.5 text-center">{c.viewer ? <IconCheck className="mx-auto size-4 text-leaf-deep" strokeWidth={2.6} aria-label="allowed" /> : <IconMinus className="mx-auto size-4 text-slate-300" aria-label="not allowed" />}</td>
                    <td className="border-t border-line/60 py-2.5 text-center"><IconCheck className="mx-auto size-4 text-leaf-deep" strokeWidth={2.6} aria-label="allowed" /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <section className="rise grid gap-3 md:grid-cols-3" style={{ "--i": 4 } as CSSProperties}>
        {[
          { title: "Separate from organizations", text: "An owner or admin of an organization has no access here, and an operator has no automatic access to an organization's data." },
          { title: "Everything is on the record", text: "Every read and change is in the audit log with who, what and when.", link: { href: "/operator/audit", label: "Open the audit log" } },
          { title: "Safeguards", text: "You cannot change your own role, and the platform always keeps at least one admin. Time-limited support access and SSO or multi-factor sign-in are planned." },
        ].map((x) => (
          <div key={x.title} className="rounded-2xl bg-white/70 p-4 ring-1 ring-line/70">
            <p className="font-semibold">{x.title}</p>
            <p className="mt-1.5 text-sm text-muted">{x.text}</p>
            {x.link && <Link href={x.link.href} className="mt-2 inline-block text-sm font-medium text-primary hover:underline">{x.link.label} →</Link>}
          </div>
        ))}
      </section>
    </div>
  );
}
