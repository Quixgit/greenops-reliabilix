"use client";

import { SelectField } from "@/components/ui/select-field";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { IconArrowLeft, IconArrowRight, IconCheck, IconCluster, IconCopy, IconKey, IconLock, IconShield } from "@/components/ui/icons";
import { useK8sClusters, useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { apiKeyCreated } from "@/lib/api/schemas";
import { BUILD_VERSION } from "@/lib/version";
import { cn } from "@/lib/utils";

const STEPS = ["cluster", "key", "install", "wait"] as const;
const field = "w-full rounded-xl border border-line bg-white px-3.5 py-2.5 text-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-300 focus:border-leaf focus:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]";
const NAME = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;
const REGION = /^[a-z0-9][a-z0-9-]{2,30}$/;

function Code({ title, text }: { title: string; text: string }) {
  const t = useT();
  const [done, setDone] = useState(false);
  return (
    <div>
      <div className="mb-1 flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</p>
        <button type="button" onClick={() => { void navigator.clipboard?.writeText(text); setDone(true); window.setTimeout(() => setDone(false), 1500); }}
          className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink">
          {done ? <IconCheck className="size-3.5 text-leaf-deep" strokeWidth={2.6} /> : <IconCopy className="size-3.5" />}{done ? t("au.copied") : t("au.copy")}
        </button>
      </div>
      <pre className="overflow-x-auto whitespace-pre rounded-xl bg-slate-900 px-4 py-3 font-mono text-xs leading-relaxed text-slate-100">{text}</pre>
    </div>
  );
}

/** Connect a cluster: the platform never reaches into it, so the person gets a key and a ready manifest for the agent. */
export function ConnectCluster({ onClose }: { onClose: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const projects = useProjects().data ?? [];
  const clusters = useK8sClusters();
  const [step, setStep] = useState(0);
  const [dir, setDir] = useState(1);
  const [name, setName] = useState("");
  const [project, setProject] = useState("");
  const [region, setRegion] = useState("");
  const [apiUrl, setApiUrl] = useState("");
  const [key, setKey] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => { if (projects.length === 1 && !project) setProject(projects[0].id); }, [projects, project]);
  const nameOk = NAME.test(name), regionOk = REGION.test(region), step0 = nameOk && regionOk && !!project;

  const create = useMutation({
    mutationFn: () => api("api-keys", apiKeyCreated, undefined, { method: "POST", body: JSON.stringify({ name: `k8s-${name}`, role: "agent" }) }),
    onSuccess: (r) => { setKey(r.key); void qc.invalidateQueries({ queryKey: ["api-keys"] }); },
  });
  const connected = (clusters.data ?? []).some((c) => c.cluster === name);
  useEffect(() => {
    if (step !== 3) return;
    const id = window.setInterval(() => void clusters.refetch(), 5000);
    return () => window.clearInterval(id);
  }, [step, clusters]);

  const go = (n: number) => { setDir(n > step ? 1 : -1); setStep(n); };
  const url = apiUrl.trim() || "https://<your Reliabilix API address>";
  const secretCmd = `kubectl -n monitoring create secret generic reliabilix-agent \\\n  --from-literal=API_KEY='${key || "<key>"}' \\\n  --from-literal=API_URL='${url}' \\\n  --from-literal=PROJECT_ID='${project}' \\\n  --from-literal=CLUSTER='${name}' \\\n  --from-literal=REGION='${region}' \\\n  --from-literal=PROM_URL='http://prometheus.monitoring.svc:9090'`;
  const cronjob = `apiVersion: batch/v1\nkind: CronJob\nmetadata: {name: reliabilix-energy, namespace: monitoring}\nspec:\n  schedule: "7 * * * *"\n  concurrencyPolicy: Forbid\n  jobTemplate:\n    spec:\n      template:\n        spec:\n          restartPolicy: OnFailure\n          containers:\n            - name: push\n              image: ghcr.io/quixgit/reliabilix/agent:${BUILD_VERSION.full}\n              envFrom: [{secretRef: {name: reliabilix-agent}}]`;
  const stepKey = connected && step === 3 ? "done" : STEPS[step];

  return (
    <div className="glass overflow-hidden">
      <div className="px-6 pt-5 sm:px-8">
        <div className="flex items-center justify-between gap-4">
          <ol className="flex flex-1 items-center gap-2 text-[13px]">
            {STEPS.map((s, i) => {
              const on = i === step && stepKey !== "done", done = i < step || stepKey === "done";
              return (
                <li key={s} className="flex flex-1 items-center gap-2">
                  <span className={cn("grid size-6 shrink-0 place-items-center rounded-full text-[11px] font-bold transition-colors duration-300", done ? "bg-leaf text-white" : on ? "bg-leaf-soft text-leaf-deep ring-2 ring-leaf" : "bg-slate-100 text-slate-400")}>
                    {done ? <IconCheck className="size-3.5" strokeWidth={3} /> : i + 1}
                  </span>
                  <span className={cn("hidden whitespace-nowrap font-medium sm:inline", on ? "text-ink" : "text-slate-400")}>{t(`kc.step.${s}`)}</span>
                  {i < STEPS.length - 1 && <span className="h-0.5 flex-1 overflow-hidden rounded-full bg-slate-100"><span className="block h-full origin-left bg-leaf transition-transform duration-500" style={{ transform: `scaleX(${done ? 1 : 0})` }} /></span>}
                </li>
              );
            })}
          </ol>
          <button type="button" onClick={onClose} className="shrink-0 rounded-lg px-2 py-1 text-sm text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink">{stepKey === "done" ? t("common.close") : t("common.cancel")}</button>
        </div>
      </div>

      <div className="min-h-[300px] px-6 py-6 sm:px-8">
        <div key={stepKey} className={dir > 0 ? "form-swap" : "form-swap-back"}>
          {stepKey === "cluster" && (
            <div>
              <IconCluster className="size-12 text-[#5b8fdc]" strokeWidth={1.3} />
              <h2 className="mt-3 text-2xl font-bold tracking-tight">{t("kc.cluster.title")}</h2>
              <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("kc.cluster.body")}</p>
              <div className="mt-5 grid gap-4 sm:grid-cols-2">
                <label className="block text-xs font-medium text-slate-500">{t("kc.f.name")}
                  <input value={name} onChange={(e) => setName(e.target.value.toLowerCase())} className={cn(field, "mt-1")} placeholder="prod-eu-1" maxLength={63} autoFocus />
                  {name && !nameOk && <span role="alert" className="mt-1 block text-xs text-bad">{t("kc.err.name")}</span>}
                </label>
                <label className="block text-xs font-medium text-slate-500">{t("kc.f.project")}
                  <SelectField value={project} onChange={(e) => setProject(e.target.value)} className={cn(field, "mt-1")}>
                    <option value="" disabled>{t("cw.f.project_pick")}</option>{projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
                  </SelectField>
                </label>
                <label className="block text-xs font-medium text-slate-500">{t("kc.f.region")}
                  <input value={region} onChange={(e) => setRegion(e.target.value.trim())} className={cn(field, "mt-1")} placeholder="eu-central-1 · westeurope · europe-west4" />
                  {region && !regionOk && <span role="alert" className="mt-1 block text-xs text-bad">{t("kc.err.region")}</span>}
                </label>
                <label className="block text-xs font-medium text-slate-500">{t("kc.f.api")}
                  <input value={apiUrl} onChange={(e) => setApiUrl(e.target.value)} className={cn(field, "mt-1")} placeholder="https://api.example.com" inputMode="url" />
                </label>
              </div>
              <p className="mt-4 flex items-start gap-2 text-xs leading-relaxed text-muted"><IconShield className="mt-0.5 size-4 shrink-0 text-leaf-deep" />{t("kc.push_note")}</p>
            </div>
          )}

          {stepKey === "key" && (
            <div>
              <IconKey className="size-12 text-leaf-deep" strokeWidth={1.3} />
              <h2 className="mt-3 text-2xl font-bold tracking-tight">{t("kc.key.title")}</h2>
              <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("kc.key.body")}</p>
              {!key ? (
                <div className="mt-6">
                  <button type="button" disabled={create.isPending} onClick={() => create.mutate()} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold">
                    <IconKey className="size-4" strokeWidth={2} />{create.isPending ? t("kc.key.creating") : t("kc.key.create", { name })}
                  </button>
                  {create.isError && <p role="alert" className="mt-3 text-sm text-bad">{create.error.message}</p>}
                </div>
              ) : (
                <div className="mt-6 space-y-3">
                  <Code title={t("kc.key.once")} text={key} />
                  <label className="flex cursor-pointer items-center gap-2.5 text-sm text-slate-600">
                    <input type="checkbox" className="peer sr-only" checked={saved} onChange={(e) => setSaved(e.target.checked)} />
                    <span aria-hidden className="grid size-[18px] place-items-center rounded-md border border-slate-300 bg-white transition-all peer-checked:border-leaf peer-checked:bg-leaf">
                      <IconCheck className={cn("size-3 text-white transition-all", saved ? "scale-100 opacity-100" : "scale-50 opacity-0")} strokeWidth={3} />
                    </span>
                    {t("kc.key.saved")}
                  </label>
                  <p className="flex items-start gap-2 text-xs text-muted"><IconLock className="mt-0.5 size-4 shrink-0 text-leaf-deep" />{t("kc.key.scope")}</p>
                </div>
              )}
            </div>
          )}

          {stepKey === "install" && (
            <div className="space-y-4">
              <h2 className="text-2xl font-bold tracking-tight">{t("kc.install.title")}</h2>
              <p className="max-w-2xl text-sm leading-relaxed text-slate-600">{t("kc.install.body")}</p>
              <Code title={t("kc.install.secret")} text={secretCmd} />
              <Code title={t("kc.install.cron")} text={cronjob} />
              <p className="text-xs text-muted">{t("kc.install.req")}</p>
            </div>
          )}

          {stepKey === "wait" && (
            <div className="py-6 text-center">
              <span className="relative mx-auto grid size-20 place-items-center">
                <span className="absolute inset-0 animate-ping rounded-full bg-leaf-soft" />
                <span className="relative grid size-16 place-items-center rounded-full bg-leaf-soft"><IconCluster className="size-9 text-leaf-deep" strokeWidth={1.4} /></span>
              </span>
              <h2 className="mt-5 text-2xl font-bold tracking-tight">{t("kc.wait.title")}</h2>
              <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-slate-600">{t("kc.wait.body", { name })}</p>
            </div>
          )}

          {stepKey === "done" && (
            <div className="py-4 text-center">
              <span className="mx-auto grid size-20 place-items-center rounded-full bg-leaf-soft text-leaf-deep"><IconCheck className="pop size-10" strokeWidth={2.4} /></span>
              <h2 className="mt-5 text-2xl font-bold tracking-tight">{t("kc.done.title", { name })}</h2>
              <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-slate-600">{t("kc.done.body")}</p>
            </div>
          )}
        </div>
      </div>

      <div className="flex items-center justify-between border-t border-line/70 bg-slate-50/60 px-6 py-3.5 sm:px-8">
        {stepKey === "done" ? (
          <><span /><button type="button" onClick={onClose} className="btn-leaf rounded-xl px-5 py-2.5 text-sm font-semibold">{t("kc.done.close")}</button></>
        ) : (
          <>
            <button type="button" onClick={() => go(step - 1)} disabled={step === 0 || (step === 1 && !!key)}
              className="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-100 disabled:invisible"><IconArrowLeft className="size-4" />{t("wiz.back")}</button>
            {step === 3 ? <button type="button" onClick={onClose} className="rounded-xl px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100">{t("kc.wait.later")}</button>
              : <button type="button" disabled={(step === 0 && !step0) || (step === 1 && !(key && saved))} onClick={() => go(step + 1)}
                  className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold">{t("wiz.next")}<IconArrowRight className="size-4" /></button>}
          </>
        )}
      </div>
    </div>
  );
}
