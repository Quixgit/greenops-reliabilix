"use client";

import { Bell } from "lucide-react";
import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useAnomalies, useConnections } from "@/hooks/use-overview";
import { useDeliveries } from "@/hooks/use-admin";
import { formatCost, timeAgo } from "@/lib/formatters";

/** Everything that needs attention, derived from data the platform already holds: no invented alerts. */
export function AlertsView() {
  const anomalies = useAnomalies();
  const connections = useConnections();
  const deliveries = useDeliveries();
  const badConnections = (connections.data ?? []).filter((c) => c.sync_status === "error");
  const failed = (deliveries.data ?? []).filter((d) => d.status === "failed");
  const loading = anomalies.isLoading || connections.isLoading;
  const empty = !loading && !anomalies.data?.length && !badConnections.length && !failed.length;
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Alerts</h1>
      {loading ? <Skeleton className="h-24" /> : anomalies.isError || connections.isError ? <ErrorState /> : empty && (
        <Card><EmptyState icon={Bell} text="Nothing needs attention: no cost anomalies, failing connections or undelivered notifications." /></Card>
      )}
      {badConnections.length > 0 && (
        <Card index={0}>
          <CardHeader title="Integration health" action={<Link href="/connectors" className="text-sm font-medium text-primary">Open connectors</Link>} />
          <ul className="divide-y divide-line/70 text-sm">
            {badConnections.map((c) => (
              <li key={c.id} className="flex items-center justify-between py-2.5">
                <span><b className="uppercase">{c.provider}</b> <span className="text-muted">{c.account_ref}</span></span>
                <Badge tone="bad">needs attention</Badge>
              </li>
            ))}
          </ul>
        </Card>
      )}
      {!!anomalies.data?.length && (
        <Card index={1}>
          <CardHeader title="Cost anomalies" />
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
          <CardHeader title="Undelivered notifications" action={<Link href="/settings" className="text-sm font-medium text-primary">Delivery settings</Link>} />
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
