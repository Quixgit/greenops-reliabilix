"use client";

import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnections } from "@/hooks/use-overview";
import { cn } from "@/lib/utils";

const CLOUDS = [
  { key: "aws", label: "AWS" }, { key: "azure", label: "Azure" }, { key: "gcp", label: "GCP" }, { key: "kubernetes", label: "Kubernetes" },
];

/** Always lists all four targets; status reflects reality (Phase 1: at most one cloud, Kubernetes arrives later). */
export function ConnectedAccounts() {
  const q = useConnections();
  const connected = new Set((q.data ?? []).map((c) => c.provider));
  return (
    <Card>
      <CardHeader title="Connected Accounts" />
      {q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : (
        <ul className="space-y-3">
          {CLOUDS.map((c) => {
            const on = connected.has(c.key);
            return (
              <li key={c.key} className="flex items-center justify-between gap-3 text-sm">
                <span className="flex items-center gap-2.5"><span className={cn("size-2.5 rounded-full", on ? "bg-carbon" : "bg-slate-300")} />{c.label}</span>
                {on ? <span className="text-xs font-medium text-carbon">Connected</span> : (
                  <span className="flex items-center gap-3"><span className="text-xs text-muted">Not connected</span>
                    <Link href="/cloud" className="rounded-md px-2 py-0.5 text-xs font-medium text-primary ring-1 ring-primary/30 hover:bg-primary/5">Connect</Link></span>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
