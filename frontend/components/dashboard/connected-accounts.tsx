"use client";

import Link from "next/link";
import type { CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { IconAws, IconAzure, IconGcp, IconKubernetes, type IconType } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnections, useK8sClusters } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { connectorSummary } from "@/lib/connectors";
import { timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const CLOUDS: { key: string; label: string; icon: IconType }[] = [
  { key: "aws", label: "AWS", icon: IconAws },
  { key: "azure", label: "Azure", icon: IconAzure },
  { key: "gcp", label: "Google Cloud", icon: IconGcp },
  { key: "kubernetes", label: "Kubernetes", icon: IconKubernetes },
];

const DOT: Record<string, string> = { synced: "bg-leaf", waiting: "bg-pastel-amber", failing: "bg-pastel-rose", attention: "bg-pastel-rose" };

/**
 * The clouds and clusters feeding the platform, one drawn mark each (no tile, no frame). The state comes from the same
 * derivation as the Connectors page; a platform that is not connected leads straight to its connect panel.
 */
export function ConnectedAccounts() {
  const t = useT();
  const q = useConnections();
  const clusters = useK8sClusters();
  const facts = { connections: q.data ?? [], clusters: clusters.data ?? [], destinations: [] };
  return (
    <Card index={9} interactive={false}>
      <CardHeader title="Connected Accounts" info={t("info.accounts")} action={<Link href="/connectors" className="text-xs font-semibold text-leaf-deep hover:underline">View all →</Link>} />
      {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : (
        <ul className="grid grid-cols-2 gap-3">
          {CLOUDS.map((c, i) => {
            const s = connectorSummary(c.key, facts);
            const on = s.state !== "available";
            return (
              <li key={c.key} className="rise" style={{ "--i": i } as CSSProperties}>
                <Link href={c.key === "kubernetes" ? "/kubernetes" : on ? "/connectors" : `/connectors?connect=${c.key}`}
                  className={cn("group flex h-full flex-col gap-3 rounded-2xl border p-3.5 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-md",
                    on ? "border-leaf/40 bg-gradient-to-br from-leaf-soft/70 to-white" : "border-line bg-white hover:border-slate-300")}>
                  <c.icon className={cn("size-11 transition-transform duration-500 group-hover:-rotate-6 group-hover:scale-110", on ? "text-leaf-deep" : "text-slate-400")} strokeWidth={1.4} />
                  <span>
                    <span className="block text-sm font-semibold text-ink">{c.label}</span>
                    <span className="mt-0.5 flex items-center gap-1.5 text-[11px] text-muted">
                      <span className={cn("size-1.5 rounded-full", on ? DOT[s.state] : "bg-slate-300", s.state === "synced" && "animate-pulse")} />
                      {on ? t(`connection.state.${s.state}`) : t("connection.state.none")}
                    </span>
                    {on && s.lastAt && <span className="mt-0.5 block text-[11px] text-slate-400">{timeAgo(s.lastAt)}</span>}
                  </span>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
