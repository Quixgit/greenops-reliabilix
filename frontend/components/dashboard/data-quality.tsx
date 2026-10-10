"use client";

import { ShieldCheck } from "lucide-react";
import Link from "next/link";
import { Card, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { SOURCE_LABEL } from "@/components/charts/intensity-chart";
import { useCarbonSummary, useConnections } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { nf, timeAgo } from "@/lib/formatters";

/** How far the figures can be trusted: where the grid intensity came from, how much of it is measured, how fresh the sync is. */
export function DataQuality() {
  const t = useT();
  const carbon = useCarbonSummary();
  const conns = useConnections();
  const c = carbon.data;
  const synced = (conns.data ?? []).map((x) => x.last_sync_at).filter((x): x is string => !!x).sort();
  const failing = (conns.data ?? []).filter((x) => x.sync_status === "error").length;
  const measured = c?.intensity_measured_share_pct ?? null;
  const basis = (b: string) => t(`ov.basis.${b}`, undefined, b);
  return (
    <Card index={6} interactive={false}>
      <CardHeader title={t("ov.dq.title")} icon={<ShieldCheck className="size-4 text-emerald-600" />}
        action={<Link href="/methodology" className="text-xs font-semibold text-primary hover:underline">{t("ov.dq.methodology")}</Link>} />
      {carbon.isLoading || conns.isLoading ? <Skeleton className="h-24" /> : !c?.has_data ? (
        <p className="text-sm text-muted">{t("ov.dq.empty")}</p>
      ) : (
        <div className="space-y-3 text-sm">
          <div>
            <div className="flex items-baseline justify-between"><span className="text-muted">{t("ov.dq.measured")}</span>
              <span className="font-semibold tabular-nums">{measured === null ? "—" : `${nf(0).format(measured)}%`}</span></div>
            <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-slate-100">
              <div className="h-full rounded-full bg-gradient-to-r from-emerald-300 to-emerald-400 transition-[width] duration-700" style={{ width: `${Math.min(100, measured ?? 0)}%` }} />
            </div>
          </div>
          <ul className="space-y-1.5">
            {c.intensity_origins.slice(0, 4).map((o) => (
              <li key={`${o.source}-${o.basis}`} className="flex items-center justify-between gap-3">
                <span className="min-w-0 truncate">{SOURCE_LABEL[o.source] ?? o.source} <span className="text-xs text-muted">· {basis(o.basis)}</span></span>
                <span className="tabular-nums text-muted">{nf(0).format(o.share_pct)}%</span>
              </li>
            ))}
          </ul>
          <p className="border-t border-line/70 pt-2 text-xs text-muted">
            {synced.length ? t("ov.dq.last_sync", { when: timeAgo(synced[synced.length - 1]) }) : t("ov.dq.no_sync")}
            {failing > 0 && <span className="ml-2 font-medium text-bad">· {t("ov.dq.attention", { count: failing })}</span>}
          </p>
        </div>
      )}
    </Card>
  );
}
