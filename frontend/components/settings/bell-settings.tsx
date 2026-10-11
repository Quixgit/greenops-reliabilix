"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Switch } from "@/components/operator/ui";
import { IconBell, IconCard, IconCheck } from "@/components/ui/icons";
import { useNotificationCatalog } from "@/hooks/use-admin";
import { eventArea } from "@/hooks/use-inbox";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { settingsView } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { Panel } from "./organization-tab";

const SEVERITIES = ["info", "warning", "critical"] as const;
const REMINDER = [0, 3, 7, 14, 30];
const HIDDEN = new Set(["alert.opened", "notification.test"]); // alerts have their own list; the test message is not an event

/** What the bell shows and how: on or off, from which severity, which kinds of event, sound, and the billing reminders. */
export function BellSettings({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const settings = useQuery({ queryKey: ["settings", "settings"], queryFn: () => api("settings", settingsView) });
  const cat = useNotificationCatalog();
  const val = (k: string) => settings.data?.items.find((i) => i.key === k)?.value;
  const patch = useMutation({
    mutationFn: (values: Record<string, unknown>) => api("settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values }) }),
    onSuccess: (view) => { qc.setQueryData(["settings", "settings"], view); void qc.invalidateQueries({ queryKey: ["inbox"] }); },
  });
  const set = (key: string, value: unknown) => patch.mutate({ [key]: value });

  const enabled = val("notifications.inbox_enabled") !== false;
  const min = String(val("notifications.inbox_min_severity") ?? "info");
  const muted = new Set(((val("notifications.inbox_muted_types") as string[] | undefined) ?? []));
  const sound = val("notifications.sound") === true;
  const days = Number(val("billing.reminder_days") ?? 7);
  const copy = val("billing.email_copy") !== false;
  const types = (cat.data?.event_types ?? []).filter((e) => !HIDDEN.has(e.type));
  const disabled = !canEdit || patch.isPending || settings.isLoading;
  const toggleType = (type: string) => {
    const next = new Set(muted);
    if (next.has(type)) next.delete(type); else next.add(type);
    set("notifications.inbox_muted_types", [...next].sort());
  };

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Panel index={0} icon={IconBell} title={t("bs.bell")} hint={t("bs.bell_hint")} info={t("bs.info.bell")}>
        <div className="space-y-5 border-t border-line/70 pt-4">
          <div className="flex items-center justify-between gap-3">
            <span><span className="block text-sm font-medium">{t("bs.show")}</span><span className="block text-xs text-muted">{t("bs.show_hint")}</span></span>
            <Switch label={t("bs.show")} checked={enabled} disabled={disabled} onChange={(v) => set("notifications.inbox_enabled", v)} />
          </div>
          <div className={cn("transition-opacity", !enabled && "pointer-events-none opacity-50")}>
            <p className="mb-1.5 text-sm font-medium">{t("bs.severity")}</p>
            <div className="flex gap-1 rounded-xl bg-slate-100 p-1 text-sm" role="radiogroup" aria-label={t("bs.severity")}>
              {SEVERITIES.map((s) => (
                <button key={s} type="button" role="radio" aria-checked={min === s} disabled={disabled} onClick={() => set("notifications.inbox_min_severity", s)}
                  className={cn("flex-1 rounded-lg px-3 py-1.5 font-medium transition-all disabled:opacity-60", min === s ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{t(`bs.sev.${s}`)}</button>
              ))}
            </div>
            <p className="mt-1.5 text-xs text-muted">{t(`bs.sev.${min}_hint`)}</p>
          </div>
          <div className="flex items-center justify-between gap-3">
            <span><span className="block text-sm font-medium">{t("bs.sound")}</span><span className="block text-xs text-muted">{t("bs.sound_hint")}</span></span>
            <Switch label={t("bs.sound")} checked={sound} disabled={disabled} onChange={(v) => set("notifications.sound", v)} />
          </div>
        </div>
        {patch.isError && <p role="alert" className="mt-3 text-xs text-bad">{patch.error.message}</p>}
        {!canEdit && <p className="mt-3 text-xs text-muted">{t("bs.admins_only")}</p>}
      </Panel>

      <Panel index={1} icon={IconCard} title={t("bs.billing")} hint={t("bs.billing_hint")} info={t("bs.info.billing")}>
        <div className="space-y-5 border-t border-line/70 pt-4">
          <div>
            <p className="mb-1.5 text-sm font-medium">{t("bs.remind")}</p>
            <div className="flex flex-wrap gap-1.5" role="radiogroup" aria-label={t("bs.remind")}>
              {REMINDER.map((d) => (
                <button key={d} type="button" role="radio" aria-checked={days === d} disabled={disabled} onClick={() => set("billing.reminder_days", d)}
                  className={cn("rounded-full px-3 py-1 text-xs font-semibold transition-all disabled:opacity-60", days === d ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep")}>{d === 0 ? t("bs.remind_off") : t("bs.days", { count: d })}</button>
              ))}
            </div>
            <p className="mt-1.5 text-xs text-muted">{days === 0 ? t("bs.remind_off_hint") : t("bs.remind_hint", { days })}</p>
          </div>
          <div className="flex items-center justify-between gap-3">
            <span><span className="block text-sm font-medium">{t("bs.copy")}</span><span className="block text-xs text-muted">{t("bs.copy_hint")}</span></span>
            <Switch label={t("bs.copy")} checked={copy} disabled={disabled} onChange={(v) => set("billing.email_copy", v)} />
          </div>
        </div>
      </Panel>

      <Panel index={2} icon={IconCheck} title={t("bs.types")} hint={t("bs.types_hint")} info={t("bs.info.types")} className="lg:col-span-2">
        <ul className="grid gap-x-8 sm:grid-cols-2">
          {types.map((e) => {
            const always = eventArea(e.type) === "billing";
            const on = always || !muted.has(e.type);
            return (
              <li key={e.type} className="flex items-center justify-between gap-3 border-t border-line/70 py-3">
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">{t(`bs.type.${e.type}`, undefined, e.type)}</span>
                  <span className="block text-xs text-muted">{e.description}{always && ` · ${t("bs.always")}`}</span>
                </span>
                <Switch label={e.type} checked={on} disabled={disabled || always || !enabled} onChange={() => toggleType(e.type)} />
              </li>
            );
          })}
        </ul>
      </Panel>
    </div>
  );
}
