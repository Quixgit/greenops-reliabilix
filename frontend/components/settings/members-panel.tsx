"use client";

import { SelectField } from "@/components/ui/select-field";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconUserPlus as UserPlus, IconClock as Clock, IconUsers as Users } from "@/components/ui/icons";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Field, input } from "@/components/forms/project-form";
import { Badge } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { SecretOnce } from "@/components/ui/secret-once";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useInvitations, useMembers, useRoles } from "@/hooks/use-admin";
import { api, apiVoid } from "@/lib/api/client";
import { invitationCreated, member } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { useT } from "@/i18n/provider";
import { Panel } from "./organization-tab";

const MEMBER_ROLES = ["owner", "admin", "engineer", "viewer", "billing"] as const;
const INVITE_ROLES = ["admin", "engineer", "viewer", "billing"] as const;

const inviteSchema = z.object({ email: z.string().trim().email("Enter a valid e-mail"), role: z.enum(INVITE_ROLES) });
type InviteValues = z.infer<typeof inviteSchema>;

function Invite() {
  const qc = useQueryClient();
  const t = useT();
  const [token, setToken] = useState<string | null>(null);
  const [sentTo, setSentTo] = useState<string | null>(null);
  const { register, handleSubmit, reset, formState: { errors } } = useForm<InviteValues>({ resolver: zodResolver(inviteSchema), defaultValues: { role: "viewer" } });
  const m = useMutation({
    mutationFn: (v: InviteValues) => api("invitations", invitationCreated, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => { setToken(r.token); setSentTo(r.email_sent ? r.invitation.email : null); reset({ email: "", role: "viewer" }); void qc.invalidateQueries({ queryKey: ["invitations"] }); },
  });
  return (
    <div className="space-y-3">
      {token && <SecretOnce title="Invitation token" value={token} onDismiss={() => setToken(null)} />}
      {token && <p className="text-xs text-muted">{sentTo ? t("settings.invite.sent", { email: sentTo }) : t("settings.invite.nomail")}</p>}
      <form onSubmit={handleSubmit((v) => m.mutate(v))} className="grid gap-3" noValidate>
        <Field label="E-mail" error={errors.email?.message}><input type="email" {...register("email")} className={input} placeholder="colleague@yourcompany.com" /></Field>
        <Field label="Role"><select {...register("role")} className={input}>{INVITE_ROLES.map((r) => <option key={r} value={r}>{r}</option>)}</select></Field>
        <button disabled={m.isPending} className="mt-6 rounded-xl btn-leaf px-4 py-2.5 text-sm font-semibold text-white active:scale-[0.98] disabled:opacity-60">{m.isPending ? "Inviting…" : "Invite"}</button>
        {m.isError && <p role="alert" className="text-sm text-bad">{m.error.message}</p>}
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
  const t = useT();
  const pg = usePaged(members.data ?? [], 10);
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Panel info={t("info.set_members")} index={0} icon={Users} title={t("settings.users.members")} hint={t("settings.users.members_hint", { count: members.data?.length ?? 0 })} className="lg:col-span-2">
        {members.isLoading ? <Skeleton className="h-24" /> : members.isError ? <ErrorState /> : members.data!.length === 0 ? <EmptyState icon={Users} text="No members." /> : (
          <ul className="text-sm">
            {pg.slice.map((m) => (
              <li key={m.id} className="flex flex-wrap items-center justify-between gap-3 border-t border-line/70 py-2.5 transition-colors hover:bg-slate-50/70">
                <span className="flex min-w-0 items-center gap-3">
                  <span className="grid size-8 shrink-0 place-items-center rounded-full bg-gradient-to-br from-leaf to-leaf-deep text-xs font-bold text-white">{(m.email || m.user_id).slice(0, 2).toUpperCase()}</span>
                  <span className="min-w-0"><span className="block truncate font-medium">{m.email || m.user_id}</span><span className="block text-xs text-muted">{t("settings.users.joined", { when: shortTime(m.created_at) })}</span></span>
                </span>
                <span className="flex items-center gap-2">
                  <SelectField aria-label={`Role of ${m.email}`} value={m.custom_role ? `custom:${m.custom_role.id}` : `role:${m.role}`} disabled={role.isPending} onChange={(e) => role.mutate({ id: m.id, value: e.target.value })} className="rounded-lg border border-line bg-white px-2 py-1 text-xs outline-none focus:border-leaf-deep">
                    <optgroup label="Built-in">{MEMBER_ROLES.map((r) => <option key={r} value={`role:${r}`}>{r}</option>)}</optgroup>
                    {roles.data && roles.data.custom.length > 0 && <optgroup label="Custom">{roles.data.custom.map((r) => <option key={r.id} value={`custom:${r.id}`}>{r.name}</option>)}</optgroup>}
                  </SelectField>
                  <ConfirmButton label="Remove" disabled={remove.isPending} onConfirm={() => remove.mutate(m.id)} />
                </span>
              </li>
            ))}
          </ul>
        )}
        <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
        {err && <p role="alert" className="mt-2 text-sm text-bad">{err.message}</p>}
      </Panel>
      <div className="flex flex-col gap-4 [&>section]:flex-1">
        <Panel info={t("info.set_invite")} index={1} icon={UserPlus} title={t("settings.users.invite")} hint={t("settings.users.invite_hint")}><Invite /></Panel>
        {pending.length > 0 && (
          <Panel info={t("info.set_pending")} index={2} icon={Clock} title={t("settings.users.pending")} hint={t("settings.users.pending_hint", { count: pending.length })}>
            <ul className="text-sm">
              {pending.map((i) => (
                <li key={i.id} className="flex flex-wrap items-center justify-between gap-2 border-t border-line/70 py-2.5">
                  <span className="min-w-0 truncate font-medium">{i.email}</span>
                  <span className="flex items-center gap-2 text-xs text-muted"><Badge>{i.role}</Badge>{shortTime(i.expires_at)}</span>
                </li>
              ))}
            </ul>
          </Panel>
        )}
      </div>
    </div>
  );
}
