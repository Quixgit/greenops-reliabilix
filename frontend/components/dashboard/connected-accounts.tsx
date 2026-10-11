"use client";

import { Box, Cloud, Hexagon, type LucideIcon } from "lucide-react";
import Link from "next/link";
import type { CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnections, useK8sClusters } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { connectorSummary } from "@/lib/connectors";
import { cn } from "@/lib/utils";

// The state of every tile comes from lib/connectors (the same derivation as the Connectors page), so the two screens
// cannot disagree: a cloud from its connections, Kubernetes from the clusters that push data.
const CLOUDS: { key: string; label: string; icon: LucideIcon; tile: string }[] = [
  { key: "aws", label: "AWS", icon: Cloud, tile: "bg-orange-100 text-orange-600" },
  { key: "azure", label: "Azure", icon: Hexagon, tile: "bg-blue-100 text-blue-600" },
  { key: "gcp", label: "GCP", icon: Cloud, tile: "bg-emerald-100 text-emerald-600" },
  { key: "kubernetes", label: "Kubernetes", icon: Box, tile: "bg-sky-100 text-sky-600" },
];

export function ConnectedAccounts() {
  const t = useT();
  const q = useConnections();
  const clusters = useK8sClusters();
  const facts = { connections: q.data ?? [], clusters: clusters.data ?? [], destinations: [] };
  const stateOf = (provider: string) => { const s = connectorSummary(provider, facts).state; return s === "available" ? null : s; };
  return (
    <Card index={9} interactive={false}>
      <CardHeader title="Connected Accounts" action={<Link href="/connectors" className="text-xs font-semibold text-primary hover:underline">View all →</Link>} />
      {q.isLoading ? <Skeleton className="h-16" /> : q.isError ? <ErrorState /> : (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {CLOUDS.map((c, i) => {
            const st = stateOf(c.key);
            return (
              <li key={c.key} className="rise" style={{ "--i": i + 2 } as CSSProperties}>
                <Link href={c.key === "kubernetes" ? "/kubernetes" : "/connectors"} className="group flex flex-col items-center gap-1.5 rounded-xl p-2 text-center transition-colors hover:bg-slate-50">
                  <span className={cn("flex size-11 items-center justify-center rounded-full transition-transform duration-300 group-hover:scale-110", c.tile)}><c.icon className="size-5" strokeWidth={1.8} /></span>
                  <span className="text-sm font-medium">{c.label}</span>
                  {st === "attention" ? <span className="flex items-center gap-1 text-[11px] font-medium text-bad"><span className="size-1.5 rounded-full bg-bad" />{t("connection.state.attention")}</span>
                    : st === "failing" ? <span className="flex items-center gap-1 text-[11px] font-medium text-bad"><span className="size-1.5 rounded-full bg-bad" />{t("connection.state.failing")}</span>
                    : st === "waiting" ? <span className="flex items-center gap-1 text-[11px] font-medium text-amber-600"><span className="size-1.5 animate-pulse rounded-full bg-amber-500" />{t("connection.state.waiting")}</span>
                    : st === "synced" ? <span className="flex items-center gap-1 text-[11px] font-medium text-emerald-600"><span className="size-1.5 animate-pulse rounded-full bg-emerald-500" />{t("connection.state.synced")}</span>
                    : <span className="whitespace-nowrap text-[11px] text-muted">{t("connection.state.none")}</span>}
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
