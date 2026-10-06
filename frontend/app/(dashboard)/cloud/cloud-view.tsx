"use client";

import { Cloud } from "lucide-react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ConnectionForm } from "@/components/forms/connection-form";
import { useConnections } from "@/hooks/use-overview";
import { timeAgo } from "@/lib/formatters";

export function CloudView({ canConnect }: { canConnect: boolean }) {
  const q = useConnections();
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">Cloud Accounts</h1>
      {canConnect && <Card><CardHeader title="Connect an AWS account" /><ConnectionForm /></Card>}
      <Card>
        <CardHeader title="Connections" />
        {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ?
          <EmptyState icon={Cloud} text="No cloud accounts connected yet." /> : (
            <ul className="divide-y divide-line/70 text-sm">
              {q.data!.map((c) => (
                <li key={c.id} className="flex items-center justify-between py-2.5">
                  <span><b className="uppercase">{c.provider}</b> <span className="text-muted">{c.account_ref}</span></span>
                  <span className="text-xs text-muted">{c.sync_status}{c.last_sync_at ? ` · synced ${timeAgo(c.last_sync_at)}` : ""}</span>
                </li>
              ))}
            </ul>
          )}
      </Card>
    </div>
  );
}
