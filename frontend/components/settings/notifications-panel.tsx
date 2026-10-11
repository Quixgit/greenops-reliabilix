"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconHistory as HistoryIcon, IconPower as Power, IconBell as Bell, IconPlus as Plus, IconSend as Send } from "@/components/ui/icons";
import { Switch } from "@/components/operator/ui";
import { useNotificationsEnabled } from "@/hooks/use-admin";
import { settingsView } from "@/lib/api/schemas";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Field, input } from "@/components/forms/project-form";
import { Badge, statusTone } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { SecretOnce } from "@/components/ui/secret-once";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useDeliveries, useDestinations, useNotificationCatalog } from "@/hooks/use-admin";
import { api, apiVoid } from "@/lib/api/client";
import { createdDestination, destination, testQueued, type NotificationCatalog } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { useT } from "@/i18n/provider";
import { Panel } from "./organization-tab";
import { BellSettings } from "./bell-settings";

const PLACEHOLDER: Record<string, string> = {
  email: "someone@yourcompany.com (must be a member of this organization)",
  webhook: "https://example.com/hooks/reliabilix (public https)",
  slack: "https://hooks.slack.com/services/…",
};

const schema = z.object({
  kind: z.string().min(1, "Choose a channel"),
  name: z.string().trim().min(1, "Name is required").max(100, "Max 100 characters"),
  target: z.string().trim().min(1, "Address is required").max(2048),
  event_types: z.array(z.string()).default([]),
  min_severity: z.enum(["info", "warning", "critical"]),
});
type Values = z.input<typeof schema>;

function AddDestination({ catalog }: { catalog: NotificationCatalog }) {
  const qc = useQueryClient();
  const [secret, setSecret] = useState<string | null>(null);
  const { register, handleSubmit, reset, watch, formState: { errors } } = useForm<Values>({
    resolver: zodResolver(schema), defaultValues: { kind: catalog.channels[0] ?? "", min_severity: "info", event_types: [] },
  });
  const kind = watch("kind");
  const m = useMutation({
    mutationFn: (v: Values) => api("notifications/destinations", createdDestination, undefined, {
      method: "POST", body: JSON.stringify({ ...v, event_types: v.event_types?.length ? v.event_types : ["*"] }),
    }),
    onSuccess: (r) => {
      reset({ kind: catalog.channels[0] ?? "", name: "", target: "", event_types: [], min_severity: "info" });
      setSecret(r.signing_secret ?? null);
      void qc.invalidateQueries({ queryKey: ["notif-destinations"] });
    },
  });
  return (
    <div className="space-y-3">
      {secret && <SecretOnce title="Webhook signing secret" value={secret} onDismiss={() => setSecret(null)} />}
      <form onSubmit={handleSubmit((v) => m.mutate(v))} className="grid gap-3" noValidate>
        <div><Field label="Channel" error={errors.kind?.message}>
          <select {...register("kind")} className={input}>{catalog.channels.map((c) => <option key={c} value={c}>{c}{catalog.stubbed.includes(c) ? " (stub)" : ""}</option>)}</select>
        </Field></div>
        <div><Field label="Name" error={errors.name?.message}><input {...register("name")} className={input} placeholder="e.g. ops-alerts" /></Field></div>
        <div><Field label="Address" error={errors.target?.message}><input {...register("target")} className={input} placeholder={PLACEHOLDER[kind] ?? ""} autoComplete="off" /></Field></div>
        <fieldset>
          <legend className="mb-1 text-sm font-medium text-slate-600">Events <span className="font-normal text-muted">(none selected = all)</span></legend>
          <div className="grid grid-cols-1 gap-y-1.5 sm:grid-cols-2">
            {catalog.event_types.map((t) => (
              <label key={t.type} className="flex items-center gap-1.5 text-sm" title={t.description}>
                <input type="checkbox" value={t.type} {...register("event_types")} className="size-4 accent-leaf-deep" />{t.type}
              </label>
            ))}
          </div>
        </fieldset>
        <div><Field label="Minimum severity">
          <select {...register("min_severity")} className={input}><option value="info">info</option><option value="warning">warning</option><option value="critical">critical</option></select>
        </Field></div>
        <button disabled={m.isPending} className="rounded-lg btn-leaf px-4 py-2.5 text-sm font-semibold text-white active:scale-[0.98] disabled:opacity-60">{m.isPending ? "Adding…" : "Add"}</button>
        {m.isError && <p role="alert" className="text-sm text-bad">{m.error.message}</p>}
      </form>
    </div>
  );
}

