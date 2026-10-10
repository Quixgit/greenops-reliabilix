"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Users } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Field, input } from "@/components/forms/project-form";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { SecretOnce } from "@/components/ui/secret-once";
import { Skeleton } from "@/components/ui/skeleton";
import { useInvitations, useMembers, useRoles } from "@/hooks/use-admin";
import { api, apiVoid } from "@/lib/api/client";
import { invitationCreated, member } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";

const MEMBER_ROLES = ["owner", "admin", "engineer", "viewer", "billing"] as const;
const INVITE_ROLES = ["admin", "engineer", "viewer", "billing"] as const;

const inviteSchema = z.object({ email: z.string().trim().email("Enter a valid e-mail"), role: z.enum(INVITE_ROLES) });
type InviteValues = z.infer<typeof inviteSchema>;

function Invite() {
  const qc = useQueryClient();
  const [token, setToken] = useState<string | null>(null);
  const { register, handleSubmit, reset, formState: { errors } } = useForm<InviteValues>({ resolver: zodResolver(inviteSchema), defaultValues: { role: "viewer" } });
  const m = useMutation({
    mutationFn: (v: InviteValues) => api("invitations", invitationCreated, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => { setToken(r.token); reset({ email: "", role: "viewer" }); void qc.invalidateQueries({ queryKey: ["invitations"] }); },
  });
  return (
    <div className="space-y-3">
      {token && <SecretOnce title="Invitation token" value={token} onDismiss={() => setToken(null)} />}
      {token && <p className="text-xs text-muted">E-mail invitations are not set up yet (stub): pass this token to the person over a secure channel. It expires in 7 days and only works for the invited address.</p>}
      <form onSubmit={handleSubmit((v) => m.mutate(v))} className="grid gap-3 sm:grid-cols-[2fr_1fr_auto] sm:items-start" noValidate>
        <Field label="E-mail" error={errors.email?.message}><input type="email" {...register("email")} className={input} placeholder="colleague@yourcompany.com" /></Field>
        <Field label="Role"><select {...register("role")} className={input}>{INVITE_ROLES.map((r) => <option key={r} value={r}>{r}</option>)}</select></Field>
        <button disabled={m.isPending} className="brand-gradient mt-6 rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{m.isPending ? "Inviting…" : "Invite"}</button>
        {m.isError && <p role="alert" className="text-sm text-bad sm:col-span-3">{m.error.message}</p>}
      </form>
    </div>
  );
}

export function MembersPanel() {
  const qc = useQueryClient();
  const members = useMembers();
  const invites = useInvitations();
  const refresh = () => void qc.invalidateQueries({ queryKey: ["members"] });
  const roles = useRoles();
  // The value is "role:<built-in>" or "custom:<id>": one select holds both kinds.
  const role = useMutation({
    mutationFn: (v: { id: string; value: string }) => {
      const [kind, key] = v.value.split(":");
      return api(`members/${v.id}`, member, undefined, { method: "PATCH", body: JSON.stringify(kind === "custom" ? { custom_role_id: key } : { role: key }) });
    },
    onSuccess: () => { refresh(); void qc.invalidateQueries({ queryKey: ["roles"] }); },
  });
  const remove = useMutation({ mutationFn: (id: string) => apiVoid(`members/${id}`, { method: "DELETE" }), onSuccess: refresh });
  const err = role.error ?? remove.error;
  const pending = invites.data?.filter((i) => !i.accepted_at) ?? [];
  return (
    <div className="space-y-5">
      <Card><CardHeader title="Invite someone" /><Invite /></Card>
      <Card>
        <CardHeader title="Members" />
        {members.isLoading ? <Skeleton className="h-24" /> : members.isError ? <ErrorState /> : members.data!.length === 0 ? <EmptyState icon={Users} text="No members." /> : (
          <ul className="divide-y divide-line/70 text-sm">
            {members.data!.map((m) => (
              <li key={m.id} className="flex flex-wrap items-center justify-between gap-3 py-2.5">
                <span className="min-w-0 truncate font-medium">{m.email || m.user_id}</span>
                <span className="flex items-center gap-2">
                  <select aria-label={`Role of ${m.email}`} value={m.custom_role ? `custom:${m.custom_role.id}` : `role:${m.role}`} disabled={role.isPending} onChange={(e) => role.mutate({ id: m.id, value: e.target.value })} className="rounded-lg bg-white/80 px-2 py-1 text-xs ring-1 ring-line">
                    <optgroup label="Built-in">{MEMBER_ROLES.map((r) => <option key={r} value={`role:${r}`}>{r}</option>)}</optgroup>
                    {roles.data && roles.data.custom.length > 0 && <optgroup label="Custom">{roles.data.custom.map((r) => <option key={r.id} value={`custom:${r.id}`}>{r.name}</option>)}</optgroup>}
                  </select>
                  <ConfirmButton label="Remove" disabled={remove.isPending} onConfirm={() => remove.mutate(m.id)} />
                </span>
              </li>
            ))}
          </ul>
        )}
        {err && <p role="alert" className="mt-2 text-sm text-bad">{err.message}</p>}
      </Card>
      {pending.length > 0 && (
        <Card>
          <CardHeader title="Pending invitations" />
          <ul className="divide-y divide-line/70 text-sm">
            {pending.map((i) => (
              <li key={i.id} className="flex flex-wrap items-center justify-between gap-2 py-2.5">
                <span className="font-medium">{i.email}</span>
                <span className="flex items-center gap-2 text-xs text-muted"><Badge>{i.role}</Badge>expires {shortTime(i.expires_at)}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}
