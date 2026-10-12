"use client";

import Link from "next/link";
import { useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconWallet } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useShowback, useTagKeys } from "@/hooks/use-showback";
import { useT } from "@/i18n/provider";
import { formatCost, nf } from "@/lib/formatters";
import { cn } from "@/lib/utils";

/** Who spends what: the bill split by the values of a cloud tag (team, environment ...), with how much of the bill the tag explains. */
export function ShowbackPanel() {
  const t = useT();
  const keys = useTagKeys();
  const [picked, setTag] = useState<string | null>(null);
  const items = keys.data?.items ?? [];
  const tag = picked && items.some((k) => k.key === picked) ? picked : items[0]?.key ?? null; // the first key until one is picked, or when the picked one has no cost in this period
  const sb = useShowback(tag);
  const d = sb.data;
  const cur = d?.currency || keys.data?.currency || undefined;
  const max = Math.max(...(d?.lines ?? []).map((l) => l.cost), 1e-9);
  return (
    <div className="space-y-4">
      <Card index={0} interactive={false}>
        <CardHeader icon={<IconWallet className="size-[18px] text-sky-500" />} title={t("sb.title")} info={t("sb.info")} />
        <p className="max-w-3xl text-sm text-muted">{t("sb.lead")}</p>
        {items.length > 0 && (
          <div role="tablist" className="mt-3 flex flex-wrap gap-2">
            {items.map((k) => (
              <button key={k.key} type="button" role="tab" aria-selected={tag === k.key} onClick={() => setTag(k.key)}
                className={cn("rounded-full px-3 py-1 text-sm ring-1 ring-inset transition-all duration-200", tag === k.key ? "bg-leaf-deep text-white ring-leaf-deep" : "bg-white text-slate-700 ring-line hover:bg-leaf-soft hover:text-leaf-deep")}
                title={t("sb.key_hint", { values: k.values, pct: k.coverage_pct.toFixed(0) })}>
                {k.key} <span className="opacity-70">· {k.coverage_pct.toFixed(0)}%</span>
              </button>
            ))}
          </div>
        )}
      </Card>
      {keys.isLoading ? <Skeleton className="h-40" /> : keys.isError ? <ErrorState /> : items.length === 0 ? (
        <Card interactive={false}>
          <EmptyState icon={IconWallet} text={t("sb.empty")} />
          <p className="-mt-2 mb-3 text-center text-xs text-muted">{t("sb.empty_hint")} <Link href="/settings" className="font-semibold text-primary hover:underline">{t("sb.empty_link")} →</Link></p>
        </Card>
      ) : sb.isLoading ? <Skeleton className="h-60" /> : sb.isError || !d ? <ErrorState /> : (
        <>
          <div className="grid gap-4 sm:grid-cols-3">
            <Card index={1} interactive={false}><p className="text-[13px] font-medium text-slate-600">{t("sb.total")}</p><p className="mt-1 text-2xl font-bold tabular-nums">{formatCost(d.total_cost, cur)}</p></Card>
            <Card index={2} interactive={false}><p className="text-[13px] font-medium text-slate-600">{t("sb.tagged")}</p><p className="mt-1 text-2xl font-bold tabular-nums">{formatCost(d.tagged_cost, cur)}</p><p className="text-xs text-muted">{t("sb.coverage", { pct: d.coverage_pct.toFixed(0), key: d.tag })}</p></Card>
            <Card index={3} interactive={false}><p className="text-[13px] font-medium text-slate-600">{t("sb.untagged")}</p><p className={cn("mt-1 text-2xl font-bold tabular-nums", d.coverage_pct < 50 && "text-[#b7791f]")}>{formatCost(d.untagged_cost, cur)}</p><p className="text-xs text-muted">{t("sb.untagged_hint")}</p></Card>
          </div>
          <Card index={4} interactive={false} className="space-y-1">
            {d.lines.length === 0 ? <p className="py-6 text-center text-sm text-muted">{t("sb.no_values", { key: d.tag })}</p> : d.lines.map((l, i) => (
              <div key={l.value} className="row-in grid items-center gap-x-4 gap-y-1 rounded-xl px-2 py-2.5 hover:bg-slate-50 sm:grid-cols-[minmax(0,12rem)_minmax(0,1fr)_7rem]" style={{ "--i": i } as CSSProperties}>
                <span className="truncate text-sm font-medium" title={l.value}>{l.value}</span>
                <span className="min-w-0">
                  <span className="block h-2 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full bg-[#7aa8e6]" style={{ width: `${Math.max(1, (Math.max(l.cost, 0) / max) * 100)}%` }} /></span>
                  <span className="mt-1 block truncate text-[11px] text-muted">{l.top_services.map((s) => `${s.service} ${formatCost(s.cost, cur)}`).join(" · ")}</span>
                </span>
                <span className="text-right text-sm tabular-nums"><span className="font-semibold">{formatCost(l.cost, cur)}</span><span className="block text-[11px] text-muted">{nf(1).format(l.share_pct)}%</span></span>
              </div>
            ))}
          </Card>
          {d.tags_from && <p className="rounded-xl bg-slate-50 px-3 py-2 text-xs text-slate-600">{t("sb.tags_from", { date: d.tags_from })}</p>}
          <p className="text-[11px] leading-snug text-muted">{t("sb.basis")}</p>
        </>
      )}
    </div>
  );
}
