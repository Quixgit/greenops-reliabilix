"use client";

import { useConnections, useProjects } from "@/hooks/use-overview";
import { content } from "@/content/schema";
import { useT } from "@/i18n/provider";
import { timeAgo } from "@/lib/formatters";
import { periodLabel, useUi } from "@/lib/store";

/** Title and the scope of every figure on the page: which projects, which period, how fresh. */
export function OverviewHeader() {
  const t = useT();
  const { projectId, period, custom } = useUi();
  const projects = useProjects().data ?? [];
  const conns = useConnections().data ?? [];
  const label = periodLabel(period, custom);
  const name = projectId ? projects.find((p) => p.id === projectId)?.name ?? t("ov.head.selected") : t("ov.head.all");
  const last = conns.map((c) => c.last_sync_at).filter((x): x is string => !!x).sort().pop();
  return (
    <div className="rise">
      <h1 className="text-[28px] font-bold tracking-tight text-ink">{content.pageTitle}</h1>
      <p className="mt-1 flex flex-wrap items-center gap-x-2 text-[15px] text-slate-600">
        <span>{name}</span><span aria-hidden>·</span><span>{label}</span><span aria-hidden>·</span>
        <span>{last ? t("ov.head.synced", { when: timeAgo(last) }) : t("ov.head.no_sync")}</span>
        <span className="rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-semibold text-amber-700 ring-1 ring-amber-200">Alpha</span>
      </p>
    </div>
  );
}
