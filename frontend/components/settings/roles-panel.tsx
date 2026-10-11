"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Lock, Pencil, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useRoles } from "@/hooks/use-admin";
import { useT } from "@/i18n/provider";
import { api, apiVoid } from "@/lib/api/client";
import { customRole, type CustomRole, type RolesView } from "@/lib/api/schemas";
import { Panel } from "./organization-tab";

const field = "w-full rounded-xl bg-white/80 px-3 py-2 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-primary";

type Draft = { id?: string; name: string; description: string; permissions: Set<string> };

function Editor({ view, draft, onClose }: { view: RolesView; draft: Draft; onClose: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const [d, setD] = useState<Draft>(draft);
  const groups = useMemo(() => {
    const m = new Map<string, string[]>();
    for (const p of view.permissions.filter((x) => x.assignable)) m.set(p.group, [...(m.get(p.group) ?? []), p.key]);
    return [...m.entries()];
  }, [view]);
  const save = useMutation({
    mutationFn: () => api(d.id ? `roles/${d.id}` : "roles", customRole, undefined, {
      method: d.id ? "PUT" : "POST", body: JSON.stringify({ name: d.name, description: d.description, permissions: [...d.permissions].sort() }),
    }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["roles"] }); void qc.invalidateQueries({ queryKey: ["members"] }); onClose(); },
  });
  const toggle = (k: string) => setD((x) => { const n = new Set(x.permissions); if (n.has(k)) n.delete(k); else n.add(k); return { ...x, permissions: n }; });
  return (
    <Panel icon={ShieldCheck} title={d.id ? t("roles.edit") : t("roles.new")} className="lg:col-span-2">
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
      <p className="mt-3 text-xs text-muted">{t("roles.limits_note")}</p>
      {save.isError && <p role="alert" className="mt-2 text-sm text-bad">{save.error.message}</p>}
      <div className="mt-4 flex gap-2">
        <button type="button" disabled={save.isPending || d.name.trim().length < 2 || d.permissions.size === 0} onClick={() => save.mutate()}
          className="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-all hover:bg-emerald-700 disabled:opacity-50">{t("common.save")}</button>
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
  const remove = useMutation({
    mutationFn: (id: string) => apiVoid(`roles/${id}`, { method: "DELETE" }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["roles"] }); void qc.invalidateQueries({ queryKey: ["members"] }); },
  });
  if (q.isLoading) return <Skeleton className="h-48" />;
  if (q.isError || !q.data) return <ErrorState />;
  const view = q.data;
  const start = (r?: CustomRole) => setEditing({ id: r?.id, name: r?.name ?? "", description: r?.description ?? "", permissions: new Set(r?.permissions ?? []) });
  return (
    <div className="grid items-start gap-4 lg:grid-cols-2">
      {editing && <Editor view={view} draft={editing} onClose={() => setEditing(null)} />}
      <Panel index={0} icon={ShieldCheck} title={t("roles.custom")} hint={t("settings.roles.custom_hint")} action={!editing && (
        <button type="button" onClick={() => start()} className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-50 px-3 py-1.5 text-sm font-medium text-emerald-700 transition-colors hover:bg-emerald-100"><Plus className="size-4" aria-hidden />{t("roles.new")}</button>
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

      <Panel index={1} icon={Lock} title={t("roles.builtin")} hint={t("settings.roles.builtin_hint")}>
        <ul className="text-sm">
          {view.builtin.map((b) => (
            <li key={b.role} className="border-t border-line/70 py-3">
              <p className="font-medium capitalize">{b.role}</p>
              <p className="mt-1 flex flex-wrap gap-1">{b.permissions.map((p) => <code key={p} className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px]">{p}</code>)}</p>
            </li>
          ))}
        </ul>
      </Panel>
    </div>
  );
}
