"use client";

import { Box, Cloud, Hexagon, type LucideIcon } from "lucide-react";
import Link from "next/link";
import type { CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnections } from "@/hooks/use-overview";
import { cn } from "@/lib/utils";

// "soon" providers are stubs: the connector does not exist yet, so they are never shown as connectable or connected.
const CLOUDS: { key: string; label: string; icon: LucideIcon; tile: string; soon?: boolean }[] = [
  { key: "aws", label: "AWS", icon: Cloud, tile: "bg-orange-100 text-orange-600" },
  { key: "azure", label: "Azure", icon: Hexagon, tile: "bg-blue-100 text-blue-600", soon: true },
  { key: "gcp", label: "GCP", icon: Cloud, tile: "bg-emerald-100 text-emerald-600" },
  { key: "kubernetes", label: "Kubernetes", icon: Box, tile: "bg-sky-100 text-sky-600", soon: true },
];

export function ConnectedAccounts() {
  const q = useConnections();
  const connected = new Set((q.data ?? []).filter((c) => c.sync_status !== "error").map((c) => c.provider));
  const broken = new Set((q.data ?? []).filter((c) => c.sync_status === "error").map((c) => c.provider));
  return (
    <Card index={9} interactive={false}>
      <CardHeader title="Connected Accounts" action={<Link href="/cloud" className="text-xs font-semibold text-primary hover:underline">View all →</Link>} />
      {q.isLoading ? <Skeleton className="h-16" /> : q.isError ? <ErrorState /> : (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {CLOUDS.map((c, i) => {
            const on = connected.has(c.key), bad = broken.has(c.key);
            return (
              <li key={c.key} className="rise" style={{ "--i": i + 2 } as CSSProperties}>
                <Link href="/cloud" className="group flex flex-col items-center gap-1.5 rounded-xl p-2 text-center transition-colors hover:bg-slate-50">
                  <span className={cn("flex size-11 items-center justify-center rounded-full transition-transform duration-300 group-hover:scale-110", c.tile)}><c.icon className="size-5" strokeWidth={1.8} /></span>
                  <span className="text-sm font-medium">{c.label}</span>
                  {c.soon ? <span className="whitespace-nowrap rounded-full bg-violet-50 px-2 py-0.5 text-[10px] font-medium text-violet-600">Soon</span>
                    : bad ? <span className="flex items-center gap-1 text-[11px] font-medium text-bad"><span className="size-1.5 rounded-full bg-bad" />Needs attention</span>
                    : on ? <span className="flex items-center gap-1 text-[11px] font-medium text-emerald-600"><span className="size-1.5 animate-pulse rounded-full bg-emerald-500" />Connected</span>
                    : <span className="whitespace-nowrap text-[11px] text-muted">Not connected</span>}
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
