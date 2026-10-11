"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import Link from "next/link";
import { useEffect, useMemo, useState, type CSSProperties, type ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { IconCheck, IconCheckCircle, IconClipboard, IconCopy, IconFlow, IconGlobe, IconPlay, IconPower, IconServer, IconShield, IconTrend, IconWarning, type IconType } from "@/components/ui/icons";
import { Info } from "@/components/ui/info";
import { Skeleton } from "@/components/ui/skeleton";
import { useAutomationPlans, useFinopsSummary, useProjects, useRecommendations } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { automationJob, type AutomationJob, type Recommendation } from "@/lib/api/schemas";
import { formatCost, nf, timeAgo } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const LIVE = new Set(["planned", "approved"]);
const KIND_ICON: Record<string, IconType> = { ec2_resize: IconServer, ec2_terminate: IconPower, region_shift: IconGlobe };
const RISK: Record<string, string> = { low: "bg-leaf-soft text-leaf-deep", medium: "bg-[#fdf3dc] text-[#a9741a]", high: "bg-[#fdecef] text-[#c25667]" };
const STATUS: Record<string, string> = {
  planned: "bg-[#eef3fd] text-[#4a6fb0]", approved: "bg-[#fdf3dc] text-[#a9741a]", completed: "bg-leaf-soft text-leaf-deep",
  failed: "bg-[#fdecef] text-[#c25667]", rolled_back: "bg-[#f1ecfc] text-[#7a68c8]", cancelled: "bg-slate-100 text-slate-500",
};
const PLANNABLE = (r: Recommendation) => r.type === "region_shift" || (r.type === "rightsizing" && (r.details?.action === "modify" || r.details?.action === "terminate"));

export type Stage = "ready" | "review" | "apply" | "done";
const STAGES: { id: Stage; icon: IconType; tone: string }[] = [
  { id: "ready", icon: IconClipboard, tone: "text-[#5b8fdc]" },
  { id: "review", icon: IconShield, tone: "text-[#d9a441]" },
  { id: "apply", icon: IconPlay, tone: "text-leaf-deep" },
  { id: "done", icon: IconCheckCircle, tone: "text-[#8a78d8]" },
];

function useBoard() {
  const jobs = useAutomationPlans();
  const approved = useRecommendations("approved");
  const all = useRecommendations("all");
  const projects = useProjects().data ?? [];
  const list = jobs.data ?? [];
  const liveRecs = new Set(list.filter((j) => LIVE.has(j.status)).map((j) => j.recommendation_id));
  const ready = (approved.data ?? []).filter((r) => PLANNABLE(r) && !liveRecs.has(r.id));
  const recById = useMemo(() => new Map([...(all.data ?? []), ...(approved.data ?? [])].map((r) => [r.id, r])), [all.data, approved.data]);
  return {
    jobs, loading: jobs.isLoading || approved.isLoading, error: jobs.isError,
    ready, review: list.filter((j) => j.status === "planned"), apply: list.filter((j) => j.status === "approved"),
    done: list.filter((j) => !LIVE.has(j.status)), recById, projectName: (id: string) => projects.find((p) => p.id === id)?.name ?? "",
  };
}

/** The four steps a change goes through, as one clickable pipeline with live counts. */
export function Pipeline({ stage, onStage }: { stage: Stage; onStage: (s: Stage) => void }) {
  const t = useT();
  const b = useBoard();
  const counts: Record<Stage, number> = { ready: b.ready.length, review: b.review.length, apply: b.apply.length, done: b.done.length };
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" role="tablist" aria-label={t("au.title")}>
      {STAGES.map((s, i) => {
        const on = stage === s.id;
        return (
          <Card key={s.id} index={i} interactive={false} className="relative p-0">
            <button type="button" role="tab" aria-selected={on} onClick={() => onStage(s.id)}
              className={cn("group flex h-full w-full items-center gap-4 rounded-[inherit] p-4 text-left transition-all duration-300 hover:-translate-y-0.5", on && "bg-leaf-soft/50")}>
              <s.icon className={cn("size-11 shrink-0 transition-transform duration-500 group-hover:scale-110 group-hover:-rotate-6", s.tone)} strokeWidth={1.4} />
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-1 text-[13px] font-medium text-slate-600">{t(`au.stage.${s.id}`)}</span>
                <span className="mt-0.5 block text-2xl font-bold tabular-nums tracking-tight">{b.loading ? "–" : <CountUp value={counts[s.id]} format={(n) => String(Math.round(n))} />}</span>
                <span className="block truncate text-xs text-muted">{t(`au.stage.${s.id}_hint`)}</span>
              </span>
              {i < STAGES.length - 1 && <span aria-hidden className="absolute -right-3 top-1/2 z-10 hidden -translate-y-1/2 text-slate-300 xl:block">›</span>}
            </button>
            <span aria-hidden className={cn("absolute inset-x-4 bottom-0 h-0.5 origin-left rounded-full bg-leaf-deep transition-transform duration-300", on ? "scale-x-100" : "scale-x-0")} />
          </Card>
        );
      })}
    </div>
  );
}

