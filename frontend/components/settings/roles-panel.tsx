"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconChevronDown as ChevronDown, IconPencil as Pencil, IconTrash as Trash2, IconLock as Lock, IconPlus as Plus, IconShield as ShieldCheck } from "@/components/ui/icons";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useMembers, useRoles } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import { api, apiVoid } from "@/lib/api/client";
import { customRole, member as memberSchema, type CustomRole, type RolesView } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { Panel } from "./organization-tab";

const field = "w-full rounded-xl bg-white/80 px-3 py-2 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-leaf/40";

type Draft = { id?: string; name: string; description: string; permissions: Set<string> };

/** Ready-made starting points: a name, what the role is for and the permissions that fit it. The person only chooses who gets it. */
const TEMPLATES: { id: string; permissions: string[] }[] = [
  { id: "finops", permissions: ["project:read", "usage:read", "carbon:read", "billing:read", "budget:write", "report:read", "report:write", "recommendation:read"] },
  { id: "sustainability", permissions: ["project:read", "usage:read", "carbon:read", "carbon:compute", "report:read", "report:write", "recommendation:read", "recommendation:apply"] },
  { id: "developer", permissions: ["project:read", "carbon:read", "recommendation:read", "ci:evaluate", "automation:read"] },
  { id: "auditor", permissions: ["project:read", "usage:read", "carbon:read", "report:read", "audit:read", "settings:read", "notification:read"] },
  { id: "approver", permissions: ["project:read", "recommendation:read", "recommendation:apply", "automation:read", "automation:approve"] },
];

