"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { History, RotateCcw, Search } from "lucide-react";
import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import { IconBell as Bell, IconBell as BellRing, IconChart as BarChart3, IconDatabase as Database, IconGlobe as Globe2, IconLeaf as Leaf, IconLock as Lock, IconRefresh as RefreshCw, IconReport as FileText, IconSliders as Settings2 } from "@/components/ui/icons";
import { useMemo, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Panel } from "./organization-tab";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiError, api } from "@/lib/api/client";
import { settingChanges, settingsView, type SettingItem, type SettingsView } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { Check_, Switch } from "@/components/operator/ui";
import { cn } from "@/lib/utils";

/**
 * Where the settings live and what saving means:
 *  - tenant:          an organization edits its own settings,
 *  - platform:        an operator edits platform-wide values (the default every organization inherits),
 *  - operator-tenant: an operator fixes values for ONE organization (they become locked for it).
 */
export type EditorMode = "tenant" | "platform" | "operator-tenant";

const GROUP_ICON: Record<string, LucideIcon> = {
  reporting: BarChart3, alerts: BellRing, retention: Database, security: Lock, notifications: Bell, carbon: Leaf, sync: RefreshCw, reports: FileText, i18n: Globe2,
};
const inputCls = "w-full rounded-xl bg-white/80 px-3 py-2 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-leaf/40 disabled:opacity-60";

const SOURCE_LABEL: Record<string, { text: string; tone: "neutral" | "good" | "stub" | "warn" }> = {
  default: { text: "default", tone: "neutral" },
  platform: { text: "platform default", tone: "stub" },
  tenant: { text: "your value", tone: "good" },
  operator: { text: "fixed by platform", tone: "warn" },
};

export function Control({ item, value, onChange, disabled }: { item: SettingItem; value: unknown; onChange: (v: unknown) => void; disabled: boolean }) {
  switch (item.type) {
    case "bool":
      return (
        <span className="inline-flex items-center gap-3">
          <Switch label={item.title} checked={value === true} disabled={disabled} onChange={onChange} />
          <span className={cn("text-sm font-medium", value === true ? "text-leaf-deep" : "text-muted")}>{value === true ? "On" : "Off"}</span>
        </span>
      );
    case "int":
    case "float":
      return (
        <div className="flex items-center gap-2">
          <input type="number" aria-label={item.title} className={cn(inputCls, "max-w-40")} value={typeof value === "number" ? value : ""} min={item.min} max={item.max}
            step={item.type === "int" ? 1 : "any"} disabled={disabled} onChange={(e) => onChange(e.target.value === "" ? "" : Number(e.target.value))} />
          {item.unit && <span className="text-sm text-muted">{item.unit}</span>}
          {item.min !== undefined && item.max !== undefined && <span className="text-xs text-muted">({item.min}–{item.max})</span>}
        </div>
      );
    case "enum":
      return (
        <select aria-label={item.title} className={cn(inputCls, "max-w-60")} value={String(value ?? "")} disabled={disabled} onChange={(e) => onChange(e.target.value)}>
          {item.enum?.map((o) => <option key={o} value={o}>{o}</option>)}
        </select>
      );
    case "list":
      return item.list_enum?.length ? (
        <div className="flex flex-wrap gap-x-4 gap-y-1.5" role="group" aria-label={item.title}>
          {item.list_enum.map((o) => {
            const cur = Array.isArray(value) ? (value as string[]) : [];
            const on = cur.includes(o);
            return (
              <span key={o} className={cn("rounded-xl px-2.5 py-1.5 ring-1 transition", on ? "bg-leaf-soft ring-leaf-soft" : "bg-white ring-line")}>
                <Check_ label={o} disabled={disabled} checked={on} onChange={(v) => onChange(v ? [...cur, o] : cur.filter((x) => x !== o))}>{o}</Check_>
              </span>
            );
          })}
        </div>
      ) : (
        <textarea aria-label={item.title} rows={2} className={inputCls} disabled={disabled} placeholder="one per line (empty = no restriction)"
          value={Array.isArray(value) ? (value as string[]).join("\n") : ""}
          onChange={(e) => onChange(e.target.value.split("\n").map((x) => x.trim()).filter(Boolean))} />
      );
    default:
      return <input aria-label={item.title} className={inputCls} value={String(value ?? "")} maxLength={item.max_len} disabled={disabled} onChange={(e) => onChange(e.target.value)} />;
  }
}

export const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