function CopyBlock({ title, text }: { title: string; text: string }) {
  const t = useT();
  const [done, setDone] = useState(false);
  return (
    <div>
      <div className="mb-1.5 flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</p>
        <button type="button" onClick={() => { void navigator.clipboard?.writeText(text); setDone(true); window.setTimeout(() => setDone(false), 1500); }}
          className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink">
          {done ? <IconCheck className="size-3.5 text-leaf-deep" strokeWidth={2.6} /> : <IconCopy className="size-3.5" />}{done ? t("au.copied") : t("au.copy")}
        </button>
      </div>
      <pre className="overflow-x-auto rounded-xl bg-slate-900 px-4 py-3 text-xs leading-relaxed text-slate-100">{text}</pre>
    </div>
  );
}

/** Steps the person ticks off while applying the plan; the ticks stay in this browser. */
function Checklist({ id, steps }: { id: string; steps: string[] }) {
  const key = `rlx.plan.${id}`;
  const [done, setDone] = useState<number[]>([]);
  useEffect(() => { try { setDone(JSON.parse(localStorage.getItem(key) ?? "[]") as number[]); } catch { /* storage blocked */ } }, [key]);
  const toggle = (i: number) => setDone((d) => {
    const n = d.includes(i) ? d.filter((x) => x !== i) : [...d, i];
    try { localStorage.setItem(key, JSON.stringify(n)); } catch { /* ignore */ }
    return n;
  });
  return (
    <div>
      <div className="mb-2 h-1 overflow-hidden rounded-full bg-slate-100"><span className="block h-full rounded-full bg-leaf transition-all duration-500" style={{ width: `${steps.length ? (done.length / steps.length) * 100 : 0}%` }} /></div>
      <ol className="space-y-1.5">
        {steps.map((s, i) => (
          <li key={i}>
            <label className="flex cursor-pointer items-start gap-3 rounded-lg px-1 py-1 text-sm transition-colors hover:bg-slate-50">
              <input type="checkbox" className="peer sr-only" checked={done.includes(i)} onChange={() => toggle(i)} />
              <span className="mt-0.5 grid size-[18px] shrink-0 place-items-center rounded-md border border-slate-300 bg-white transition-all peer-checked:border-leaf peer-checked:bg-leaf peer-focus-visible:shadow-[0_0_0_3px_rgba(111,195,168,0.25)]">
                <IconCheck className={cn("size-3 text-white transition-all", done.includes(i) ? "scale-100 opacity-100" : "scale-50 opacity-0")} strokeWidth={3} />
              </span>
              <span className={cn("transition-colors", done.includes(i) && "text-slate-400 line-through")}>{s}</span>
            </label>
          </li>
        ))}
      </ol>
    </div>
  );
}

