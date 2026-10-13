"use client";

import { cloudMeta } from "@/lib/clouds";
import { useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconBolt, IconCoin, IconDownload, IconGauge, IconLeaf, IconRefresh, IconReport, IconTrash, IconWarning } from "@/components/ui/icons";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useProjects, useReports } from "@/hooks/use-overview";
import { useDeleteReport, useRequestReport } from "@/hooks/use-reports";
import { useT } from "@/i18n/provider";
import type { Report } from "@/lib/api/schemas";
import { timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const KIND_ICON = { carbon: [IconLeaf, "text-leaf-deep"], finops: [IconCoin, "text-[#5d7fb0]"], sci: [IconGauge, "text-[#5a6aa0]"], sustainability: [IconBolt, "text-[#cba649]"], ghg: [IconLeaf, "text-[#3f9d7a]"] } as const;
const STATUS = [["", "rp.f.all"], ["ready", "rp.f.ready"], ["pending", "rp.f.pending"], ["failed", "rp.f.failed"]] as const;

function Row({ r, i, canAct, projectName }: { r: Report; i: number; canAct: boolean; projectName: string }) {
  const t = useT();
  const del = useDeleteReport();
  const again = useRequestReport();
  const [ask, setAsk] = useState(false);
  const [Icon, tone] = KIND_ICON[r.kind];
  const day = (s: string) => s.slice(0, 10);
  return (
    <li className="rise relative" style={{ "--i": Math.min(i, 8) } as CSSProperties}>
      <div className="group flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-3 py-3 transition-colors hover:bg-slate-50">
        <Icon className={cn("size-10 shrink-0", tone, r.status === "pending" && "animate-pulse")} strokeWidth={1.4} />
        <div className="min-w-0 flex-1 basis-56">
          <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-ink">
            {t(`rp.kind.${r.kind}`)}
            <span className="rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide text-slate-500">{r.format}</span>
          </p>
          <p className="mt-0.5 truncate text-xs text-muted">{projectName}{r.clouds.length > 0 && <> ({r.clouds.map((c) => cloudMeta(c).name).join(", ")})</>} · {day(r.period_start)} → {day(r.period_end)} · {timeAgo(r.created_at)}</p>
        </div>
        <div className="flex items-center gap-2">
          {r.status === "pending" && (
            <span className="flex items-center gap-2 text-xs font-medium text-slate-500">
              <span className="relative h-1.5 w-20 overflow-hidden rounded-full bg-slate-100"><span className="absolute inset-y-0 w-1/2 report-bar rounded-full bg-leaf" /></span>
              {t("rp.rendering")}
            </span>
          )}
          {r.status === "failed" && <span className="flex items-center gap-1.5 rounded-full bg-[#f8eceb] px-2.5 py-1 text-xs font-semibold text-[#a8504b]"><IconWarning className="size-3.5" />{t("rp.failed")}</span>}
          {r.status === "ready" && (
            <a href={`/api/proxy/reports/${r.id}/download`} download className="btn-leaf inline-flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold"><IconDownload className="size-3.5" strokeWidth={2.2} />{t("rp.download")}</a>
          )}
          {r.status === "failed" && canAct && (
            <button type="button" disabled={again.isPending} onClick={() => again.mutate({ kind: r.kind, format: r.format, project_id: r.project_id, period_start: day(r.period_start), period_end: day(r.period_end), clouds: r.clouds })}
              className="inline-flex items-center gap-1.5 rounded-lg bg-slate-100 px-3 py-1.5 text-xs font-semibold text-slate-700 transition-colors hover:bg-leaf-soft hover:text-leaf-deep"><IconRefresh className="size-3.5" />{t("rp.retry")}</button>
          )}
          {canAct && r.status !== "pending" && (
            <button type="button" aria-label={t("rp.delete")} title={t("rp.delete")} onClick={() => setAsk(true)}
              className="grid size-8 place-items-center rounded-full text-slate-400 opacity-0 transition-all hover:bg-[#f8eceb] hover:text-[#b8625c] focus-visible:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"><IconTrash className="size-4" strokeWidth={1.8} /></button>
          )}
        </div>
        {r.status === "failed" && r.error && <p className="basis-full pl-14 text-xs text-[#a8504b]">{r.error}</p>}
      </div>
      {ask && (
        <div className="del-wipe absolute inset-0 z-10 flex items-center justify-between gap-3 overflow-hidden rounded-2xl bg-[#f8eceb] px-4 ring-1 ring-[#d3a09b]">
          <p className="flex min-w-0 items-center gap-2 text-sm font-semibold text-ink"><IconTrash className="size-5 shrink-0 text-[#b8625c]" strokeWidth={1.6} /><span className="truncate">{t("rp.delete_ask")}</span></p>
          <div className="flex shrink-0 gap-2">
            <button type="button" onClick={() => setAsk(false)} className="rounded-lg bg-white px-3 py-1.5 text-xs font-medium text-slate-600 ring-1 ring-line hover:text-ink">{t("common.cancel")}</button>
            <button type="button" disabled={del.isPending} onClick={() => del.mutate(r.id)} className="rounded-lg bg-[#d3a09b] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#b8625c] disabled:opacity-60">{t("rp.delete_do")}</button>
          </div>
        </div>
      )}
    </li>
  );
}

/** The organization's reports: filter by what and how it went, download, retry a failed one, delete one you no longer need. */
export function ReportList({ canAct }: { canAct: boolean }) {
  const t = useT();
  const q = useReports();
  const projects = useProjects().data ?? [];
  const [kind, setKind] = useState("");
  const [status, setStatus] = useState("");
  const all = q.data ?? [];
  const items = all.filter((r) => (!kind || r.kind === kind) && (!status || r.status === status))
    .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at));
  const pg = usePaged(items, 10);
  const chip = (on: boolean) => cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", on ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep");
  return (
    <Card index={2} interactive={false}>
      <CardHeader icon={<IconReport className="size-[18px] text-leaf-deep" />} title={t("rp.list")} info={t("rp.info.list")} />
      <div className="mb-3 flex flex-wrap items-center gap-x-5 gap-y-2">
        <div className="flex flex-wrap gap-1.5" role="group" aria-label={t("rp.step.kind")}>
          <button type="button" className={chip(kind === "")} onClick={() => setKind("")}>{t("rp.f.all_kinds")}</button>
          {(["carbon", "finops", "sci", "sustainability", "ghg"] as const).map((k) => <button key={k} type="button" className={chip(kind === k)} onClick={() => setKind(k)}>{t(`rp.kind.${k}`)}</button>)}
        </div>
        <div className="flex flex-wrap gap-1.5" role="group" aria-label={t("rp.f.status")}>
          {STATUS.map(([v, k]) => <button key={v} type="button" className={chip(status === v)} onClick={() => setStatus(v)}>{t(k)}</button>)}
        </div>
      </div>
      {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : all.length === 0 ? (
        <EmptyState icon={IconReport} text={t("rp.empty")} />
      ) : items.length === 0 ? <p className="py-8 text-center text-sm text-muted">{t("rp.none_match")}</p> : (
        <ul key={`${kind}${status}${pg.page}`} className="-mx-1 space-y-0.5">
          {pg.slice.map((r, i) => <Row key={r.id} r={r} i={i} canAct={canAct} projectName={projects.find((p) => p.id === r.project_id)?.name ?? t("rp.all_projects")} />)}
        </ul>
      )}
      <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
    </Card>
  );
}