export function SettingsEditor({ mode, path, historyPath, canEdit = true, scopes, onlyGroup, hideKeys }: {
  mode: EditorMode; path: string; historyPath: string; canEdit?: boolean; scopes?: SettingItem["scope"][];
  /** Show one group only, without the search and group tabs: for a card that sits next to the thing it configures. */
  onlyGroup?: string;
  /** Keys shown elsewhere with a better control: not repeated here. */
  hideKeys?: string[];
}) {
  const qc = useQueryClient();
  const key = ["settings", path];
  const q = useQuery({ queryKey: key, queryFn: () => api(path, settingsView), staleTime: 0 });
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [active, setActive] = useState<string>("all");
  const [filter, setFilter] = useState("");
  const [saved, setSaved] = useState(false);

  const save = useMutation({
    mutationFn: (values: Record<string, unknown>) => api(path, settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values }) }),
    onSuccess: (view: SettingsView) => { qc.setQueryData(key, view); setDraft({}); setSaved(true); void qc.invalidateQueries({ queryKey: ["settings-history"] }); },
  });
  const reset = useMutation({
    mutationFn: (keys: string[]) => api(`${path}/reset`, settingsView, undefined, { method: "POST", body: JSON.stringify({ keys }) }),
    onSuccess: (view: SettingsView, keys) => { qc.setQueryData(key, view); setDraft((d) => Object.fromEntries(Object.entries(d).filter(([k]) => !keys.includes(k)))); setSaved(true); void qc.invalidateQueries({ queryKey: ["settings-history"] }); },
  });

  // On an organization's console page the suspension switch is the Status panel above the editor: not repeated here.
  const items = useMemo(() => (q.data?.items ?? []).filter((i) => (!scopes || scopes.includes(i.scope)) && (!onlyGroup || i.group === onlyGroup) && !(hideKeys ?? []).includes(i.key) && !(mode === "operator-tenant" && i.group === "organization")), [q.data, scopes, mode, onlyGroup, hideKeys]);
  const groups = useMemo(() => (q.data?.groups ?? []).filter((g) => items.some((i) => i.group === g.id)), [q.data, items]);
  const dirty = Object.keys(draft).filter((k) => { const it = items.find((i) => i.key === k); return it && !same(draft[k], it.value); });
  const shown = items.filter((i) => (active === "all" || i.group === active)
    && (!filter || `${i.title} ${i.key} ${i.description}`.toLowerCase().includes(filter.toLowerCase())));
  const fieldErrors = (save.error instanceof ApiError ? save.error.fields : {}) as Record<string, string>;
  const otherError = (save.error ?? reset.error) && !Object.keys(fieldErrors).length ? (save.error ?? reset.error)!.message : null;

  if (q.isLoading) return <Skeleton className="h-64" />;
  if (q.isError) return <ErrorState />;

  const editableHere = (i: SettingItem) => canEdit && i.editable;
  const resettable = (i: SettingItem) => editableHere(i) && ((mode === "tenant" && i.source === "tenant") || (mode === "operator-tenant" && i.source === "operator")
    || (mode === "platform" && i.source === "platform"));
  const resetLabel = mode === "operator-tenant" ? "Unlock" : mode === "platform" ? "Use default" : "Use inherited";

  return (
    <div className="space-y-4">
      {mode === "operator-tenant" && <p role="note" className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900 ring-1 ring-amber-200">Values saved here are <b>fixed for this organization</b>: it can no longer change them until you unlock them. The change is written to its audit log.</p>}
      {mode === "platform" && !onlyGroup && <p role="note" className="rounded-xl bg-violet-50 p-3 text-sm text-violet-900 ring-1 ring-violet-200">Platform-wide values. For organization settings this is the <b>default every organization inherits</b>; organizations that chose their own value keep it.</p>}
      {!onlyGroup && <div className="flex flex-wrap items-center gap-2">
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-2.5 size-4 text-muted" aria-hidden />
          <input aria-label="Search settings" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="Search settings…" className={cn(inputCls, "w-64 pl-9")} />
        </div>
        <div className="flex flex-wrap gap-1" role="tablist" aria-label="Setting groups">
          {[{ id: "all", title: "All" }, ...groups].map((g) => (
            <button key={g.id} type="button" role="tab" aria-selected={active === g.id} onClick={() => setActive(g.id)}
              className={cn("rounded-lg px-3 py-1.5 text-sm font-medium transition", active === g.id ? "bg-leaf-soft text-leaf-deep" : "text-muted hover:text-ink")}>{g.title}</button>
          ))}
        </div>
      </div>}

      <div className={cn("grid gap-4", onlyGroup ? "" : "lg:grid-cols-2")}>
      {groups.filter((g) => shown.some((i) => i.group === g.id)).map((g, gi) => (
        <Panel key={g.id} index={gi} icon={GROUP_ICON[g.id] ?? Settings2} title={g.title} hint={g.description}>
          <ul className="divide-y divide-line/70">
            {shown.filter((i) => i.group === g.id).map((i) => {
              const value = i.key in draft ? draft[i.key] : i.value;
              const src = SOURCE_LABEL[i.source];
              return (
                <li key={i.key} className="grid gap-2 py-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] lg:gap-6">
                  <div>
                    <p className="flex flex-wrap items-center gap-2 text-sm font-medium">{i.title}
                      {i.locked && <Lock className="size-3.5 text-amber-600" aria-label="locked" />}
                      <Badge tone={src.tone}>{i.source === "platform" && mode === "platform" ? "platform value" : src.text}</Badge>
                      {mode !== "tenant" && <Badge>{i.scope}</Badge>}
                    </p>
                    <p className="mt-0.5 text-xs text-muted">{i.description}</p>
                    {i.note && <p className="mt-1 text-xs text-muted">{i.note}</p>}
                    <code className="mt-1 block text-[11px] text-slate-400">{i.key}</code>
                  </div>
                  <div className="space-y-1.5">
                    <Control item={i} value={value} disabled={!editableHere(i) || save.isPending} onChange={(v) => { setSaved(false); setDraft((d) => ({ ...d, [i.key]: v })); }} />
                    {!editableHere(i) && <p className="text-xs text-muted">{i.scope === "operator" && mode === "tenant" ? "Managed by the platform." : i.locked ? "Fixed by the platform for your organization." : "Read-only for your role."}</p>}
                    {fieldErrors[i.key] && <p role="alert" className="text-xs text-bad">{fieldErrors[i.key]}</p>}
                    {resettable(i) && !(i.key in draft) && (
                      <button type="button" disabled={reset.isPending} onClick={() => reset.mutate([i.key])} className="inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"><RotateCcw className="size-3" />{resetLabel}</button>
                    )}
                    {i.key in draft && !same(draft[i.key], i.value) && (
                      <button type="button" onClick={() => setDraft((d) => { const n = { ...d }; delete n[i.key]; return n; })} className="text-xs font-medium text-muted hover:underline">Undo change</button>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        </Panel>
      ))}
      </div>
      {shown.length === 0 && <p className="py-6 text-center text-sm text-muted">No settings match.</p>}

      {canEdit && (
        <div className="sticky bottom-3 z-10 flex flex-wrap items-center gap-3 rounded-2xl bg-white/90 p-3 shadow-lg ring-1 ring-line backdrop-blur">
          <button type="button" disabled={dirty.length === 0 || save.isPending}
            onClick={() => save.mutate(Object.fromEntries(dirty.map((k) => [k, draft[k]])))}
            className="rounded-lg bg-leaf-deep px-4 py-2 text-sm font-semibold text-white shadow-sm transition-all hover:bg-leaf-deep active:scale-[0.98] disabled:opacity-50">{save.isPending ? "Saving…" : mode === "operator-tenant" ? `Save and lock ${dirty.length || ""}` : `Save ${dirty.length || ""} change${dirty.length === 1 ? "" : "s"}`}</button>
          {dirty.length > 0 && <button type="button" onClick={() => setDraft({})} className="text-sm text-muted hover:underline">Discard</button>}
          {saved && dirty.length === 0 && <span role="status" className="text-sm text-leaf-deep">Saved.</span>}
          {Object.keys(fieldErrors).length > 0 && <span role="alert" className="text-sm text-bad">{Object.keys(fieldErrors).length} setting(s) were rejected; nothing was saved.</span>}
          {otherError && <span role="alert" className="text-sm text-bad">{otherError}</span>}
        </div>
      )}
    </div>
  );
}

export function SettingsHistory({ path }: { path: string }) {
  const q = useQuery({ queryKey: ["settings-history", path], queryFn: () => api(path, settingChanges), staleTime: 0 });
  const pg = usePaged(q.data ?? [], 10);
  if (q.isLoading) return <Skeleton className="h-32" />;
  if (q.isError) return <ErrorState />;
  if (q.data!.length === 0) return <Panel icon={History} title="Change history" hint="Who changed which setting"><p className="py-4 text-center text-sm text-muted">No changes yet.</p></Panel>;
  const show = (v: unknown) => (v === null || v === undefined ? "—" : Array.isArray(v) ? `[${v.join(", ")}]` : String(v));
  return (
    <Panel icon={History} title="Change history" hint="Who changed which setting, and when">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead><tr className="text-left text-xs text-muted"><th className="py-1.5 pr-3 font-medium">When</th><th className="pr-3 font-medium">Setting</th><th className="pr-3 font-medium">Change</th><th className="pr-3 font-medium">By</th></tr></thead>
          <tbody className="divide-y divide-line/70">
            {pg.slice.map((h, n) => (
              <tr key={n}>
                <td className="whitespace-nowrap py-2 pr-3 text-muted" title={h.at}>{shortTime(h.at)}</td>
                <td className="pr-3"><code className="text-xs">{h.key}</code></td>
                <td className="pr-3">{show(h.old_value)} → <b>{show(h.new_value)}</b></td>
                <td className="pr-3">{h.actor}{h.via === "operator" && <Badge tone="warn" className="ml-2">operator</Badge>}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
    </Panel>
  );
}