function ResultForm({ job, rolledBackOnly, onDone }: { job: AutomationJob; rolledBackOnly?: boolean; onDone: () => void }) {
  const t = useT();
  const [outcome, setOutcome] = useState<"completed" | "failed" | "rolled_back">(rolledBackOnly ? "rolled_back" : "completed");
  const [note, setNote] = useState("");
  const needsNote = outcome !== "completed";
  const send = useMutation({
    mutationFn: () => api(`automation/jobs/${job.id}/result`, automationJob, undefined, { method: "POST", body: JSON.stringify({ outcome, note: note.trim() || undefined }) }),
    onSuccess: onDone,
  });
  const opts = rolledBackOnly ? (["rolled_back"] as const) : (["completed", "failed", "rolled_back"] as const);
  return (
    <form onSubmit={(e) => { e.preventDefault(); send.mutate(); }} className="form-swap space-y-3 rounded-2xl bg-slate-50 p-4 ring-1 ring-line">
      <p className="text-sm font-semibold">{rolledBackOnly ? t("au.rollback_title") : t("automation.report_title")}</p>
      <div className="flex flex-wrap gap-1.5" role="radiogroup" aria-label={t("automation.outcome")}>
        {opts.map((o) => (
          <button key={o} type="button" role="radio" aria-checked={outcome === o} onClick={() => setOutcome(o)}
            className={cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", outcome === o ? "bg-leaf-deep text-white" : "bg-white text-slate-600 ring-1 ring-line hover:text-ink")}>{t(`automation.outcome.${o}`)}</button>
        ))}
      </div>
      <textarea value={note} onChange={(e) => setNote(e.target.value)} maxLength={2000} rows={3}
        placeholder={needsNote ? t("automation.note_required") : t("automation.note_optional")} className="block w-full rounded-xl border border-line bg-white px-3 py-2 text-sm" />
      <div className="flex items-center gap-3">
        <button disabled={send.isPending || (needsNote && note.trim() === "")} className="btn-leaf rounded-xl px-4 py-2 text-sm font-semibold">{t("automation.report_send")}</button>
        {send.isError && <p role="alert" className="text-xs text-bad">{send.error.message}</p>}
      </div>
    </form>
  );
}

const historySchema = z.object({ items: z.array(z.object({ action: z.string(), actor: z.string(), at: z.string(), note: z.string().nullish() })) });

/** Every recorded step of the plan from the audit trail, including those the plan row has since overwritten. */
function History({ id }: { id: string }) {
  const t = useT();
  const q = useQuery({ queryKey: ["automation-history", id], queryFn: () => api(`automation/jobs/${id}/history`, historySchema) });
  const rows = q.data?.items ?? [];
  if (q.isLoading || rows.length === 0) return null;
  return (
    <div>
      <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("au.history")}</p>
      <ol className="relative space-y-3 border-l border-line pl-5">
        {rows.map((e, i) => (
          <li key={i} className="relative text-sm">
            <span className={cn("absolute -left-[26px] top-1.5 size-2.5 rounded-full ring-4 ring-white", e.action === "failed" || e.action === "rolled_back" ? "bg-[#e08b98]" : e.action === "cancelled" ? "bg-slate-300" : "bg-leaf")} />
            <p className="font-semibold text-ink">{t(`au.hist.${e.action}`, undefined, e.action)} <span className="font-normal text-muted">· {e.actor} · {timeAgo(e.at)}</span></p>
            {e.note && <p className="mt-0.5 text-slate-600">{e.note}</p>}
          </li>
        ))}
      </ol>
    </div>
  );
}

function Dot({ on, bad, label, sub }: { on: boolean; bad?: boolean; label: string; sub?: string }) {
  return (
    <li className="flex items-start gap-2.5">
      <span className={cn("mt-1 size-2.5 shrink-0 rounded-full ring-4", on ? (bad ? "bg-[#e08b98] ring-[#fdecef]" : "bg-leaf ring-leaf-soft") : "bg-slate-200 ring-slate-100")} />
      <span className="text-xs leading-snug"><span className={cn("block font-semibold", on ? "text-ink" : "text-slate-400")}>{label}</span>{sub && <span className="block text-muted">{sub}</span>}</span>
    </li>
  );
}

function Impact({ r }: { r?: Recommendation }) {
  const t = useT();
  const cur = useFinopsSummary().data?.currency || "USD";
  if (!r) return null;
  return (
    <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-slate-600">
      <span className="flex items-center gap-1 font-semibold text-leaf-deep"><IconTrend className="size-3.5" />−{nf(0).format(r.estimated_carbon_reduction_pct)}% · {nf(1).format(r.carbon_reduction_kg_month)} kg CO₂e/{t("au.mo")}</span>
      {r.estimated_cost_impact !== null && <span className="text-slate-600">{r.estimated_cost_impact < 0 ? t("au.saving") : t("au.increase")} {formatCost(Math.abs(r.estimated_cost_impact), cur)}/{t("au.mo")}</span>}
    </p>
  );
}

