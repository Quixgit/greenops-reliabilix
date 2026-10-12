"use client";

import Link from "next/link";
import { Card } from "@/components/ui/card";
import { Glyph } from "@/components/ui/glyphs";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { SOURCE_LABEL } from "@/components/charts/intensity-chart";
import { useConnectionRuns } from "@/hooks/use-connectors";
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
  // Cost against the cloud's own totals: the latest run of every connection that has been checked.
  const runs = useConnectionRuns((conns.data ?? []).map((x) => x.id));
  const checks = Object.values(runs.byId).map((r) => r.find((x) => x.quality)?.quality).filter((q): q is NonNullable<typeof q> => !!q);
  const mismatched = checks.filter((q) => q.status === "mismatch").length;
  const verified = checks.filter((q) => q.status === "ok").length;
  const basis = (b: string) => t(`ov.basis.${b}`, undefined, b);
  return (
    <Card index={6} interactive={false} className="p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-ink"><Glyph name="shield" className="size-[18px] text-[#2f8f78]" />{t("ov.dq.title")}</h2><Info corner>{t("info.quality")}</Info>
        <Link href="/methodology" className="text-xs font-semibold text-primary hover:underline">{t("ov.dq.methodology")}</Link>
      </div>
      {carbon.isLoading || conns.isLoading ? <Skeleton className="h-24" /> : !c?.has_data ? (
        <p className="text-sm text-muted">{t("ov.dq.empty")}</p>
      ) : (
        <div className="space-y-2.5 text-[13px]">
          <div>
            <div className="flex items-baseline justify-between"><span className="text-muted">{t("ov.dq.measured")}</span>
              <span className="font-semibold tabular-nums">{measured === null ? "—" : `${nf(0).format(measured)}%`}</span></div>
            <div className="mt-1.5 h-2 overflow-hidden rounded-full bg-slate-100">
              <div className="h-full rounded-full bg-gradient-to-r from-leaf/80 to-leaf transition-[width] duration-700" style={{ width: `${Math.min(100, measured ?? 0)}%` }} />
            </div>
          </div>
          {measured !== null && measured < 50 && <p className="rounded-lg bg-amber-50 px-2.5 py-2 text-xs leading-snug text-amber-900 ring-1 ring-amber-100">{t("ov2.dq.estimated")}</p>}
          <div className="flex items-center justify-between rounded-lg bg-slate-50 px-2.5 py-2 text-[13px]">
            <span className="text-slate-600">{t("ov2.dq.cost_check")}</span>
            <span className={mismatched ? "font-semibold text-bad" : verified ? "font-semibold text-leaf-deep" : "text-muted"}>
              {runs.loading ? "…" : mismatched ? t("ov2.dq.cost_mismatch", { count: mismatched }) : verified ? t("ov2.dq.cost_ok", { count: verified }) : t("ov2.dq.cost_unchecked")}
            </span>
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
