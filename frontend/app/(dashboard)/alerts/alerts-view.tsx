"use client";

import { useT } from "@/i18n/provider";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconBell as Bell } from "@/components/ui/icons";
import { useState } from "react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { useAlerts, useAnomalies } from "@/hooks/use-overview";
import { useDeliveries } from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import type { Alert } from "@/lib/api/schemas";
import { alert as alertSchema } from "@/lib/api/schemas";
import { formatCost, timeAgo } from "@/lib/formatters";

const FILTERS = ["new", "acknowledged", "muted", "resolved", "all"] as const;
const SEV = { critical: "bad", warning: "warn", info: "neutral" } as const;

function AlertRow({ a, canAct }: { a: Alert; canAct: boolean }) {
  const qc = useQueryClient();
  const act = useMutation({
    mutationFn: (what: "acknowledge" | "mute" | "unmute") => api(`alerts/${a.id}/${what}`, alertSchema, undefined, { method: "POST", body: "{}" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["alerts"] }),
  });
  const btn = "rounded-lg px-2.5 py-1 text-xs font-medium ring-1 ring-line hover:bg-slate-50 disabled:opacity-60";
  return (
    <li className="space-y-1.5 py-3 text-sm">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="font-medium">{a.title}</p>
        <span className="flex items-center gap-1.5"><Badge tone={SEV[a.severity]}>{a.severity}</Badge><Badge tone="neutral">{a.status}</Badge></span>
      </div>
      {a.detail && <p className="text-xs text-muted">{a.detail}</p>}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs text-muted">since {timeAgo(a.first_seen_at)}{a.resolved_at ? ` · resolved ${timeAgo(a.resolved_at)}` : ""}</span>
        {canAct && a.status !== "resolved" && (
          <span className="flex gap-1.5">
            {a.status === "new" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("acknowledge")}>Acknowledge</button>}
            {a.status !== "muted" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("mute")}>Mute</button>}
            {a.status === "muted" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("unmute")}>Unmute</button>}
          </span>
        )}
      </div>
    </li>
  );
}

/** Everything that needs attention, derived from data the platform already holds: no invented alerts. */
export function AlertsView({ canAct = false }: { canAct?: boolean }) {
  const t = useT();
  const [status, setStatus] = useState<(typeof FILTERS)[number]>("new");
  const stored = useAlerts(status);
  const pg = usePaged(stored.data ?? [], 10);
  const anomalies = useAnomalies();
  const deliveries = useDeliveries();
  const failed = (deliveries.data ?? []).filter((d) => d.status === "failed");
  const loading = anomalies.isLoading;
  const empty = !loading && !anomalies.data?.length && !failed.length;
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Alerts</h1>
      <Card index={0}>
        <CardHeader info={t("info.alerts_list")} title="Alerts" />
        <div role="tablist" className="mb-2 flex flex-wrap gap-2">
          {FILTERS.map((f) => (
            <button key={f} role="tab" aria-selected={f === status} onClick={() => { setStatus(f); }}
              className={`rounded-full px-3 py-1 text-xs font-medium capitalize ring-1 ring-line ${f === status ? "bg-primary text-white" : "bg-white"}`}>{f}</button>
          ))}
        </div>
        {stored.isLoading ? <Skeleton className="h-16" /> : stored.isError ? <ErrorState /> : stored.data!.length === 0 ?
          <EmptyState icon={Bell} text="No alerts here. Budgets and connections are checked every hour." /> :
          <ul className="divide-y divide-line/70">{pg.slice.map((a) => <AlertRow key={a.id} a={a} canAct={canAct} />)}</ul>}
        <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
      </Card>
      {loading ? <Skeleton className="h-24" /> : anomalies.isError ? <ErrorState /> : empty && (
        <Card><EmptyState icon={Bell} text="No cost anomalies and no undelivered notifications." /></Card>
      )}
      {!!anomalies.data?.length && (
        <Card index={1}>
          <CardHeader info={t("info.alerts_anomalies")} title="Cost anomalies" />
          <ul className="divide-y divide-line/70 text-sm">
            {anomalies.data.map((a) => (
              <li key={a.day} className="flex items-center justify-between py-2.5">
                <span>{new Date(a.day).toLocaleDateString("en-GB")}</span>
                <span className="text-muted">{formatCost(a.cost)} vs expected {formatCost(a.expected)}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
      {failed.length > 0 && (
        <Card index={2}>
          <CardHeader info={t("info.alerts_undelivered")} title="Undelivered notifications" action={<Link href="/settings" className="text-sm font-medium text-primary">Delivery settings</Link>} />
          <ul className="divide-y divide-line/70 text-sm">
            {failed.map((d) => (
              <li key={d.id} className="flex items-center justify-between py-2.5">
                <span>{d.event_type} <span className="text-muted">→ {d.destination_name}</span></span>
                <span className="text-xs text-muted">{d.last_error ?? "failed"} · {timeAgo(d.created_at)}</span>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}
