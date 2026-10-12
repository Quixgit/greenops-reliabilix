"use client";

import { useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { IconCheck, IconClock, IconMail, IconPlus, IconTrash } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { SelectField } from "@/components/ui/select-field";
import { useMembers } from "@/hooks/use-admin";
import { useProjects } from "@/hooks/use-overview";
import { useCreateSchedule, useDeleteSchedule, useProjectClouds, useReportOptions, useReportSchedules, useToggleSchedule, type ReportSchedule } from "@/hooks/use-reports";
import { CloudPicker } from "@/components/reports/cloud-picker";
import { useT } from "@/i18n/provider";
import { shortTime } from "@/lib/format-time";
import { cn } from "@/lib/utils";
import { cloudMeta } from "@/lib/clouds";

const KINDS = ["carbon", "finops", "sci", "sustainability", "ghg"] as const;

function Row({ s, i, canAct, project }: { s: ReportSchedule; i: number; canAct: boolean; project: string }) {
  const t = useT();
  const toggle = useToggleSchedule();
  const del = useDeleteSchedule();
  const [ask, setAsk] = useState(false);
  return (
    <li className="rise relative" style={{ "--i": Math.min(i, 8) } as CSSProperties}>
      <div className={cn("group flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-3 py-3 transition-all duration-300 hover:bg-slate-50", !s.enabled && "opacity-60")}>
        <span className={cn("grid size-10 shrink-0 place-items-center rounded-xl transition-colors", s.enabled ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-400")}><IconClock className="size-5" strokeWidth={1.6} /></span>
        <div className="min-w-0 flex-1 basis-56">
          <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-ink">
            {t(`rp.kind.${s.kind}`)}
            <span className="rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide text-slate-500">{s.format}</span>
            <span className="rounded-md bg-leaf-soft px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide text-leaf-deep">{t(`rs.cadence.${s.cadence}`)}</span>
          </p>
          <p className="mt-0.5 truncate text-xs text-muted">
            {project}{s.clouds.length > 0 && <> ({s.clouds.map((c) => cloudMeta(c).name).join(", ")})</>} · {s.enabled ? `${t("rs.next")}: ${shortTime(s.next_run_at)}` : t("rs.paused")}
            {s.recipients.length > 0 && <> · <IconMail className="-mt-0.5 inline size-3" /> {s.recipients.join(", ")}</>}
          </p>
        </div>
        {canAct && (
          <div className="flex items-center gap-2">
            <button type="button" role="switch" aria-checked={s.enabled} aria-label={s.enabled ? t("rs.pause") : t("rs.resume")} title={s.enabled ? t("rs.pause") : t("rs.resume")}
              disabled={toggle.isPending} onClick={() => toggle.mutate({ id: s.id, enabled: !s.enabled })}
              className={cn("relative h-6 w-11 rounded-full transition-colors duration-300", s.enabled ? "bg-leaf" : "bg-slate-300")}>
              <span className={cn("absolute left-0.5 top-0.5 size-5 rounded-full bg-white shadow transition-transform duration-300", s.enabled && "translate-x-5")} />
            </button>
            <button type="button" aria-label={t("rs.delete")} title={t("rs.delete")} onClick={() => setAsk(true)}
              className="grid size-8 place-items-center rounded-full text-slate-400 opacity-0 transition-all hover:bg-[#fdecef] hover:text-[#d9707f] focus-visible:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"><IconTrash className="size-4" strokeWidth={1.8} /></button>
          </div>
        )}
      </div>
      {ask && (
        <div className="del-wipe absolute inset-0 z-10 flex items-center justify-between gap-3 overflow-hidden rounded-2xl bg-[#fdf1f3] px-4 ring-1 ring-[#eba7b3]">
          <span className="text-sm font-medium text-[#a23b4d]">{t("rs.delete_ask")}</span>
          <span className="flex gap-2">
            <button type="button" onClick={() => setAsk(false)} className="rounded-lg bg-white px-3 py-1.5 text-xs font-semibold text-slate-600 ring-1 ring-line">{t("rs.cancel")}</button>
            <button type="button" disabled={del.isPending} onClick={() => del.mutate(s.id)} className="rounded-lg bg-[#d9707f] px-3 py-1.5 text-xs font-semibold text-white">{t("rs.delete")}</button>
          </span>
        </div>
      )}
    </li>
  );
}

function NewSchedule({ onDone }: { onDone: () => void }) {
  const t = useT();
  const opts = useReportOptions().data;
  const projects = useProjects().data ?? [];
  const members = useMembers();
  const create = useCreateSchedule();
  const [kind, setKind] = useState<string>("sustainability");
  const [cadence, setCadence] = useState<"weekly" | "monthly">("weekly");
  const [format, setFormat] = useState("pdf");
  const [project, setProject] = useState("");
  const [pickedClouds, setClouds] = useState<string[] | null>(null);
  const [to, setTo] = useState<string[]>([]);
  const today = new Date().toISOString().slice(0, 10), monthAgo = new Date(Date.now() - 30 * 86_400_000).toISOString().slice(0, 10);
  const available = useProjectClouds(project || null, monthAgo, today).data ?? [];
  const picked = pickedClouds === null ? null : pickedClouds.filter((c) => available.some((a) => a.provider === c));
  const pickedValue = picked !== null && picked.length === 0 ? null : picked;
  const clouds = pickedValue === null || pickedValue.length >= available.length ? [] : pickedValue;
  const formats = opts?.formats ?? ["pdf", "csv", "json"];
  const chosen = formats.includes(format) ? format : formats[0];
  const people = (members.data ?? []).filter((m) => m.email);
  const flip = (e: string) => setTo((v) => (v.includes(e) ? v.filter((x) => x !== e) : [...v, e]));
  const seg = (on: boolean) => cn("flex-1 rounded-lg px-3 py-1.5 text-sm font-semibold transition-all", on ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink");
  return (
    <div className="form-swap mb-4 rounded-2xl border border-line bg-slate-50/60 p-4">
      <div className="grid gap-4 md:grid-cols-2">
        <label className="block text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.kind")}
          <div className="mt-1.5 normal-case"><SelectField value={kind} onChange={(e) => setKind(e.target.value)} aria-label={t("rp.step.kind")} className="w-full rounded-xl border border-line bg-white px-3 py-2.5 text-sm font-medium">
            {KINDS.map((k) => <option key={k} value={k}>{t(`rp.kind.${k}`)}</option>)}
          </SelectField></div>
        </label>
        <label className="block text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.scope")}
          <div className="mt-1.5 normal-case"><SelectField value={project} onChange={(e) => setProject(e.target.value)} aria-label={t("rp.step.scope")} className="w-full rounded-xl border border-line bg-white px-3 py-2.5 text-sm font-medium">
            <option value="">{t("rp.all_projects")}</option>
            {projects.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
          </SelectField></div>
        </label>
        <div>
          <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rs.cadence")}</p>
          <div className="flex gap-1 rounded-xl bg-slate-100 p-1" role="radiogroup" aria-label={t("rs.cadence")}>
            {(["weekly", "monthly"] as const).map((c) => <button key={c} type="button" role="radio" aria-checked={cadence === c} onClick={() => setCadence(c)} className={seg(cadence === c)}>{t(`rs.cadence.${c}`)}</button>)}
          </div>
          <p className="mt-1.5 text-xs text-muted">{t(`rs.cadence.${cadence}_hint`)}</p>
        </div>
        <div>
          <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.format")}</p>
          <div className="flex gap-1 rounded-xl bg-slate-100 p-1" role="radiogroup" aria-label={t("rp.step.format")}>
            {["pdf", "csv", "json"].map((f) => <button key={f} type="button" role="radio" aria-checked={chosen === f} disabled={!formats.includes(f)} onClick={() => setFormat(f)} className={cn(seg(chosen === f), "uppercase disabled:cursor-not-allowed disabled:opacity-40")}>{f}</button>)}
          </div>
        </div>
      </div>
      {available.length > 1 && <div className="mt-4"><CloudPicker clouds={available} value={pickedValue} onChange={setClouds} /></div>}
      <div className="mt-4">
        <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rs.recipients")}</p>
        {members.isLoading ? <Skeleton className="h-8" /> : members.isError || people.length === 0 ? <p className="text-xs text-muted">{t("rs.recipients_none")}</p> : (
          <div className="flex flex-wrap gap-1.5">
            {people.map((m) => {
              const on = to.includes(m.email.toLowerCase());
              return (
                <button key={m.id} type="button" aria-pressed={on} onClick={() => flip(m.email.toLowerCase())}
                  className={cn("inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold transition-all duration-200", on ? "bg-leaf-deep text-white" : "bg-white text-slate-600 ring-1 ring-line hover:bg-leaf-soft hover:text-leaf-deep")}>
                  <IconCheck className={cn("size-3 transition-all duration-200", on ? "w-3 opacity-100" : "w-0 opacity-0")} strokeWidth={3} />{m.email}
                </button>
              );
            })}
          </div>
        )}
        <p className="mt-2 text-xs text-muted">{t("rs.recipients_hint")}</p>
      </div>
      <div className="mt-4 flex flex-wrap items-center justify-end gap-3">
        {create.isError && <span role="alert" className="text-sm text-bad">{create.error.message}</span>}
        <button type="button" onClick={onDone} className="rounded-xl px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-100">{t("rs.cancel")}</button>
        <button type="button" disabled={create.isPending} onClick={() => create.mutate({ kind, format: chosen, cadence, project_id: project || null, recipients: to, clouds }, { onSuccess: onDone })}
          className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-2 text-sm font-semibold"><IconClock className="size-4" strokeWidth={2} />{create.isPending ? t("rs.saving") : t("rs.create")}</button>
      </div>
    </div>
  );
}

/** Reports that make themselves: pick what, how often and whom to tell by e-mail; each run appears in the list below. */
export function ReportSchedules({ canAct }: { canAct: boolean }) {
  const t = useT();
  const q = useReportSchedules();
  const projects = useProjects().data ?? [];
  const [open, setOpen] = useState(false);
  const items = q.data ?? [];
  return (
    <Card index={2} interactive={false}>
      <CardHeader icon={<IconClock className="size-[18px] text-leaf-deep" />} title={t("rs.title")} info={t("rs.info")}
        action={canAct && !open ? <button type="button" onClick={() => setOpen(true)} className="inline-flex items-center gap-1.5 rounded-xl bg-leaf-soft px-3 py-1.5 text-xs font-semibold text-leaf-deep transition-colors hover:bg-leaf/30"><IconPlus className="size-3.5" strokeWidth={2.4} />{t("rs.new")}</button> : null} />
      {open && <NewSchedule onDone={() => setOpen(false)} />}
      {q.isLoading ? <Skeleton className="h-16" /> : q.isError ? <p role="alert" className="text-sm text-bad">{q.error.message}</p> : items.length === 0 ? (
        !open && <p className="rounded-2xl bg-slate-50 px-4 py-6 text-center text-sm text-muted">{t("rs.empty")}</p>
      ) : (
        <ul className="-mx-1 divide-y divide-line/60">
          {items.map((s, i) => <Row key={s.id} s={s} i={i} canAct={canAct} project={projects.find((p) => p.id === s.project_id)?.name ?? t("rp.all_projects")} />)}
        </ul>
      )}
    </Card>
  );
}