function DestinationList({ canEdit, catalog }: { canEdit: boolean; catalog: NotificationCatalog }) {
  const qc = useQueryClient();
  const q = useDestinations();
  const refresh = () => { void qc.invalidateQueries({ queryKey: ["notif-destinations"] }); void qc.invalidateQueries({ queryKey: ["notif-deliveries"] }); };
  const toggle = useMutation({
    mutationFn: (v: { id: string; enable: boolean }) => api(`notifications/destinations/${v.id}/${v.enable ? "enable" : "disable"}`, destination, undefined, { method: "POST" }),
    onSuccess: refresh,
  });
  const test = useMutation({ mutationFn: (id: string) => api(`notifications/destinations/${id}/test`, testQueued, undefined, { method: "POST" }), onSuccess: refresh });
  const del = useMutation({ mutationFn: (id: string) => apiVoid(`notifications/destinations/${id}`, { method: "DELETE" }), onSuccess: refresh });
  const err = toggle.error ?? test.error ?? del.error;
  if (q.isLoading) return <Skeleton className="h-24" />;
  if (q.isError) return <ErrorState />;
  if (q.data!.length === 0) return <EmptyState icon={Bell} text="No destinations yet. Add one to get notified about finished reports and cloud connections that need attention." />;
  return (
    <>
      <ul className="divide-y divide-line/70 text-sm">
        {q.data!.map((d) => (
          <li key={d.id} className="flex flex-wrap items-center justify-between gap-3 border-t border-line/70 py-3 transition-colors hover:bg-slate-50/70">
            <div className="min-w-0">
              <p className="flex flex-wrap items-center gap-2 font-medium">{d.name}
                <Badge>{d.kind}</Badge>{catalog.stubbed.includes(d.kind) && <Badge tone="stub">stub</Badge>}{!d.enabled && <Badge tone="warn">paused</Badge>}</p>
              <p className="mt-0.5 truncate text-xs text-muted">{d.target_hint} · {d.event_types.includes("*") ? "all events" : d.event_types.join(", ")} · from {d.min_severity}</p>
            </div>
            {canEdit && (
              <div className="flex items-center gap-2">
                <button type="button" disabled={test.isPending} onClick={() => test.mutate(d.id)} className="inline-flex items-center gap-1 rounded-lg px-2.5 py-1 text-xs font-medium text-primary ring-1 ring-line hover:bg-white disabled:opacity-60"><Send className="size-3" />Test</button>
                <button type="button" disabled={toggle.isPending} onClick={() => toggle.mutate({ id: d.id, enable: !d.enabled })} className="rounded-lg px-2.5 py-1 text-xs font-medium ring-1 ring-line hover:bg-white disabled:opacity-60">{d.enabled ? "Pause" : "Resume"}</button>
                <ConfirmButton label="Delete" disabled={del.isPending} onConfirm={() => del.mutate(d.id)} />
              </div>
            )}
          </li>
        ))}
      </ul>
      {err && <p role="alert" className="mt-2 text-sm text-bad">{err.message}</p>}
    </>
  );
}

