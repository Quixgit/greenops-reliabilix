"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Cable } from "lucide-react";
import type { AnyIcon as LucideIcon } from "@/components/ui/icons";
import { IconBell as BellRing, IconChart as BarChart3, IconKey as KeyRound } from "@/components/ui/icons";
import Link from "next/link";
import { useMemo, useState } from "react";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/empty-state";
import { useConnections } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { ApiError, api } from "@/lib/api/client";
import { settingsView, type SettingItem, type SettingsView } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { DataPanel } from "./data-panel";
import { Panel } from "./organization-tab";
import { Control, same } from "./settings-editor";

type Card = { id: string; icon: LucideIcon; keys: string[] };
const CARDS: Card[] = [
  { id: "sources", icon: Cable, keys: ["sync.min_interval_hours", "connectors.allowed_providers"] },
  { id: "reporting", icon: BarChart3, keys: ["finops.reporting_currency", "reports.allowed_formats"] },
  { id: "alerts", icon: BellRing, keys: ["alerts.warn_pct", "alerts.carbon_monthly_kg"] },
  { id: "access", icon: KeyRound, keys: ["security.invitation_expiry_days", "security.invitation_allowed_domains", "security.api_keys_max_active"] },
];

/** One setting as a compact row: the name and a one-line purpose on the left, the control on the right. */
function Field({ item, value, error, onChange, onUndo, dirty, disabled }: {
  item: SettingItem; value: unknown; error?: string; onChange: (v: unknown) => void; onUndo: () => void; dirty: boolean; disabled: boolean;
}) {
  const t = useT();
  const stacked = item.type === "list";
  return (
    <div className="border-t border-line/70 py-3 first:border-t-0 first:pt-0">
      <div className={cn("flex gap-3", stacked ? "flex-col" : "items-center justify-between")}>
        <div className="min-w-0">
          <p className="text-[13px] font-medium text-ink">{item.title}</p>
          <p className="mt-0.5 line-clamp-2 text-xs leading-snug text-muted" title={item.description}>{item.description}</p>
        </div>
        <div className={cn("shrink-0", !stacked && item.type !== "bool" && "w-32")}>
          <Control item={{ ...item, unit: undefined, min: undefined, max: undefined }} value={value} disabled={disabled} onChange={onChange} />
        </div>
      </div>
      {error && <p role="alert" className="mt-1 text-xs text-bad">{error}</p>}
      {dirty && <button type="button" onClick={onUndo} className="mt-1 text-xs font-medium text-muted hover:underline">{t("settings.data.undo")}</button>}
    </div>
  );
}

/**
 * Data & integrations: where data comes from, how it is reported, when to be warned, who may be invited, and what
 * the plan allows. Every card is the same size; changes are saved together from one bar.
 */
export function DataTab({ canEdit, isOwner }: { canEdit: boolean; isOwner: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["settings", "settings"];
  const q = useQuery({ queryKey: key, queryFn: () => api("settings", settingsView) });
  const conns = useConnections();
  const [draft, setDraft] = useState<Record<string, unknown>>({});
  const [saved, setSaved] = useState(false);
  const save = useMutation({
    mutationFn: (values: Record<string, unknown>) => api("settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values }) }),
    onSuccess: (view: SettingsView) => { qc.setQueryData(key, view); setDraft({}); setSaved(true); void qc.invalidateQueries({ queryKey: ["settings-history"] }); },
  });
  const items = useMemo(() => new Map((q.data?.items ?? []).map((i) => [i.key, i])), [q.data]);
  const dirty = Object.keys(draft).filter((k) => items.has(k) && !same(draft[k], items.get(k)!.value));
  const errors = (save.error instanceof ApiError ? save.error.fields : {}) as Record<string, string>;

  if (q.isLoading) return <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{Array.from({ length: 6 }, (_, i) => <Skeleton key={i} className="h-56" />)}</div>;
  if (q.isError) return <ErrorState />;

  const suspended = items.get("organization.suspended")?.value === true;

  const card = (c: Card, index: number) => {
    const rows = c.keys.map((k) => items.get(k)).filter((i): i is SettingItem => !!i);
    return (
      <Panel key={c.id} index={index} icon={c.icon} title={t(`settings.data.${c.id}`)} hint={t(`settings.data.${c.id}_hint`)} className="h-full">
        {c.id === "sources" && (
          <div className="mb-3 flex items-center justify-between rounded-lg bg-slate-50 px-3 py-2 text-[13px]">
            <span className="text-slate-600">{t("settings.data.connections")}</span>
            <Link href="/connectors" className="font-semibold text-teal-700 hover:underline">{conns.data?.length ?? 0} →</Link>
          </div>
        )}
        {rows.map((i) => (
          <Field key={i.key} item={i} value={i.key in draft ? draft[i.key] : i.value} error={errors[i.key]} dirty={i.key in draft && !same(draft[i.key], i.value)}
            disabled={!canEdit || !i.editable || save.isPending}
            onChange={(v) => { setSaved(false); setDraft((d) => ({ ...d, [i.key]: v })); }}
            onUndo={() => setDraft((d) => { const n = { ...d }; delete n[i.key]; return n; })} />
        ))}
      </Panel>
    );
  };

  return (
    <div className="space-y-4">
      {suspended && (
        <p role="status" className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900 ring-1 ring-amber-200">{t("settings.data.suspended")}</p>
      )}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3 [&>section]:h-full">
        {CARDS.map((c, i) => card(c, i))}
        {isOwner && <DataPanel />}
      </div>

      {canEdit && dirty.length > 0 && (
        <div className="rise sticky bottom-3 z-10 flex flex-wrap items-center gap-3 rounded-xl bg-white/95 p-3 shadow-lg ring-1 ring-line backdrop-blur">
          <button type="button" disabled={save.isPending} onClick={() => save.mutate(Object.fromEntries(dirty.map((k) => [k, draft[k]])))}
            className="rounded-lg bg-teal-700 px-4 py-2 text-sm font-semibold text-white shadow-sm transition-all hover:bg-teal-800 active:scale-[0.98] disabled:opacity-50">
            {save.isPending ? t("settings.org.saving") : t("settings.data.save", { count: dirty.length })}
          </button>
          <button type="button" onClick={() => setDraft({})} className="text-sm text-muted hover:underline">{t("settings.data.discard")}</button>
          {Object.keys(errors).length > 0 && <span role="alert" className="text-sm text-bad">{t("settings.data.rejected")}</span>}
        </div>
      )}
      {saved && dirty.length === 0 && <p role="status" className="text-sm text-teal-700">{t("settings.org.saved")}</p>}
      {save.isError && Object.keys(errors).length === 0 && <p role="alert" className="text-sm text-bad">{save.error.message}</p>}
    </div>
  );
}

