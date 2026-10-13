"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState, type CSSProperties } from "react";
import { Card } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconBell, IconCard, IconSparkles, IconCheck, IconCluster, IconFlow, IconLeaf, IconReport, IconSend, IconTrend, IconWarning, type IconType } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useDeliveries } from "@/hooks/use-admin";
import { eventArea, useInbox, useMarkInboxRead } from "@/hooks/use-inbox";
import { useAlerts, useAnomalies } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { alert as alertSchema, type Alert } from "@/lib/api/schemas";
import { formatCost, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const TABS = ["messages", "alerts", "anomalies", "delivery"] as const;
type Tab = (typeof TABS)[number];
const AREA_ICON: Record<string, IconType> = { billing: IconCard, reports: IconReport, automation: IconFlow, recommendations: IconLeaf, connections: IconCluster, platform: IconSparkles, other: IconBell };
const SEV_TEXT = { critical: "text-[#b8625c]", warning: "text-[#cba649]", info: "text-[#5d7fb0]" } as const;
const SEV_CHIP = { critical: "bg-[#f8eceb] text-[#a8504b]", warning: "bg-[#faf5e3] text-[#8a6420]", info: "bg-[#eef2f8] text-[#3f6094]" } as const;
const STATUS_CHIP: Record<string, string> = { new: "bg-[#eef2f8] text-[#3f6094]", acknowledged: "bg-slate-100 text-slate-600", muted: "bg-slate-100 text-slate-400", resolved: "bg-leaf-soft text-leaf-deep" };

function Stat({ index, icon: Icon, tone, label, info, value, hint, onClick, on }: { index: number; icon: IconType; tone: string; label: string; info: string; value: number | null; hint?: string; onClick: () => void; on: boolean }) {
  // The ⓘ is a button too and a button cannot hold a button: it sits in the card's corner, beside the clickable area.
  return (
    <Card index={index} interactive={false} className="relative p-0">
      <button type="button" onClick={onClick} aria-pressed={on} className={cn("flex w-full items-center gap-4 rounded-[inherit] p-4 text-left transition-all duration-300 hover:-translate-y-0.5", on && "bg-leaf-soft/50")}>
        <Icon className={cn("size-11 shrink-0", tone)} strokeWidth={1.4} />
        <span className="min-w-0">
          <span className="block text-[13px] font-medium text-slate-600">{label}</span>
          <span className="mt-0.5 block text-2xl font-bold tracking-tight tabular-nums">{value === null ? "–" : <CountUp value={value} format={(n) => String(Math.round(n))} />}</span>
          {hint && <span className="block truncate text-xs text-muted">{hint}</span>}
        </span>
      </button>
      <Info corner>{info}</Info>
    </Card>
  );
}

function Messages() {
  const t = useT();
  const q = useInbox();
  const read = useMarkInboxRead();
  const [area, setArea] = useState("all");
  const all = q.data?.items ?? [];
  const areas = [...new Set(all.map((i) => eventArea(i.type)))];
  const rows = all.filter((i) => area === "all" || eventArea(i.type) === area);
  const pg = usePaged(rows, 10);
  const chip = (on: boolean) => cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", on ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep");
  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <button type="button" className={chip(area === "all")} onClick={() => setArea("all")}>{t("nc.area.all")}</button>
        {areas.map((a) => <button key={a} type="button" className={chip(area === a)} onClick={() => setArea(a)}>{t(`nc.area.${a}`)}</button>)}
        <button type="button" disabled={(q.data?.unread ?? 0) === 0 || read.isPending} onClick={() => read.mutate()}
          className="ml-auto inline-flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-sm font-medium text-slate-600 ring-1 ring-line transition-colors hover:bg-slate-50 disabled:opacity-50"><IconCheck className="size-4" />{t("bell.mark_read")}</button>
      </div>
      {q.isLoading ? <Skeleton className="h-48" /> : q.isError ? <ErrorState /> : rows.length === 0 ? <EmptyState icon={IconCheck} text={t("nc.messages.empty")} /> : (
        <ul key={`${area}${pg.page}`} className="space-y-2">
          {pg.slice.map((i, n) => {
            const Icon = i.severity === "critical" ? IconWarning : AREA_ICON[eventArea(i.type)];
            return (
              <li key={i.id} className="rise" style={{ "--i": Math.min(n, 8) } as CSSProperties}>
                <Link href={i.path || "/alerts"} className={cn("group flex items-start gap-4 rounded-2xl border p-4 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-[0_10px_28px_-18px_rgba(47,143,120,0.5)]", i.unread ? "border-leaf/40 bg-leaf-soft/40" : "border-line bg-white")}>
                  <Icon className={cn("size-10 shrink-0", SEV_TEXT[i.severity])} strokeWidth={1.4} />
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap items-center gap-2"><span className="text-sm font-semibold text-ink">{i.title}</span>
                      <span className={cn("rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide", SEV_CHIP[i.severity])}>{i.severity}</span>
                      {i.unread && <span className="size-2 rounded-full bg-leaf" aria-label={t("nc.unread")} />}</span>
                    {i.body && <span className="mt-0.5 block text-sm text-slate-600">{i.body}</span>}
                    <span className="mt-1 block text-xs text-muted">{timeAgo(i.created_at)}</span>
                  </span>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
    </div>
  );
}

function AlertRow({ a, canAct, index }: { a: Alert; canAct: boolean; index: number }) {
  const t = useT();
  const qc = useQueryClient();
  const act = useMutation({
    mutationFn: (what: "acknowledge" | "mute" | "unmute") => api(`alerts/${a.id}/${what}`, alertSchema, undefined, { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["alerts"] }),
  });
  const btn = "rounded-xl px-3 py-1.5 text-xs font-semibold ring-1 ring-line transition-colors hover:bg-slate-50 disabled:opacity-60";
  return (
    <li className="rise rounded-2xl border border-line bg-white p-4" style={{ "--i": Math.min(index, 8) } as CSSProperties}>
      <div className="flex flex-wrap items-start gap-3">
        <IconWarning className={cn("size-9 shrink-0", SEV_TEXT[a.severity])} strokeWidth={1.4} />
        <div className="min-w-0 flex-1">
          <p className="flex flex-wrap items-center gap-2 text-sm font-semibold">{a.title}
            <span className={cn("rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide", SEV_CHIP[a.severity])}>{a.severity}</span>
            <span className={cn("rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide", STATUS_CHIP[a.status])}>{t(`nc.status.${a.status}`)}</span></p>
          {a.detail && <p className="mt-0.5 text-sm text-slate-600">{a.detail}</p>}
          <p className="mt-1 text-xs text-muted">{t("nc.since", { ago: timeAgo(a.first_seen_at) })}{a.resolved_at ? ` · ${t("nc.resolved", { ago: timeAgo(a.resolved_at) })}` : ""}</p>
        </div>
        {canAct && a.status !== "resolved" && (
          <div className="flex gap-2">
            {a.status === "new" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("acknowledge")}>{t("nc.ack")}</button>}
            {a.status !== "muted" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("mute")}>{t("nc.mute")}</button>}
            {a.status === "muted" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("unmute")}>{t("nc.unmute")}</button>}
          </div>
        )}
      </div>
    </li>
  );
}

function Alerts({ canAct }: { canAct: boolean }) {
  const t = useT();
  const [status, setStatus] = useState<"new" | "acknowledged" | "muted" | "resolved" | "all">("new");
  const q = useAlerts(status);
  const pg = usePaged(q.data ?? [], 10);
  const chip = (on: boolean) => cn("rounded-full px-3 py-1 text-xs font-semibold capitalize transition-all", on ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep");
  return (
    <div>
      <div role="tablist" className="mb-3 flex flex-wrap gap-1.5">
        {(["new", "acknowledged", "muted", "resolved", "all"] as const).map((f) => <button key={f} role="tab" aria-selected={f === status} type="button" className={chip(f === status)} onClick={() => setStatus(f)}>{t(`nc.status.${f}`, undefined, f)}</button>)}
      </div>
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : (q.data ?? []).length === 0 ? <EmptyState icon={IconBell} text={t("nc.alerts.empty")} /> : (
        <ul key={`${status}${pg.page}`} className="space-y-2">{pg.slice.map((a, i) => <AlertRow key={a.id} a={a} canAct={canAct} index={i} />)}</ul>
      )}
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
    </div>
  );
}

/** Messages the platform sent, alerts that need a decision, cost anomalies and deliveries that failed: one place, one count each. */
export function AlertsView({ canAct = false }: { canAct?: boolean }) {
  const t = useT();
  const [tab, setTab] = useState<Tab>("messages");
  const inbox = useInbox();
  const alerts = useAlerts("new");
  const anomalies = useAnomalies();
  const deliveries = useDeliveries();
  const failed = (deliveries.data ?? []).filter((d) => d.status === "failed");
  useEffect(() => {
    const h = window.location.hash.slice(1);
    if ((TABS as readonly string[]).includes(h)) setTab(h as Tab);
  }, []);
  const go = (x: Tab) => { setTab(x); window.history.replaceState(null, "", `#${x}`); };
  return (
    <div className="page-enter space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{t("nc.title")}</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted">{t("nc.subtitle")}</p>
        </div>
        <Link href="/settings#notifications" className="rounded-xl px-4 py-2.5 text-sm font-semibold text-leaf-deep ring-1 ring-line transition-colors hover:bg-leaf-soft">{t("nc.configure")}</Link>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat index={0} icon={IconSend} tone="text-[#5d7fb0]" label={t("nc.stat.messages")} info={t("nc.info.messages")} value={inbox.isLoading ? null : inbox.data?.unread ?? 0} hint={t("nc.stat.unread")} on={tab === "messages"} onClick={() => go("messages")} />
        <Stat index={1} icon={IconWarning} tone="text-[#b8625c]" label={t("nc.stat.alerts")} info={t("nc.info.alerts")} value={alerts.isLoading ? null : alerts.data?.length ?? 0} hint={t("nc.stat.need_decision")} on={tab === "alerts"} onClick={() => go("alerts")} />
        <Stat index={2} icon={IconTrend} tone="text-[#cba649]" label={t("nc.stat.anomalies")} info={t("nc.info.anomalies")} value={anomalies.isLoading ? null : anomalies.data?.length ?? 0} hint={t("nc.stat.period")} on={tab === "anomalies"} onClick={() => go("anomalies")} />
        <Stat index={3} icon={IconCluster} tone="text-[#5a6aa0]" label={t("nc.stat.delivery")} info={t("nc.info.delivery")} value={deliveries.isLoading ? null : failed.length} hint={t("nc.stat.failed")} on={tab === "delivery"} onClick={() => go("delivery")} />
      </div>

      <Card interactive={false}>
        <div key={tab} className="form-swap">
          {tab === "messages" && <Messages />}
          {tab === "alerts" && <Alerts canAct={canAct} />}
          {tab === "anomalies" && (
            anomalies.isLoading ? <Skeleton className="h-24" /> : anomalies.isError ? <ErrorState /> : (anomalies.data ?? []).length === 0 ? <EmptyState icon={IconTrend} text={t("nc.anomalies.empty")} /> : (
              <ul className="divide-y divide-line/70 text-sm">
                {anomalies.data!.map((a) => (
                  <li key={a.day} className="flex flex-wrap items-center justify-between gap-2 py-3">
                    <span className="font-medium">{new Date(a.day).toLocaleDateString()}</span>
                    <span className="text-slate-600"><b className="text-ink">{formatCost(a.cost)}</b> {t("nc.anomaly.vs", { expected: formatCost(a.expected) })}</span>
                  </li>
                ))}
              </ul>
            )
          )}
          {tab === "delivery" && (
            failed.length === 0 ? <EmptyState icon={IconCheck} text={t("nc.delivery.empty")} /> : (
              <ul className="divide-y divide-line/70 text-sm">
                {failed.map((d) => (
                  <li key={d.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                    <span><span className="font-medium">{d.event_type}</span> <span className="text-muted">→ {d.destination_name}</span></span>
                    <span className="text-xs text-muted">{d.last_error ?? t("nc.delivery.failed")} · {timeAgo(d.created_at)}</span>
                  </li>
                ))}
              </ul>
            )
          )}
        </div>
      </Card>
    </div>
  );
}
