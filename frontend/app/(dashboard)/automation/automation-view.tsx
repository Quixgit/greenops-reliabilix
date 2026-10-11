"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconFlow as Workflow } from "@/components/ui/icons";
import Link from "next/link";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useAutomationPlans } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { automationJob, type AutomationJob } from "@/lib/api/schemas";
import { timeAgo } from "@/lib/formatters";

const FILTERS = ["active", "finished", "all"] as const;
type Filter = (typeof FILTERS)[number];
const ACTIVE = new Set(["planned", "approved"]);
const TONE: Record<string, "good" | "warn" | "bad" | "neutral"> = { completed: "good", planned: "neutral", approved: "warn", failed: "bad", rolled_back: "bad", cancelled: "neutral" };
const RISK_TONE: Record<string, "good" | "warn" | "bad"> = { low: "good", medium: "warn", high: "bad" };
const btn = "rounded-lg px-3 py-1.5 text-xs font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-60";

function Block({ title, children }: { title: string; children: React.ReactNode }) {
  return <div><p className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</p>{children}</div>;
}

function ResultForm({ job, onDone }: { job: AutomationJob; onDone: () => void }) {
  const t = useT();
  const [outcome, setOutcome] = useState<"completed" | "failed" | "rolled_back">("completed");
  const [note, setNote] = useState("");
  const needsNote = outcome !== "completed";
  const send = useMutation({
    mutationFn: () => api(`automation/jobs/${job.id}/result`, automationJob, undefined, { method: "POST", body: JSON.stringify({ outcome, note: note.trim() || undefined }) }),
    onSuccess: onDone,
  });
  return (
    <form onSubmit={(e) => { e.preventDefault(); send.mutate(); }} className="space-y-2 rounded-xl bg-slate-50 p-3 ring-1 ring-line">
      <p className="text-sm font-medium">{t("automation.report_title")}</p>
      <select aria-label={t("automation.outcome")} value={outcome} onChange={(e) => setOutcome(e.target.value as typeof outcome)} className="rounded-lg bg-white px-2.5 py-1.5 text-sm ring-1 ring-line">
        <option value="completed">{t("automation.outcome.completed")}</option>
        <option value="failed">{t("automation.outcome.failed")}</option>
        <option value="rolled_back">{t("automation.outcome.rolled_back")}</option>
      </select>
      <textarea value={note} onChange={(e) => setNote(e.target.value)} maxLength={2000} rows={3} required={needsNote}
        placeholder={needsNote ? t("automation.note_required") : t("automation.note_optional")} className="block w-full rounded-lg bg-white px-3 py-2 text-sm ring-1 ring-line" />
      <button disabled={send.isPending || (needsNote && note.trim() === "")} className={btn}>{t("automation.report_send")}</button>
      {send.isError && <p role="alert" className="text-xs text-bad">{send.error.message}</p>}
    </form>
  );
}

