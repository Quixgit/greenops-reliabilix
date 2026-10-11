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
        <ul className="-mx-2 divide-y divide-line/60">
          {CLOUDS.map((c, i) => {
            const s = connectorSummary(c.key, facts);
            const on = s.state !== "available";
            const href = c.key === "kubernetes" ? "/kubernetes" : on ? "/connectors" : `/connectors?connect=${c.key}`;
            return (
              <li key={c.key} className="rise" style={{ "--i": i } as CSSProperties}>
                <Link href={href} className="group flex items-center gap-3.5 rounded-xl px-2 py-3 transition-colors duration-200 hover:bg-slate-50">
                  <c.icon className={cn("size-10 shrink-0 transition-all duration-300 group-hover:scale-110", !on && "opacity-55 saturate-50 group-hover:opacity-100 group-hover:saturate-100")} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-semibold text-ink">{c.label}</span>
                    <span className="mt-0.5 flex items-center gap-1.5 text-xs text-muted">
                      <span className={cn("size-1.5 shrink-0 rounded-full", on ? DOT[s.state] : "bg-slate-300", s.state === "synced" && "animate-pulse")} />
                      <span className="truncate">{on ? t(`connection.state.${s.state}`) : t("connection.state.none")}{on && s.lastAt ? ` · ${timeAgo(s.lastAt)}` : ""}</span>
                    </span>
                  </span>
                  <span className={cn("shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold transition-colors", on ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-500 group-hover:bg-leaf-soft group-hover:text-leaf-deep")}>
                    {on ? t("ov2.accounts.open") : t("ov2.accounts.connect")}
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
