"use client";

import Link from "next/link";
import type { CSSProperties } from "react";
import { Card } from "@/components/ui/card";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
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
    <Card index={9} interactive={false} className="p-4">
      <div className="mb-2 flex items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink"><Glyph name="cloud" className="size-[18px] text-[#3b93b8]" />{t("ov3.accounts.title")}</h2><Info corner>{t("info.accounts")}</Info>
        <Link href="/connectors" className="text-xs font-semibold text-leaf-deep hover:underline">{t("ov3.recs.all")}</Link>
      </div>
      {q.isLoading ? <Skeleton className="h-28" /> : q.isError ? <ErrorState /> : (
        <ul className="grid grid-cols-2 gap-2">
          {CLOUDS.map((c, i) => {
            const s = connectorSummary(c.key, facts);
            const on = s.state !== "available";
            const href = c.key === "kubernetes" ? "/kubernetes" : on ? "/connectors" : `/connectors?connect=${c.key}`;
            return (
              <li key={c.key} className="rise" style={{ "--i": i } as CSSProperties}>
                <Link href={href} className={cn("group flex items-center gap-2.5 rounded-xl p-2 ring-1 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-sm",
                  on ? "bg-white ring-line hover:ring-slate-300" : "border border-dashed border-slate-200 bg-slate-50/70 ring-0")}>
                  <c.icon className={cn("size-8 shrink-0 transition-all duration-300 group-hover:scale-110", !on && "opacity-50 saturate-50 group-hover:opacity-100 group-hover:saturate-100")} />
                  <span className="min-w-0 flex-1 leading-tight">
                    <span className="block truncate text-[13px] font-semibold text-ink">{c.label}</span>
                    <span className="flex items-center gap-1 text-[11px] text-muted">
                      <span className={cn("size-1.5 shrink-0 rounded-full", on ? DOT[s.state] : "bg-slate-300", s.state === "synced" && "animate-pulse")} />
                      <span className="truncate">{on ? (s.lastAt ? timeAgo(s.lastAt) : t(`connection.state.${s.state}`)) : t("ov3.accounts.connect")}</span>
                    </span>
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