function JobCard({ job, canAct }: { job: AutomationJob; canAct: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const [open, setOpen] = useState(job.status === "planned" || job.status === "approved");
  const [reporting, setReporting] = useState(false);
  const refresh = () => { setReporting(false); void qc.invalidateQueries({ queryKey: ["automation-jobs"] }); void qc.invalidateQueries({ queryKey: ["automation-jobs-lite"] }); void qc.invalidateQueries({ queryKey: ["recommendations"] }); };
  const act = useMutation({
    mutationFn: (verb: "approve" | "cancel") => api(`automation/jobs/${job.id}/${verb}`, automationJob, undefined, { method: "POST", body: "{}" }),
    onSuccess: refresh,
  });
  return (
    <li className="py-3 text-sm">
      <button onClick={() => setOpen(!open)} aria-expanded={open} className="flex w-full flex-wrap items-start justify-between gap-2 text-left">
        <span>
          <span className="block font-medium">{job.plan.summary}</span>
          <span className="block text-muted">{t(`automation.kind.${job.kind}`, undefined, job.kind)} · {timeAgo(job.created_at)}</span>
        </span>
        <span className="flex items-center gap-2">
          <Badge tone={RISK_TONE[job.risk.level] ?? "neutral"}>{t("automation.risk", { level: t(`automation.risk.${job.risk.level}`, undefined, job.risk.level) })}</Badge>
          <Badge tone={TONE[job.status] ?? "neutral"}>{t(`automation.status.${job.status}`, undefined, job.status)}</Badge>
        </span>
      </button>
      {open && (
        <div className="mt-3 space-y-3">
          {!!job.plan.steps?.length && <Block title={t("automation.steps")}><ol className="list-decimal space-y-1 pl-5">{job.plan.steps.map((s, i) => <li key={i} className="break-words">{s}</li>)}</ol></Block>}
          {!!job.plan.cli?.length && <Block title={t("automation.cli")}><pre className="overflow-x-auto rounded-lg bg-slate-100 p-3 text-xs">{job.plan.cli.join("\n")}</pre></Block>}
          {job.plan.terraform && <Block title="Terraform"><pre className="overflow-x-auto rounded-lg bg-slate-100 p-3 text-xs">{job.plan.terraform}</pre></Block>}
          {!!job.risk.factors?.length && <Block title={t("automation.risk_factors")}><ul className="list-disc space-y-1 pl-5 text-muted">{job.risk.factors.map((f, i) => <li key={i}>{f}</li>)}</ul></Block>}
          {job.rollback_plan && <Block title={t("automation.rollback")}><p className="whitespace-pre-line text-muted">{job.rollback_plan}</p></Block>}
          <p className="text-xs text-muted">
            {t("automation.created_by", { who: job.created_by })}
            {job.approved_at && ` · ${t("automation.approved_by", { who: job.approved_by ?? "", when: timeAgo(job.approved_at) })}`}
            {job.finished_at && ` · ${t("automation.finished_by", { who: job.finished_by ?? "", when: timeAgo(job.finished_at) })}`}
          </p>
          {job.result_note && <p className="rounded-lg bg-slate-50 px-3 py-2 text-muted ring-1 ring-line">{job.result_note}</p>}
          <p className="text-xs"><Link href="/recommendations" className="font-medium text-primary hover:underline">{t("automation.open_recommendation")}</Link></p>
          {canAct && ACTIVE.has(job.status) && (
            <div className="flex flex-wrap gap-2">
              {job.status === "planned" && <button className={btn} disabled={act.isPending} onClick={() => act.mutate("approve")}>{t("automation.approve")}</button>}
              {job.status === "approved" && <button className={btn} disabled={act.isPending} onClick={() => setReporting(!reporting)}>{t("automation.report")}</button>}
              <button className={btn} disabled={act.isPending} onClick={() => act.mutate("cancel")}>{t("automation.cancel")}</button>
            </div>
          )}
          {reporting && <ResultForm job={job} onDone={refresh} />}
          {act.isError && <p role="alert" className="text-xs text-bad">{act.error.message}</p>}
        </div>
      )}
    </li>
  );
}

export function AutomationView({ canSee, canAct }: { canSee: boolean; canAct: boolean }) {
  const t = useT();
  const [filter, setFilter] = useState<Filter>("active");
  const q = useAutomationPlans();
  const all = q.data ?? [];
  const shown = all.filter((j) => filter === "all" || (filter === "active" ? ACTIVE.has(j.status) : !ACTIVE.has(j.status)));
  return (
    <div className="space-y-5">
      <h1 className="text-2xl font-bold tracking-tight">{t("automation.title")}</h1>
      <div className="rounded-xl bg-sky-50 px-4 py-3 text-sm leading-relaxed text-sky-900 ring-1 ring-sky-100">{t("automation.banner")}</div>
      <Card index={0}>
        <div className="mb-3 flex flex-wrap gap-2" role="tablist">
          {FILTERS.map((f) => (
            <button key={f} role="tab" aria-selected={filter === f} onClick={() => setFilter(f)}
              className={`rounded-full px-3 py-1 text-xs font-semibold ring-1 ring-line ${filter === f ? "bg-primary text-white" : "hover:bg-slate-50"}`}>
              {t(`automation.filter.${f}`)}{q.data ? ` · ${all.filter((j) => f === "all" || (f === "active" ? ACTIVE.has(j.status) : !ACTIVE.has(j.status))).length}` : ""}
            </button>
          ))}
        </div>
        {!canSee ? <EmptyState icon={Workflow} text={t("automation.no_access")} /> :
          q.isLoading ? <Skeleton className="h-32" /> : q.isError ? <ErrorState /> : shown.length === 0 ? (
            <EmptyState icon={Workflow} text={all.length === 0 ? t("automation.empty") : t("automation.empty_filter")} action={{ href: "/recommendations", label: t("automation.open_recommendations") }} />
          ) : <ul className="divide-y divide-line">{shown.map((j) => <JobCard key={j.id} job={j} canAct={canAct} />)}</ul>}
      </Card>
    </div>
  );
}
