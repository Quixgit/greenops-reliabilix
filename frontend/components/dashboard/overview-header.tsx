"use client";

import Link from "next/link";
import { usePlanState } from "@/components/layout/plan-card";
import { useConnections, useProjects } from "@/hooks/use-overview";
import { content } from "@/content/schema";
import { useT } from "@/i18n/provider";
import { timeAgo } from "@/lib/formatters";
import { periodLabel, useUi } from "@/lib/store";
import { cn } from "@/lib/utils";
import { cloudMeta } from "@/lib/clouds";
import { useCloudScope } from "@/hooks/use-overview";
import { CloudSwitcher } from "@/components/dashboard/cloud-switcher";

/** Title and the scope of every figure on the page: which projects, which period, how fresh. */
export function OverviewHeader() {
  const t = useT();
  const { projectId, period, custom } = useUi();
  const projects = useProjects().data ?? [];
  const conns = useConnections().data ?? [];
  const label = periodLabel(period, custom);
  const plan = usePlanState();
  const name = projectId ? projects.find((p) => p.id === projectId)?.name ?? t("ov.head.selected") : t("ov.head.all");
  const scope = useCloudScope();
  const last = conns.map((c) => c.last_sync_at).filter((x): x is string => !!x).sort().pop();
  return (
    <div className="rise flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
      <div className="min-w-0">
        <h1 className="text-xl font-bold tracking-tight text-ink sm:text-2xl">{content.pageTitle}</h1>
        <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-[13px] text-slate-600">
          <span>{name}</span>{scope && <><span aria-hidden>·</span><span className="font-semibold" style={{ color: cloudMeta(scope).ink }}>{cloudMeta(scope).name}</span></>}<span aria-hidden>·</span><span>{label}</span><span aria-hidden>·</span>
          <span>{last ? t("ov.head.synced", { when: timeAgo(last) }) : t("ov.head.no_sync")}</span>
          <span className="rounded-full bg-amber-50 px-2 py-px text-[10px] font-semibold text-amber-700 ring-1 ring-amber-200">Alpha</span>
          {plan && (
            <Link href="/settings#plan" title={plan.action}
              className={cn("inline-flex items-center gap-1.5 rounded-full px-2 py-px text-[10px] font-semibold ring-1 transition hover:shadow-sm", plan.attention || !plan.paid ? "bg-leaf-soft text-leaf-deep ring-leaf/30 hover:bg-leaf-soft/70" : "bg-slate-50 text-slate-600 ring-line hover:bg-slate-100")}>
              {t("ov.head.plan", { plan: plan.p.name })}<span className="font-medium opacity-70">· {plan.action}</span>
            </Link>
          )}
        </p>
      </div>
      <CloudSwitcher />
    </div>
  );
}
