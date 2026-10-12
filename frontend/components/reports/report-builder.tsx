"use client";

import { SelectField } from "@/components/ui/select-field";
import { useEffect, useMemo, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { DateField } from "@/components/ui/date-field";
import { IconBolt, IconCheck, IconCoin, IconDownload, IconGauge, IconLeaf, IconReport } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { useProjects, useReports } from "@/hooks/use-overview";
import { useReportOptions, useReportPreview, useRequestReport } from "@/hooks/use-reports";
import { useT } from "@/i18n/provider";
import { cn } from "@/lib/utils";

const KINDS = [
  { id: "carbon", icon: IconLeaf, tone: "text-leaf-deep" },
  { id: "finops", icon: IconCoin, tone: "text-[#5b8fdc]" },
  { id: "sci", icon: IconGauge, tone: "text-[#8a78d8]" },
  { id: "sustainability", icon: IconBolt, tone: "text-[#d9a441]" },
  { id: "ghg", icon: IconLeaf, tone: "text-[#3f9d7a]" },
] as const;

const iso = (d: Date) => d.toISOString().slice(0, 10);
const utcDay = (offset = 0) => { const n = new Date(); return new Date(Date.UTC(n.getFullYear(), n.getMonth(), n.getDate() + offset)); };
const PRESETS: { id: string; range: () => [string, string] }[] = [
  { id: "7d", range: () => [iso(utcDay(-6)), iso(utcDay())] },
  { id: "30d", range: () => [iso(utcDay(-29)), iso(utcDay())] },
  { id: "90d", range: () => [iso(utcDay(-89)), iso(utcDay())] },
  { id: "month", range: () => { const n = utcDay(); return [iso(new Date(Date.UTC(n.getUTCFullYear(), n.getUTCMonth(), 1))), iso(n)]; } },
  { id: "last_month", range: () => { const n = utcDay(); return [iso(new Date(Date.UTC(n.getUTCFullYear(), n.getUTCMonth() - 1, 1))), iso(new Date(Date.UTC(n.getUTCFullYear(), n.getUTCMonth(), 0)))]; } },
];
const daysBetween = (a: string, b: string) => Math.round((Date.parse(b) - Date.parse(a)) / 86_400_000) + 1;

/** Pick what the report is about, see its first rows right away, then ask for the file. */
export function ReportBuilder({ onRequested }: { onRequested: () => void }) {
  const t = useT();
  const opts = useReportOptions();
  const projects = useProjects().data ?? [];
  const request = useRequestReport();
  const [kind, setKind] = useState<string>("carbon");
  const [format, setFormat] = useState<string>("pdf");
  const [project, setProject] = useState<string>("");
  const [preset, setPreset] = useState<string>("30d");
  const [range, setRange] = useState<[string, string]>(() => PRESETS[1].range());
  const [from, to] = range;

  // Progress of the report just requested: it eases toward 92% while it renders, then fills when the file is ready.
  const [jobId, setJobId] = useState<string | null>(null);
  const [pct, setPct] = useState(0);
  const [phase, setPhase] = useState<"idle" | "run" | "done" | "fail">("idle");
  const job = (useReports().data ?? []).find((r) => r.id === jobId);
  useEffect(() => {
    if (phase !== "run") return;
    const id = window.setInterval(() => setPct((v) => v + (92 - v) * 0.08), 200);
    return () => window.clearInterval(id);
  }, [phase]);
  useEffect(() => {
    if (!jobId || phase !== "run" || !job) return;
    if (job.status === "ready") { setPct(100); setPhase("done"); }
    else if (job.status === "failed") { setPhase("fail"); }
  }, [job, jobId, phase]);
  useEffect(() => {
    if (phase !== "done") return;
    const id = window.setTimeout(() => { setPhase("idle"); setPct(0); setJobId(null); }, 1800);
    return () => window.clearTimeout(id);
  }, [phase]);

  const maxDays = opts.data?.max_days ?? 400;
  const formats = opts.data?.formats ?? ["pdf", "csv", "json"];
  const chosenFormat = formats.includes(format) ? format : formats[0];
  const days = from && to ? daysBetween(from, to) : 0;
  const problem = !from || !to ? t("rp.err.dates") : days < 1 ? t("rp.err.order") : days > maxDays ? t("rp.err.long", { max: maxDays }) : null;
  const ok = !problem && !!chosenFormat;
  const preview = useReportPreview({ kind, project_id: project || null, period_start: from, period_end: to }, ok);
  const p = preview.data;
  const projectName = projects.find((x) => x.id === project)?.name ?? t("rp.all_projects");
  const summary = useMemo(() => `${t(`rp.kind.${kind}`)} · ${chosenFormat?.toUpperCase()} · ${projectName} · ${from} → ${to}`, [t, kind, chosenFormat, projectName, from, to]);

  const setPresetRange = (id: string) => { setPreset(id); setRange(PRESETS.find((x) => x.id === id)!.range()); };
  const submit = () => {
    setPct(4); setPhase("run");
    request.mutate({ kind, format: chosenFormat, project_id: project || null, period_start: from, period_end: to }, {
      onSuccess: (r) => { setJobId(r.id); onRequested(); },
      onError: () => { setPhase("idle"); setPct(0); },
    });
  };
  const busy = phase === "run";

  return (
    <Card index={1} interactive={false} className="relative overflow-hidden">
      <div aria-hidden className={cn("absolute inset-x-0 top-0 h-[3px] bg-slate-100 transition-opacity duration-500", phase === "idle" ? "opacity-0" : "opacity-100")}>
        <div className={cn("h-full rounded-r-full transition-[width,background-color] duration-300 ease-out", phase === "fail" ? "bg-[#e08b98]" : "bg-gradient-to-r from-leaf to-leaf-deep", busy && "progress-glow")} style={{ width: `${phase === "fail" ? 100 : pct}%` }} />
      </div>
      <CardHeader icon={<IconReport className="size-[18px] text-leaf-deep" />} title={t("rp.new")} info={t("rp.info.new")} />

      <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.kind")}</p>
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" role="radiogroup" aria-label={t("rp.step.kind")}>
        {KINDS.map((k, i) => {
          const on = kind === k.id;
          return (
            <button key={k.id} type="button" role="radio" aria-checked={on} onClick={() => setKind(k.id)} style={{ "--i": i } as CSSProperties}
              className={cn("rise group relative flex items-start gap-3 rounded-2xl border p-4 text-left transition-all duration-300 hover:-translate-y-0.5",
                on ? "border-leaf bg-leaf-soft/60 shadow-[0_0_0_3px_rgba(111,195,168,0.18)]" : "border-line bg-white hover:border-leaf/50")}>
              <k.icon className={cn("size-10 shrink-0 transition-transform duration-500 group-hover:scale-110", k.tone)} strokeWidth={1.4} />
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-ink">{t(`rp.kind.${k.id}`)}</span>
                <span className="mt-0.5 block text-xs leading-snug text-muted">{t(`rp.kind.${k.id}_hint`)}</span>
              </span>
              <span className={cn("absolute right-3 top-3 grid size-5 place-items-center rounded-full bg-leaf text-white transition-all duration-300", on ? "scale-100 opacity-100" : "scale-50 opacity-0")}><IconCheck className="size-3" strokeWidth={3} /></span>
            </button>
          );
        })}
      </div>

      <div className="mt-6 grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <div className="space-y-4">
          <div>
            <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.scope")}</p>
            <SelectField value={project} onChange={(e) => setProject(e.target.value)} aria-label={t("rp.step.scope")} className="w-full rounded-xl border border-line bg-white px-3 py-2.5 text-sm font-medium">
              <option value="">{t("rp.all_projects")}</option>
              {projects.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
            </SelectField>
          </div>
          <div>
            <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.format")}</p>
            <div className="flex gap-1 rounded-xl bg-slate-100 p-1 text-sm" role="radiogroup" aria-label={t("rp.step.format")}>
              {["pdf", "csv", "json"].map((f) => {
                const allowed = formats.includes(f);
                return (
                  <button key={f} type="button" role="radio" aria-checked={chosenFormat === f} disabled={!allowed} onClick={() => setFormat(f)} title={allowed ? undefined : t("rp.format_off")}
                    className={cn("flex-1 rounded-lg px-3 py-1.5 font-semibold uppercase transition-all disabled:cursor-not-allowed disabled:opacity-40", chosenFormat === f ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{f}</button>
                );
              })}
            </div>
            <p className="mt-1.5 text-xs text-muted">{t(`rp.format.${chosenFormat ?? "pdf"}`, { rows: opts.data?.pdf_rows ?? 2000 })}</p>
          </div>
        </div>
        <div>
          <p className="mb-1.5 text-xs font-semibold uppercase tracking-wide text-slate-500">{t("rp.step.period")}</p>
          <div className="mb-3 flex flex-wrap gap-1.5">
            {PRESETS.map((x) => (
              <button key={x.id} type="button" onClick={() => setPresetRange(x.id)} aria-pressed={preset === x.id}
                className={cn("rounded-full px-3 py-1 text-xs font-semibold transition-all", preset === x.id ? "bg-leaf-deep text-white" : "bg-slate-100 text-slate-600 hover:bg-leaf-soft hover:text-leaf-deep")}>{t(`rp.preset.${x.id}`)}</button>
            ))}
          </div>
          <div className="grid grid-cols-2 gap-3">
            <label className="block text-xs font-medium text-slate-500">{t("rp.from")}<div className="mt-1"><DateField value={from} onChange={(v) => { setPreset(""); setRange([v, to]); }} max={to || undefined} label={t("rp.from")} /></div></label>
            <label className="block text-xs font-medium text-slate-500">{t("rp.to")}<div className="mt-1"><DateField value={to} onChange={(v) => { setPreset(""); setRange([from, v]); }} min={from || undefined} label={t("rp.to")} /></div></label>
          </div>
          <p className={cn("mt-2 text-xs", problem ? "text-bad" : "text-muted")} role={problem ? "alert" : undefined}>{problem ?? t("rp.days", { count: days })}</p>
        </div>
      </div>

      <div className="mt-6 overflow-hidden rounded-2xl border border-line">
        <div className="flex items-center justify-between gap-3 border-b border-line/70 bg-slate-50/70 px-4 py-2.5">
          <p className="text-sm font-semibold">{t("rp.preview")}</p>
          {p && <span className="text-xs text-muted">{t("rp.preview_rows", { shown: p.rows.length, total: p.total_rows })}</span>}
        </div>
        {!ok ? <p className="px-4 py-8 text-center text-sm text-muted">{t("rp.preview_wait")}</p>
          : preview.isLoading ? <div className="p-4"><Skeleton className="h-32" /></div>
          : preview.isError ? <p role="alert" className="px-4 py-8 text-center text-sm text-bad">{preview.error.message}</p>
          : !p || p.rows.length === 0 ? <p className="px-4 py-8 text-center text-sm text-muted">{t("rp.preview_empty")}</p> : (
            <div key={`${kind}${project}${from}${to}`} className="form-swap overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead><tr className="text-slate-500">{p.headers.map((h) => <th key={h} className="whitespace-nowrap px-4 py-2 font-semibold">{h}</th>)}</tr></thead>
                <tbody>
                  {p.rows.map((r, i) => (
                    <tr key={i} className="border-t border-line/60">{r.map((c, j) => <td key={j} className="whitespace-nowrap px-4 py-1.5 tabular-nums">{c}</td>)}</tr>
                  ))}
                </tbody>
              </table>
              {p.notes.length > 0 && <p className="border-t border-line/60 bg-slate-50/50 px-4 py-2 text-[11px] leading-snug text-muted">{p.notes[0]}</p>}
            </div>
          )}
      </div>

      <div className="mt-5 flex flex-wrap items-center justify-between gap-3">
        <p className="min-w-0 truncate text-sm text-slate-600">{ok ? summary : ""}</p>
        <div className="flex items-center gap-3">
          {phase !== "idle" && (
            <span role="status" className={cn("text-xs font-medium tabular-nums", phase === "fail" ? "text-[#c25667]" : phase === "done" ? "text-leaf-deep" : "text-slate-500")}>
              {phase === "run" ? `${t("rp.progress")} ${Math.round(pct)}%` : phase === "done" ? t("rp.progress_done") : t("rp.progress_fail")}
            </span>
          )}
          {request.isError && <span role="alert" className="text-sm text-bad">{request.error.message}</span>}
          <button type="button" disabled={!ok || request.isPending || busy} onClick={submit}
            className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold">
            <IconDownload className="size-4" strokeWidth={2} />{request.isPending ? t("rp.requesting") : t("rp.generate")}
          </button>
        </div>
      </div>
    </Card>
  );
}