function DeliveryHistory() {
  const q = useDeliveries();
  const pg = usePaged(q.data ?? [], 10);
  if (q.isLoading) return <Skeleton className="h-24" />;
  if (q.isError) return <ErrorState />;
  if (q.data!.length === 0) return <EmptyState icon={Send} text="Nothing has been delivered yet. Use “Test” on a destination to send a first message." />;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="text-xs text-muted"><tr><th className="py-1.5 pr-3 font-medium">When</th><th className="pr-3 font-medium">Destination</th><th className="pr-3 font-medium">Event</th><th className="pr-3 font-medium">Status</th><th className="font-medium">Detail</th></tr></thead>
        <tbody className="divide-y divide-line/70">
          {pg.slice.map((d) => (
            <tr key={d.id}>
              <td className="whitespace-nowrap py-2 pr-3 text-muted" title={d.created_at}>{shortTime(d.created_at)}</td>
              <td className="pr-3">{d.destination_name} <span className="text-xs text-muted">{d.kind}</span></td>
              <td className="pr-3">{d.event_type}</td>
              <td className="pr-3"><Badge tone={statusTone(d.status)}>{d.status}</Badge></td>
              <td className="text-xs text-muted">{d.status === "pending" && d.attempts > 0 ? `retry ${shortTime(d.next_attempt_at)} · ` : ""}{d.attempts > 1 || d.status !== "sent" ? `attempt ${d.attempts}` : ""}{d.last_error ? ` · ${d.last_error}` : ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
    </div>
  );
}

/** The one organization-wide switch: when off, nothing is delivered by any destination (each keeps its own pause). */
function MasterSwitch({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const enabled = useNotificationsEnabled();
  const dests = useDestinations();
  const toggle = useMutation({
    mutationFn: (v: boolean) => api("settings", settingsView, undefined, { method: "PATCH", body: JSON.stringify({ values: { "notifications.enabled": v } }) }),
    onSuccess: (view) => qc.setQueryData(["settings", "settings"], view),
  });
  const list = dests.data ?? [];
  const on = list.filter((d) => d.enabled).length;
  return (
    <Panel info={t("info.set_notif_master")} index={0} icon={Power} title={t("settings.notif.master")} hint={t("settings.notif.master_hint")}>
      <div className="flex items-center justify-between gap-3 border-t border-line/70 pt-3">
        <span className="text-[13px] text-slate-700">{enabled === false ? t("settings.notif.master_off") : t("settings.notif.master_on")}</span>
        <Switch label={t("settings.notif.master")} checked={enabled !== false} disabled={!canEdit || enabled === undefined || toggle.isPending} onChange={(v) => toggle.mutate(v)} />
      </div>
      <p className="mt-2 text-xs text-muted">{t("settings.notif.master_count", { on, total: list.length })}</p>
      {toggle.isError && <p role="alert" className="mt-2 text-xs text-bad">{toggle.error.message}</p>}
    </Panel>
  );
}

export function NotificationsPanel({ canEdit }: { canEdit: boolean }) {
  const t = useT();
  const cat = useNotificationCatalog();
  return (
    <div className="space-y-5">
      <BellSettings canEdit={canEdit} />
      {cat.data && !cat.data.configured && (
        <p role="status" className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900 ring-1 ring-amber-200">
          Notifications are not set up on this deployment: the platform has no encryption key for destination addresses, so none can be added yet. Ask the platform operator to set <code>NOTIFICATIONS_SECRET_KEY</code>.
        </p>
      )}
      {cat.data && cat.data.stubbed.length > 0 && (
        <p role="status" className="rounded-xl bg-violet-50 p-3 text-sm text-violet-900 ring-1 ring-violet-200">
          Running on a development stand-in: <b>{cat.data.stubbed.join(", ")}</b>. Messages on {cat.data.stubbed.length > 1 ? "these channels are" : "this channel is"} only written to the server log, nothing is actually sent.
        </p>
      )}
      <div className="grid gap-4 lg:grid-cols-3">
        <Panel info={t("info.set_notif_dest")} index={0} icon={Bell} title={t("settings.notif.destinations")} hint={t("settings.notif.destinations_hint")} className="lg:col-span-2">
          {cat.isLoading ? <Skeleton className="h-24" /> : cat.isError ? <ErrorState /> : <DestinationList canEdit={canEdit && cat.data!.configured} catalog={cat.data!} />}
        </Panel>
        <div className="flex flex-col gap-4 [&>section]:flex-1">
          <MasterSwitch canEdit={canEdit} />
          {canEdit && cat.data?.configured && (
            <Panel info={t("info.set_notif_add")} index={1} icon={Plus} title={t("settings.notif.add")} hint={t("settings.notif.add_hint")}><AddDestination catalog={cat.data} /></Panel>
          )}
        </div>
      </div>
      <Panel info={t("info.set_notif_hist")} index={2} icon={HistoryIcon} title={t("settings.notif.deliveries")} hint={t("settings.notif.deliveries_hint")}><DeliveryHistory /></Panel>
    </div>
  );
}