function JobCard({ job, rec, project, canAct, index }: { job: AutomationJob; rec?: Recommendation; project: string; canAct: boolean; index: number }) {
  const t = useT();
  const qc = useQueryClient();
  const [open, setOpen] = useState(job.status === "approved");
  const [reporting, setReporting] = useState(false);
  const Icon = KIND_ICON[job.kind] ?? IconFlow;
  const refresh = () => { setReporting(false); for (const k of ["automation-jobs", "automation-jobs-lite", "recommendations"]) void qc.invalidateQueries({ queryKey: [k] }); };
  const act = useMutation({
    mutationFn: (verb: "approve" | "cancel") => api(`automation/jobs/${job.id}/${verb}`, automationJob, undefined, { method: "POST", body: "{}" }),
    onSuccess: refresh,
  });
  const fin = job.status !== "planned" && job.status !== "approved";
  return (
    <li className="rise rounded-2xl border border-line bg-white transition-shadow duration-300 hover:shadow-[0_10px_28px_-18px_rgba(47,143,120,0.5)]" style={{ "--i": Math.min(index, 8) } as CSSProperties}>
      <button type="button" onClick={() => setOpen(!open)} aria-expanded={open} className="flex w-full items-start gap-4 rounded-2xl p-4 text-left">
        <Icon className="size-11 shrink-0 text-leaf-deep" strokeWidth={1.4} />
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-ink">{job.plan.summary}</span>
          <span className="mt-0.5 block text-xs text-muted">{t(`automation.kind.${job.kind}`, undefined, job.kind)}{project && ` · ${project}`} · {timeAgo(job.created_at)}</span>
          <Impact r={rec} />
        </span>
        <span className="flex shrink-0 flex-col items-end gap-1.5">
          <span className={cn("rounded-full px-2.5 py-0.5 text-[11px] font-semibold", STATUS[job.status] ?? STATUS.cancelled)}>{t(`automation.status.${job.status}`, undefined, job.status)}</span>
          <span className={cn("rounded-full px-2.5 py-0.5 text-[11px] font-semibold", RISK[job.risk.level] ?? RISK.low)}>{t("automation.risk", { level: t(`automation.risk.${job.risk.level}`, undefined, job.risk.level) })}</span>
        </span>
      </button>
      <div className={cn("grid transition-[grid-template-rows] duration-500 ease-out", open ? "grid-rows-[1fr]" : "grid-rows-[0fr]")}>
        <div className="overflow-hidden">
          <div className="space-y-5 border-t border-line/70 px-4 py-5 sm:px-6">
            <ol className="grid gap-3 sm:grid-cols-3">
              <Dot on label={t("au.tl.planned")} sub={`${job.created_by} · ${timeAgo(job.created_at)}`} />
              <Dot on={!!job.approved_at} label={t("au.tl.approved")} sub={job.approved_at ? `${job.approved_by ?? ""} · ${timeAgo(job.approved_at)}` : t("au.tl.waiting")} />
              <Dot on={fin && !!job.finished_at} bad={job.status === "failed" || job.status === "rolled_back"} label={job.status === "cancelled" ? t("automation.status.cancelled") : fin ? t(`automation.status.${job.status}`, undefined, job.status) : t("au.tl.applied")}
                sub={job.finished_at ? `${job.finished_by ?? ""} · ${timeAgo(job.finished_at)}` : t("au.tl.waiting")} />
            </ol>
            {!!job.risk.factors?.length && (
              <div className="rounded-2xl bg-slate-50 p-4">
                <p className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-500"><IconWarning className="size-4 text-[#d9a441]" />{t("automation.risk_factors")}</p>
                <ul className="space-y-1 text-sm text-slate-600">{job.risk.factors.map((f, i) => <li key={i} className="flex gap-2"><span className="mt-2 size-1.5 shrink-0 rounded-full bg-slate-300" />{f}</li>)}</ul>
              </div>
            )}
            {!!job.plan.steps?.length && (
              <div><p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("automation.steps")}</p>
                {LIVE.has(job.status) ? <Checklist id={job.id} steps={job.plan.steps} /> : (
                  <ol className="list-decimal space-y-1 pl-5 text-sm text-slate-600">{job.plan.steps.map((s, i) => <li key={i}>{s}</li>)}</ol>)}
              </div>
            )}
            {!!job.plan.cli?.length && <CopyBlock title={t("automation.cli")} text={job.plan.cli.join("\n")} />}
            {job.plan.terraform && <CopyBlock title="Terraform" text={job.plan.terraform} />}
            {job.rollback_plan && (
              <div className="rounded-2xl bg-[#f1ecfc]/60 p-4">
                <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-[#7a68c8]">{t("automation.rollback")}</p>
                <p className="whitespace-pre-line text-sm text-slate-600">{job.rollback_plan}</p>
              </div>
            )}
            {open && <History id={job.id} />}
            <div className="flex flex-wrap items-center gap-2">
              {canAct && job.status === "planned" && <button type="button" className="btn-leaf rounded-xl px-4 py-2 text-sm font-semibold" disabled={act.isPending} onClick={() => act.mutate("approve")}>{t("automation.approve")}</button>}
              {canAct && job.status === "approved" && <button type="button" className="btn-leaf rounded-xl px-4 py-2 text-sm font-semibold" disabled={act.isPending} onClick={() => setReporting(!reporting)}>{t("automation.report")}</button>}
              {canAct && job.status === "completed" && <button type="button" className="rounded-xl bg-slate-100 px-4 py-2 text-sm font-semibold text-slate-700 transition-colors hover:bg-[#f1ecfc] hover:text-[#7a68c8]" onClick={() => setReporting(!reporting)}>{t("au.mark_rollback")}</button>}
              {canAct && LIVE.has(job.status) && <button type="button" className="rounded-xl px-4 py-2 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink" disabled={act.isPending} onClick={() => act.mutate("cancel")}>{t("automation.cancel")}</button>}
              <Link href="/recommendations" className="ml-auto text-sm font-medium text-leaf-deep hover:underline">{t("automation.open_recommendation")} →</Link>
            </div>
            {reporting && <ResultForm job={job} rolledBackOnly={job.status === "completed"} onDone={refresh} />}
            {act.isError && <p role="alert" className="text-sm text-bad">{act.error.message}</p>}
          </div>
        </div>
      </div>
    </li>
  );
}

function ReadyCard({ r, project, canAct, index }: { r: Recommendation; project: string; canAct: boolean; index: number }) {
  const t = useT();
  const qc = useQueryClient();
  const plan = useMutation({
    mutationFn: () => api("automation/jobs", automationJob, undefined, { method: "POST", body: JSON.stringify({ recommendation_id: r.id }) }),
    onSuccess: () => { for (const k of ["automation-jobs", "automation-jobs-lite"]) void qc.invalidateQueries({ queryKey: [k] }); },
  });
  const Icon = r.type === "region_shift" ? IconGlobe : r.details?.action === "terminate" ? IconPower : IconServer;
  return (
    <li className="rise flex flex-wrap items-center gap-4 rounded-2xl border border-line bg-white p-4 transition-shadow duration-300 hover:shadow-[0_10px_28px_-18px_rgba(47,143,120,0.5)]" style={{ "--i": Math.min(index, 8) } as CSSProperties}>
      <Icon className="size-11 shrink-0 text-[#5b8fdc]" strokeWidth={1.4} />
      <div className="min-w-0 flex-1 basis-64">
        <p className="text-sm font-semibold text-ink">{r.title}</p>
        <p className="mt-0.5 truncate text-xs text-muted">{project && `${project} · `}{r.provider} · {r.service_name} · {r.current_region}{r.recommended_region && ` → ${r.recommended_region}`}</p>
        <Impact r={r} />
      </div>
      {canAct && (
        <div className="flex items-center gap-3">
          {plan.isError && <span role="alert" className="max-w-[16rem] text-xs text-bad">{plan.error.message}</span>}
          <button type="button" disabled={plan.isPending} onClick={() => plan.mutate()} className="btn-leaf rounded-xl px-4 py-2 text-sm font-semibold">{plan.isPending ? t("au.planning") : t("au.make_plan")}</button>
        </div>
      )}
    </li>
  );
}

const EMPTY: Record<Stage, string> = { ready: "au.empty.ready", review: "au.empty.review", apply: "au.empty.apply", done: "au.empty.done" };

/** The list of the chosen stage: recommendations waiting for a plan, or plans waiting for a decision, an application or a record. */
export function StageList({ stage, canAct }: { stage: Stage; canAct: boolean }) {
  const t = useT();
  const b = useBoard();
  if (b.loading) return <Skeleton className="h-48" />;
  if (b.error) return <ErrorState />;
  const rows: ReactNode[] = stage === "ready"
    ? b.ready.map((r, i) => <ReadyCard key={r.id} r={r} index={i} canAct={canAct} project={b.projectName(r.project_id)} />)
    : (stage === "review" ? b.review : stage === "apply" ? b.apply : b.done)
        .slice().sort((a, z) => Date.parse(z.created_at) - Date.parse(a.created_at))
        .map((j, i) => <JobCard key={j.id} job={j} index={i} canAct={canAct} rec={b.recById.get(j.recommendation_id)} project={b.projectName(j.project_id)} />);
  if (rows.length === 0) {
    return <Card interactive={false}><EmptyState icon={IconFlow} text={t(EMPTY[stage])} action={stage === "ready" ? { href: "/recommendations", label: t("automation.open_recommendations") } : undefined} /></Card>;
  }
  return <ul key={stage} className="space-y-3">{rows}</ul>;
}

export function SafetyNote() {
  const t = useT();
  return (
    <div className="flex items-start gap-3 rounded-2xl bg-[#eef3fd] px-4 py-3 text-sm leading-relaxed text-[#3d5a8c]">
      <IconShield className="mt-0.5 size-5 shrink-0" strokeWidth={1.6} />
      <p>{t("automation.banner")} <Info>{t("au.info.safety")}</Info></p>
    </div>
  );
}