function Editor({ view, draft, onClose }: { view: RolesView; draft: Draft; onClose: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const [d, setD] = useState<Draft>(draft);
  const members = useMembers().data ?? [];
  const [who, setWho] = useState<Set<string>>(new Set());
  const [tpl, setTpl] = useState<string | null>(null);
  const assignable = new Set(view.permissions.filter((x) => x.assignable).map((x) => x.key));
  const applyTemplate = (id: string) => {
    const tp = TEMPLATES.find((x) => x.id === id)!;
    setTpl(id);
    setD((x) => ({ ...x, name: t(`roles.tpl.${id}`), description: t(`roles.tpl.${id}_desc`), permissions: new Set(tp.permissions.filter((k) => assignable.has(k))) }));
  };
  const groups = useMemo(() => {
    const m = new Map<string, string[]>();
    for (const p of view.permissions.filter((x) => x.assignable)) m.set(p.group, [...(m.get(p.group) ?? []), p.key]);
    return [...m.entries()];
  }, [view]);
  const save = useMutation({
    mutationFn: async () => {
      const role = await api(d.id ? `roles/${d.id}` : "roles", customRole, undefined, {
        method: d.id ? "PUT" : "POST", body: JSON.stringify({ name: d.name, description: d.description, permissions: [...d.permissions].sort() }),
      });
      // Give the new role to the people chosen in the same step.
      for (const id of who) await api(`members/${id}`, memberSchema, undefined, { method: "PATCH", body: JSON.stringify({ custom_role_id: role.id }) });
      return role;
    },
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["roles"] }); void qc.invalidateQueries({ queryKey: ["members"] }); onClose(); },
  });
  const toggle = (k: string) => setD((x) => { const n = new Set(x.permissions); if (n.has(k)) n.delete(k); else n.add(k); return { ...x, permissions: n }; });
  return (
    <Panel icon={ShieldCheck} title={d.id ? t("roles.edit") : t("roles.new")} className="lg:col-span-2">
      {!d.id && (
        <div className="mb-4">
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">{t("roles.start_from")}</p>
          <div className="flex flex-wrap gap-2">
            {TEMPLATES.map((x) => (
              <button key={x.id} type="button" onClick={() => applyTemplate(x.id)} aria-pressed={tpl === x.id}
                className={cn("rounded-xl border px-3 py-2 text-left text-sm transition-all hover:-translate-y-0.5", tpl === x.id ? "border-leaf bg-leaf-soft/60" : "border-line bg-white hover:border-leaf/50")}>
                <span className="block font-semibold">{t(`roles.tpl.${x.id}`)}</span>
                <span className="block text-xs text-muted">{t(`roles.tpl.${x.id}_desc`)}</span>
              </button>
            ))}
          </div>
        </div>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block text-sm"><span className="mb-1 block font-medium">{t("roles.name")}</span><input value={d.name} maxLength={60} onChange={(e) => setD({ ...d, name: e.target.value })} className={field} /></label>
        <label className="block text-sm"><span className="mb-1 block font-medium">{t("roles.description")}</span><input value={d.description} maxLength={200} onChange={(e) => setD({ ...d, description: e.target.value })} className={field} /></label>
      </div>
      <div className="mt-4 grid gap-4 sm:grid-cols-2">
        {groups.map(([group, keys]) => (
          <fieldset key={group} className="rounded-xl p-3 ring-1 ring-line">
            <legend className="px-1 text-xs font-semibold uppercase tracking-wide text-muted">{t(`perm.group.${group}`, undefined, group)}</legend>
            <ul className="space-y-1.5">
              {keys.map((k) => (
                <li key={k}><label className="flex cursor-pointer items-start gap-2 text-sm">
                  <input type="checkbox" checked={d.permissions.has(k)} onChange={() => toggle(k)} className="mt-0.5 size-4 accent-[var(--color-primary)]" />
                  <span>{t(`perm.${k}`, undefined, k)}<code className="ml-2 text-[11px] text-muted">{k}</code></span>
                </label></li>
              ))}
            </ul>
          </fieldset>
        ))}
      </div>
      {!d.id && members.length > 0 && (
        <div className="mt-4">
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">{t("roles.give_to")}</p>
          <ul className="flex flex-wrap gap-2">
            {members.filter((m) => m.role !== "owner").map((m) => {
              const on = who.has(m.id);
              return (
                <li key={m.id}>
                  <button type="button" aria-pressed={on} onClick={() => setWho((x) => { const n = new Set(x); if (n.has(m.id)) n.delete(m.id); else n.add(m.id); return n; })}
                    className={cn("flex items-center gap-2 rounded-full border py-1 pl-1 pr-3 text-sm transition-all", on ? "border-leaf bg-leaf-soft/60" : "border-line bg-white hover:border-leaf/50")}>
                    <span className={cn("grid size-6 place-items-center rounded-full text-[10px] font-bold", on ? "bg-leaf text-white" : "bg-slate-100 text-slate-500")}>{(m.email || m.user_id).slice(0, 2).toUpperCase()}</span>
                    {m.email || m.user_id}
                  </button>
                </li>
              );
            })}
          </ul>
          <p className="mt-1.5 text-xs text-muted">{t("roles.give_hint")}</p>
        </div>
      )}
      <p className="mt-3 text-xs text-muted">{t("roles.limits_note")}</p>
      {save.isError && <p role="alert" className="mt-2 text-sm text-bad">{save.error.message}</p>}
      <div className="mt-4 flex gap-2">
        <button type="button" disabled={save.isPending || d.name.trim().length < 2 || d.permissions.size === 0} onClick={() => save.mutate()}
          className="rounded-xl btn-leaf px-4 py-2 text-sm font-semibold text-white disabled:opacity-50">{t("common.save")}</button>
        <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm font-medium ring-1 ring-line hover:bg-white">{t("common.cancel")}</button>
      </div>
    </Panel>
  );
}

/** Built-in roles (read-only) and the organization's own roles: define a set of permissions, then give it to members. */
export function RolesPanel() {
  const t = useT();
  const qc = useQueryClient();
  const q = useRoles();
  const [editing, setEditing] = useState<Draft | null>(null);
  const [openRole, setOpenRole] = useState<string | null>(null);
  const remove = useMutation({
    mutationFn: (id: string) => apiVoid(`roles/${id}`, { method: "DELETE" }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["roles"] }); void qc.invalidateQueries({ queryKey: ["members"] }); },
  });
  if (q.isLoading) return <Skeleton className="h-48" />;
  if (q.isError || !q.data) return <ErrorState />;
  const view = q.data;
  const start = (r?: CustomRole) => setEditing({ id: r?.id, name: r?.name ?? "", description: r?.description ?? "", permissions: new Set(r?.permissions ?? []) });
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {editing && <Editor view={view} draft={editing} onClose={() => setEditing(null)} />}
      <Panel info={t("info.set_roles_custom")} index={0} icon={ShieldCheck} title={t("roles.custom")} hint={t("settings.roles.custom_hint")} action={!editing && (
        <button type="button" onClick={() => start()} className="inline-flex items-center gap-1.5 rounded-lg bg-leaf-soft px-3 py-1.5 text-sm font-medium text-leaf-deep transition-colors hover:bg-leaf-soft"><Plus className="size-4" aria-hidden />{t("roles.new")}</button>
      )}>
        {view.custom.length === 0 ? <p className="border-t border-line/70 pt-3 text-sm text-muted">{t("roles.none")}</p> : (
          <ul className="text-sm">
            {view.custom.map((r) => (
              <li key={r.id} className="flex flex-wrap items-start justify-between gap-3 border-t border-line/70 py-3">
                <div className="min-w-0">
                  <p className="font-medium">{r.name} <Badge className="ml-1">{t("roles.members", { count: r.members })}</Badge></p>
                  {r.description && <p className="text-xs text-muted">{r.description}</p>}
                  <p className="mt-1 flex flex-wrap gap-1">{r.permissions.map((p) => <code key={p} className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px]">{p}</code>)}</p>
                </div>
                <span className="flex shrink-0 items-center gap-2">
                  <button type="button" onClick={() => start(r)} className="inline-flex items-center gap-1 rounded-lg px-2.5 py-1 text-xs font-medium ring-1 ring-line hover:bg-white"><Pencil className="size-3.5" aria-hidden />{t("roles.edit")}</button>
                  <ConfirmButton label={t("roles.delete")} confirmLabel={t("roles.delete_confirm")} disabled={remove.isPending} onConfirm={() => remove.mutate(r.id)} className="inline-flex items-center gap-1" />
                  <Trash2 className="hidden" aria-hidden />
                </span>
              </li>
            ))}
          </ul>
        )}
        {remove.isError && <p role="alert" className="mt-2 text-sm text-bad">{remove.error.message}</p>}
        <p className="mt-3 text-xs text-muted">{t("roles.assign_hint")}</p>
      </Panel>

      <Panel info={t("info.set_roles_builtin")} index={1} icon={Lock} title={t("roles.builtin")} hint={t("settings.roles.builtin_hint")}>
        <ul className="text-sm">
          {view.builtin.map((b) => {
            const isOpen = openRole === b.role;
            return (
              <li key={b.role} className="border-t border-line/70">
                <button type="button" aria-expanded={isOpen} onClick={() => setOpenRole(isOpen ? null : b.role)}
                  className="group flex w-full items-center justify-between gap-3 py-2.5 text-left transition-colors hover:bg-slate-50/70">
                  <span className="font-medium capitalize">{b.role}</span>
                  <span className="flex items-center gap-2 text-xs text-muted">{t("settings.roles.permissions", { count: b.permissions.length })}
                    <ChevronDown className={cn("size-4 transition-transform duration-300", isOpen && "rotate-180")} /></span>
                </button>
                <div className="expand" data-open={isOpen}>
                  <div><p className="flex flex-wrap gap-1 pb-3">{b.permissions.map((p) => <code key={p} className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px]">{p}</code>)}</p></div>
                </div>
              </li>
            );
          })}
        </ul>
      </Panel>
    </div>
  );
}
