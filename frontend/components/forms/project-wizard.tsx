"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useRef, useState, type CSSProperties } from "react";
import { z } from "zod";
import { IconArrowLeft, IconArrowRight, IconCheck, IconFolder, IconGauge, IconLeaf, IconUsers } from "@/components/ui/icons";
import { useTeams } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { project, type Project } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";

const STEPS = ["name", "unit", "team", "review"] as const;
const UNITS = ["request", "build", "job", "user", "transaction", "GB processed"];
const field = "w-full rounded-xl border border-line bg-white px-4 py-3 text-base outline-none transition-all placeholder:text-slate-400 hover:border-slate-300 focus:border-leaf focus:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]";

/** Create a project one question at a time, in place of the project list; ends with what to do next. */
export function ProjectWizard({ onClose, onOpen }: { onClose: () => void; onOpen: (id: string) => void }) {
  const t = useT();
  const qc = useQueryClient();
  const teams = useTeams().data ?? [];
  const [step, setStep] = useState(0);
  const [dir, setDir] = useState(1);
  const [name, setName] = useState("");
  const [unit, setUnit] = useState("request");
  const [teamId, setTeamId] = useState<string | null>(null);
  const [created, setCreated] = useState<Project | null>(null);
  const first = useRef<HTMLInputElement>(null);

  const nameErr = !z.string().trim().min(1).max(120).safeParse(name).success;
  const unitErr = !z.string().trim().min(1).max(60).safeParse(unit).success;
  const blocked = (STEPS[step] === "name" && nameErr) || (STEPS[step] === "unit" && unitErr);

  const create = useMutation({
    mutationFn: async () => {
      const p = await api("projects", project, undefined, { method: "POST", body: JSON.stringify({ name: name.trim(), functional_unit: unit.trim() }) });
      if (teamId) await api(`projects/${p.id}/team`, z.unknown(), undefined, { method: "PUT", body: JSON.stringify({ team_id: teamId }) });
      return p;
    },
    onSuccess: (p) => {
      setCreated(p); setDir(1);
      void qc.invalidateQueries({ queryKey: ["projects"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); void qc.invalidateQueries({ queryKey: ["allocation"] });
    },
  });

  useEffect(() => { first.current?.focus(); }, [step]);
  const go = (n: number) => { setDir(n > step ? 1 : -1); setStep(n); };
  const next = () => { if (blocked) return; if (step === STEPS.length - 1) create.mutate(); else go(step + 1); };
  const key = created ? "done" : STEPS[step];
  const teamName = teams.find((x) => x.id === teamId)?.name;

  return (
    <div className="page-enter mx-auto w-full max-w-3xl">
      <div className="relative overflow-hidden rounded-3xl border border-line bg-white shadow-[0_20px_60px_-30px_rgba(47,143,120,0.35)]">
        <div aria-hidden className="pointer-events-none absolute -right-24 -top-24 size-72 rounded-full bg-leaf-soft blur-2xl" />
        <div aria-hidden className="pointer-events-none absolute -bottom-28 -left-16 size-64 rounded-full bg-[#eef3fd] blur-2xl" />

        <div className="relative px-6 pt-6 sm:px-10 sm:pt-8">
          <div className="flex items-center justify-between">
            <p className="text-sm font-semibold text-leaf-deep">{created ? t("wiz.done_tag") : t("wiz.of", { n: step + 1, total: STEPS.length })}</p>
            {!created && <button type="button" onClick={onClose} className="rounded-lg px-2 py-1 text-sm text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink">{t("common.cancel")}</button>}
          </div>
          <ol className="mt-4 flex gap-2" aria-hidden>
            {STEPS.map((s, i) => (
              <li key={s} className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-100">
                <span className="block h-full origin-left rounded-full bg-leaf transition-transform duration-500 ease-out" style={{ transform: `scaleX(${created || i < step ? 1 : i === step ? 0.5 : 0})` }} />
              </li>
            ))}
          </ol>
        </div>

        <div className="relative min-h-[360px] px-6 py-8 sm:px-10">
          <div key={key} className={dir > 0 ? "form-swap" : "form-swap-back"}>
            {key === "name" && (
              <div>
                <IconFolder className="size-14 text-leaf-deep" strokeWidth={1.3} />
                <h2 className="mt-4 text-2xl font-bold tracking-tight">{t("wiz.name.title")}</h2>
                <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("wiz.name.body")}</p>
                <input ref={first} value={name} onChange={(e) => setName(e.target.value)} onKeyDown={(e) => e.key === "Enter" && next()} maxLength={120} placeholder={t("wiz.name.placeholder")} className={cn(field, "mt-6")} aria-label={t("wiz.name.title")} />
                <div className="mt-3 flex flex-wrap gap-2">
                  {["production", "staging", "data-platform"].map((x) => (
                    <button key={x} type="button" onClick={() => setName(x)} className="rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600 transition-colors hover:bg-leaf-soft hover:text-leaf-deep">{x}</button>
                  ))}
                </div>
              </div>
            )}
            {key === "unit" && (
              <div>
                <IconGauge className="size-14 text-leaf-deep" strokeWidth={1.3} />
                <h2 className="mt-4 text-2xl font-bold tracking-tight">{t("wiz.unit.title")}</h2>
                <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("wiz.unit.body")}</p>
                <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3">
                  {UNITS.map((u, i) => (
                    <button key={u} type="button" onClick={() => setUnit(u)} aria-pressed={unit === u} style={{ "--i": i } as CSSProperties}
                      className={cn("rise rounded-2xl border px-4 py-3 text-left text-sm font-medium transition-all duration-200 hover:-translate-y-0.5", unit === u ? "border-leaf bg-leaf-soft text-leaf-deep shadow-[0_0_0_3px_rgba(111,195,168,0.18)]" : "border-line bg-white hover:border-leaf/50")}>
                      {u}
                    </button>
                  ))}
                </div>
                <input ref={first} value={unit} onChange={(e) => setUnit(e.target.value)} onKeyDown={(e) => e.key === "Enter" && next()} maxLength={60} className={cn(field, "mt-4")} aria-label={t("wiz.unit.custom")} placeholder={t("wiz.unit.custom")} />
              </div>
            )}
            {key === "team" && (
              <div>
                <IconUsers className="size-14 text-leaf-deep" strokeWidth={1.3} />
                <h2 className="mt-4 text-2xl font-bold tracking-tight">{t("wiz.team.title")}</h2>
                <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("wiz.team.body")}</p>
                <div className="mt-6 grid gap-3 sm:grid-cols-2">
                  {[{ id: null as string | null, name: t("wiz.team.none") }, ...teams].map((x, i) => (
                    <button key={x.id ?? "none"} type="button" onClick={() => setTeamId(x.id)} aria-pressed={teamId === x.id} style={{ "--i": i } as CSSProperties}
                      className={cn("rise flex items-center gap-3 rounded-2xl border px-4 py-3 text-left text-sm font-medium transition-all duration-200 hover:-translate-y-0.5", teamId === x.id ? "border-leaf bg-leaf-soft text-leaf-deep shadow-[0_0_0_3px_rgba(111,195,168,0.18)]" : "border-line bg-white hover:border-leaf/50")}>
                      <span className={cn("grid size-5 place-items-center rounded-full border transition-colors", teamId === x.id ? "border-leaf bg-leaf text-white" : "border-slate-300")}>{teamId === x.id && <IconCheck className="size-3" strokeWidth={3} />}</span>
                      {x.name}
                    </button>
                  ))}
                </div>
                {teams.length === 0 && <p className="mt-4 text-sm text-muted">{t("wiz.team.empty")}</p>}
              </div>
            )}
            {key === "review" && (
              <div>
                <IconLeaf className="size-14 text-leaf-deep" strokeWidth={1.3} />
                <h2 className="mt-4 text-2xl font-bold tracking-tight">{t("wiz.review.title")}</h2>
                <dl className="mt-6 divide-y divide-line/70 rounded-2xl border border-line bg-white/70 text-sm">
                  {[[t("wiz.review.name"), name.trim(), 0], [t("wiz.review.unit"), unit.trim(), 1], [t("wiz.review.team"), teamName ?? t("wiz.team.none"), 2]].map(([k, v, i]) => (
                    <div key={String(k)} className="flex items-center justify-between gap-4 px-4 py-3">
                      <dt className="text-muted">{k}</dt>
                      <dd className="flex items-center gap-3 font-semibold">{v}<button type="button" onClick={() => go(Number(i))} className="text-xs font-medium text-leaf-deep hover:underline">{t("wiz.review.edit")}</button></dd>
                    </div>
                  ))}
                </dl>
                {create.isError && <p role="alert" className="mt-4 text-sm text-bad">{create.error.message}</p>}
              </div>
            )}
            {key === "done" && created && (
              <div className="text-center">
                <span className="mx-auto grid size-20 place-items-center rounded-full bg-leaf-soft text-leaf-deep"><IconCheck className="pop size-10" strokeWidth={2.4} /></span>
                <h2 className="mt-5 text-2xl font-bold tracking-tight">{t("wiz.done.title", { name: created.name })}</h2>
                <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-slate-600">{t("wiz.done.body")}</p>
                <div className="mt-7 grid gap-3 text-left sm:grid-cols-3">
                  {[
                    { n: 1, label: t("wiz.done.connect"), href: "/connectors" },
                    { n: 2, label: t("wiz.done.regions"), action: () => onOpen(created.id) },
                    { n: 3, label: t("wiz.done.units"), action: () => onOpen(created.id) },
                  ].map((x, i) => {
                    const cls = "rise group flex h-full items-start gap-3 rounded-2xl border border-line bg-white p-4 text-sm font-medium transition-all hover:-translate-y-0.5 hover:border-leaf/50 hover:shadow-md";
                    const body = <><span className="grid size-6 shrink-0 place-items-center rounded-full bg-leaf-soft text-xs font-bold text-leaf-deep">{x.n}</span><span>{x.label}</span></>;
                    return x.href
                      ? <Link key={x.n} href={x.href} className={cls} style={{ "--i": i } as CSSProperties}>{body}</Link>
                      : <button key={x.n} type="button" onClick={x.action} className={cls} style={{ "--i": i } as CSSProperties}>{body}</button>;
                  })}
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="relative flex items-center justify-between border-t border-line/70 bg-slate-50/60 px-6 py-4 sm:px-10">
          {created ? (
            <>
              <span />
              <button type="button" onClick={onClose} className="inline-flex items-center gap-2 rounded-xl bg-leaf-deep px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition-all hover:-translate-y-0.5 hover:brightness-110">{t("wiz.done.close")}</button>
            </>
          ) : (
            <>
              <button type="button" onClick={() => go(step - 1)} disabled={step === 0}
                className="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-100 disabled:invisible">
                <IconArrowLeft className="size-4" />{t("wiz.back")}
              </button>
              <button type="button" onClick={next} disabled={blocked || create.isPending}
                className="inline-flex items-center gap-2 rounded-xl bg-leaf-deep px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition-all hover:-translate-y-0.5 hover:brightness-110 disabled:translate-y-0 disabled:opacity-50">
                {step === STEPS.length - 1 ? (create.isPending ? t("wiz.creating") : t("wiz.create")) : t("wiz.next")}
                {step < STEPS.length - 1 && <IconArrowRight className="size-4" />}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
