"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ShieldCheck, UserPlus } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorMe, useOperatorRecords } from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { operatorRecords } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { PageTitle, selectCls, th } from "./parts";

/** Operators: who may use the console, with which role. Assigned and withdrawn here; the database enforces the rules. */
export function AccessView() {
  const q = useOperatorRecords();
  const me = useOperatorMe();
  const qc = useQueryClient();
  const canAct = me.data?.role === "admin";
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<"viewer" | "admin">("viewer");
  const done = (data: unknown) => { qc.setQueryData(["op-operators"], data); void qc.invalidateQueries({ queryKey: ["op-audit"] }); };
  const assign = useMutation({
    mutationFn: (v: { email: string; role: string }) => api("operator/operators", operatorRecords, undefined, { method: "PUT", body: JSON.stringify(v) }),
    onSuccess: (data) => { done(data); setEmail(""); },
  });
  const withdraw = useMutation({
    mutationFn: (subject: string) => api("operator/operators/withdraw", operatorRecords, undefined, { method: "POST", body: JSON.stringify({ subject }) }),
    onSuccess: done,
  });
  const err = assign.error ?? withdraw.error;
  const admins = q.data?.filter((o) => o.role === "admin").length ?? 0;

  return (
    <div className="space-y-5">
      <PageTitle title="Access & security" sub="Who may use this console and with which role. A person must have signed in to the platform before they can be made an operator. Every change is written to the audit log." />
      <Card index={0} interactive={false}>
        <CardHeader title="Operators" icon={<ShieldCheck className="size-4 text-primary" aria-hidden />} />
        {q.isLoading ? <Skeleton className="h-28" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={ShieldCheck} text="No operators." /> : (
          <div className="overflow-x-auto"><table className="w-full text-sm">
            <thead><tr><th className={th}>Operator</th><th className={th}>Role</th><th className={th}>Added</th><th className={th}>Last active</th><th className={th}>Actions (30 d)</th><th /></tr></thead>
            <tbody className="divide-y divide-line/70">{q.data!.map((o) => {
              const self = o.subject === me.data?.subject;
              const lastAdmin = o.role === "admin" && admins <= 1;
              return (
                <tr key={o.subject}>
                  <td className="py-2 pr-3 font-medium">{o.email || o.subject}{self && <Badge className="ml-2" tone="neutral">you</Badge>}{o.email && <span className="block text-xs font-normal text-muted">{o.subject}</span>}</td>
                  <td className="pr-3">
                    {canAct && !self && o.email ? (
                      <select aria-label={`Role of ${o.email}`} value={o.role} disabled={assign.isPending || lastAdmin} onChange={(e) => assign.mutate({ email: o.email, role: e.target.value })} className={selectCls}>
                        <option value="viewer">viewer</option><option value="admin">admin</option>
                      </select>
                    ) : <Badge tone={o.role === "admin" ? "good" : "neutral"}>{o.role}</Badge>}
                  </td>
                  <td className="pr-3 text-muted">{shortTime(o.created_at)}</td><td className="pr-3 text-muted">{shortTime(o.last_seen_at)}</td><td className="pr-3">{o.actions_30d}</td>
                  <td className="text-right">
                    {canAct && !self && !lastAdmin && <ConfirmButton label="Withdraw" confirmLabel="Withdraw access" disabled={withdraw.isPending} onConfirm={() => withdraw.mutate(o.subject)} />}
                  </td>
                </tr>
              );
            })}</tbody>
          </table></div>
        )}
        {err && <p role="alert" className="mt-3 text-sm text-bad">{err.message}</p>}
      </Card>

      {canAct && (
        <Card index={1} interactive={false}>
          <CardHeader title="Add an operator" icon={<UserPlus className="size-4 text-primary" aria-hidden />} />
          <form className="flex flex-wrap items-center gap-2" onSubmit={(e) => { e.preventDefault(); if (email.trim()) assign.mutate({ email: email.trim(), role }); }}>
            <input type="email" required value={email} onChange={(e) => setEmail(e.target.value)} placeholder="E-mail of a person who has signed in" aria-label="E-mail" className={`${selectCls} w-full sm:w-96`} />
            <select aria-label="Role" value={role} onChange={(e) => setRole(e.target.value as "viewer" | "admin")} className={selectCls}><option value="viewer">viewer</option><option value="admin">admin</option></select>
            <button type="submit" disabled={assign.isPending || !email.trim()} className="brand-gradient rounded-xl px-4 py-2 text-sm font-semibold text-white disabled:opacity-50">{assign.isPending ? "Adding…" : "Add operator"}</button>
          </form>
          <p className="mt-2 text-xs text-muted">You cannot change your own role, and the platform always keeps at least one admin.</p>
        </Card>
      )}

      <Card index={2}>
        <CardHeader title="Roles" />
        <ul className="space-y-2 text-sm">
          <li><b>viewer</b>: reads metadata of every organization. Cannot change anything.</li>
          <li><b>admin</b>: also changes platform settings, data source settings, plans, suspends organizations, handles errors and deliveries, and manages operators.</li>
          <li>Organization roles are separate: an owner or admin of an organization has no access here, and an operator has no automatic access to an organization’s data. Organization roles are managed in that organization’s Settings.</li>
          <li>Every read and change is in the <Link href="/operator/audit" className="text-primary hover:underline">audit log</Link>. Time-limited support access and sign-in requirements (SSO, multi-factor) are planned.</li>
        </ul>
      </Card>
    </div>
  );
}
