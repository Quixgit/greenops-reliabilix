"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type CSSProperties } from "react";
import { useForm } from "react-hook-form";
import { ProviderMark } from "@/components/connectors/provider-mark";
import { IconArrowLeft, IconArrowRight, IconCheck, IconCopy, IconLock, IconShield, IconKey } from "@/components/ui/icons";
import { useProjects } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { api } from "@/lib/api/client";
import { connection, connectResponse, type ConnectResponse } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { SetupAsCode } from "./setup-as-code";
import { EMPTY, schemaFor, toRequest, verifyHint, type Mode, type Provider, type Values } from "./connect-model";

const STEPS = ["cloud", "method", "details", "verify"] as const;
const PROVIDERS: { id: Provider; name: string }[] = [{ id: "aws", name: "Amazon Web Services" }, { id: "azure", name: "Microsoft Azure" }, { id: "gcp", name: "Google Cloud" }];
const field = "w-full rounded-xl border border-line bg-white px-3.5 py-2.5 text-sm outline-none transition-all placeholder:text-slate-400 hover:border-slate-300 focus:border-leaf focus:shadow-[0_0_0_3px_rgba(111,195,168,0.22)]";

function Copy({ text }: { text: string }) {
  const t = useT();
  const [done, setDone] = useState(false);
  return (
    <button type="button" onClick={() => { void navigator.clipboard?.writeText(text); setDone(true); window.setTimeout(() => setDone(false), 1500); }}
      className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink">
      {done ? <IconCheck className="size-3.5 text-leaf-deep" strokeWidth={2.6} /> : <IconCopy className="size-3.5" />}{done ? t("au.copied") : t("au.copy")}
    </button>
  );
}
function Code({ title, text }: { title: string; text: string }) {
  return (
    <div>
      <div className="mb-1 flex items-center justify-between"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</p><Copy text={text} /></div>
      <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-xl bg-slate-900 px-4 py-3 font-mono text-xs leading-relaxed text-slate-100">{text}</pre>
    </div>
  );
}

/** What the person must set up on the cloud side, depending on the provider and the way chosen. */
function Setup({ created }: { created: ConnectResponse }) {
  const t = useT();
  const { setup, connection: c } = created;
  const steps = (setup.steps ?? []);
  const azureSp = c.provider === "azure" && (c.credential_ref ?? "").startsWith("azcost://");
  const awsKey = c.provider === "aws" && (c.credential_ref ?? "").startsWith("awskey://");
  const gcpKey = c.provider === "gcp" && c.access === "service_account_key";
  const policy = JSON.stringify(setup.permissions_policy, null, 2);
  return (
    <div className="space-y-4 text-sm">
      <p className="leading-relaxed text-slate-600">
        {azureSp ? t("cw.setup.azure_sp", { id: c.account_ref }) : awsKey ? t("cw.setup.aws_key", { id: c.account_ref }) : gcpKey ? t("cw.setup.gcp_key", { id: c.account_ref })
          : c.provider === "azure" ? t("cw.setup.azure_export", { id: c.account_ref }) : c.provider === "gcp" ? t("cw.setup.gcp_share", { id: c.account_ref }) : t("cw.setup.aws_role", { id: c.account_ref })}
      </p>
      {steps.length > 0 && (
        <ol className="space-y-2">
          {steps.map((s, i) => (
            <li key={i} className="flex gap-3"><span className="grid size-6 shrink-0 place-items-center rounded-full bg-leaf-soft text-xs font-bold text-leaf-deep">{i + 1}</span><span className="break-words pt-0.5 text-slate-700">{s}</span></li>
          ))}
        </ol>
      )}
      {setup.platform_account_id && !azureSp && !awsKey && !gcpKey && <Code title={t("cw.setup.platform_account")} text={setup.platform_account_id} />}
      {!azureSp && !awsKey && !gcpKey && setup.external_id && (
        <Code title={c.provider === "aws" ? "External ID" : c.provider === "azure" ? t("cw.setup.token_azure") : t("cw.setup.token_gcp")} text={c.provider === "azure" ? setup.external_id.replaceAll("-", "_") : setup.external_id} />
      )}
      {setup.trust_policy && !awsKey && <Code title={t("cw.setup.trust")} text={JSON.stringify(setup.trust_policy, null, 2)} />}
      {(c.provider === "aws" && !(azureSp || gcpKey)) && <Code title={t("cw.setup.permissions")} text={policy} />}
    </div>
  );
}

/** Connect a cloud account one question at a time, in place; ends with the cloud-side steps and a check that access works. */
export function ConnectWizard({ initialProvider, onClose }: { initialProvider?: Provider | null; onClose: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const projects = useProjects().data ?? [];
  const [step, setStep] = useState(initialProvider ? 1 : 0);
  const [dir, setDir] = useState(1);
  const [created, setCreated] = useState<ConnectResponse | null>(null);
  const [verified, setVerified] = useState(false);

  const form = useForm<Values>({
    resolver: (values, ctx, opts) => zodResolver(schemaFor(values))(values as never, ctx, opts as never) as never,
    defaultValues: { provider: initialProvider ?? "aws", ...EMPTY },
  });
  const { register, handleSubmit, watch, setValue, formState: { errors } } = form;
  const provider = watch("provider");
  const mode = watch("mode");
  const fieldError = (name: string) => (errors as Record<string, { message?: string }>)[name]?.message;
  useEffect(() => { if (projects.length === 1 && !form.getValues("project_id")) setValue("project_id", projects[0].id); }, [projects, form, setValue]);

  const create = useMutation({
    mutationFn: (v: unknown) => api("cloud-accounts", connectResponse, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => { setCreated(r); setDir(1); setStep(3); void qc.invalidateQueries({ queryKey: ["connections"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  const verify = useMutation({
    mutationFn: (id: string) => api(`cloud-accounts/${id}/verify`, connection, undefined, { method: "POST" }),
    onSuccess: () => { setVerified(true); void qc.invalidateQueries({ queryKey: ["connections"] }); },
  });
  const go = (n: number) => { setDir(n > step ? 1 : -1); setStep(n); };
  const name = PROVIDERS.find((p) => p.id === provider)?.name ?? provider;
  const key = verified ? "done" : STEPS[step];

  const method = (m: Mode, icon: typeof IconKey, title: string, body: string, tag?: string, i = 0) => (
    <button key={m} type="button" onClick={() => setValue("mode", m)} aria-pressed={mode === m} style={{ "--i": i } as CSSProperties}
      className={cn("rise flex items-start gap-4 rounded-2xl border p-4 text-left transition-all duration-300 hover:-translate-y-0.5",
        mode === m ? "border-leaf bg-leaf-soft/60 shadow-[0_0_0_3px_rgba(111,195,168,0.18)]" : "border-line bg-white hover:border-leaf/50")}>
      {(() => { const I = icon; return <I className="size-10 shrink-0 text-leaf-deep" strokeWidth={1.4} />; })()}
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-2 text-sm font-semibold text-ink">{title}{tag && <span className="rounded-full bg-leaf px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide text-white">{tag}</span>}</span>
        <span className="mt-1 block text-xs leading-relaxed text-muted">{body}</span>
      </span>
    </button>
  );

  return (
    <div className="glass overflow-hidden">
      <div className="px-6 pt-5 sm:px-8">
        <div className="flex items-center justify-between gap-4">
          <ol className="flex flex-1 items-center gap-2 text-[13px]">
            {STEPS.map((s, i) => {
              const on = !verified && i === step, done = verified || i < step;
              return (
                <li key={s} className="flex flex-1 items-center gap-2">
                  <span className={cn("grid size-6 shrink-0 place-items-center rounded-full text-[11px] font-bold transition-colors duration-300", done ? "bg-leaf text-white" : on ? "bg-leaf-soft text-leaf-deep ring-2 ring-leaf" : "bg-slate-100 text-slate-400")}>
                    {done ? <IconCheck className="size-3.5" strokeWidth={3} /> : i + 1}
                  </span>
                  <span className={cn("hidden whitespace-nowrap font-medium sm:inline", on ? "text-ink" : "text-slate-400")}>{t(`cw.step.${s}`)}</span>
                  {i < STEPS.length - 1 && <span className="h-0.5 flex-1 overflow-hidden rounded-full bg-slate-100"><span className="block h-full origin-left bg-leaf transition-transform duration-500" style={{ transform: `scaleX(${done ? 1 : 0})` }} /></span>}
                </li>
              );
            })}
          </ol>
          <button type="button" onClick={onClose} className="shrink-0 rounded-lg px-2 py-1 text-sm text-slate-500 transition-colors hover:bg-slate-100 hover:text-ink">{verified ? t("common.close", undefined, "Close") : t("common.cancel")}</button>
        </div>
      </div>

      <div className="min-h-[300px] px-6 py-6 sm:px-8">
        <div key={key} className={dir > 0 ? "form-swap" : "form-swap-back"}>
          {key === "cloud" && (
            <div>
              <h2 className="text-2xl font-bold tracking-tight">{t("cw.cloud.title")}</h2>
              <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("cw.cloud.body")}</p>
              <div className="mt-6 grid gap-3 sm:grid-cols-3">
                {PROVIDERS.map((p, i) => (
                  <button key={p.id} type="button" onClick={() => { setValue("provider", p.id); setValue("mode", "own"); go(1); }} style={{ "--i": i } as CSSProperties}
                    className={cn("rise group flex flex-col items-center gap-3 rounded-2xl border bg-white p-6 text-center transition-all duration-300 hover:-translate-y-1 hover:border-leaf/60 hover:shadow-[0_14px_32px_-14px_rgba(47,143,120,0.35)]", provider === p.id ? "border-leaf" : "border-line")}>
                    <span className="transition-transform duration-500 group-hover:scale-110 group-hover:-rotate-6"><ProviderMark id={p.id} size="lg" /></span>
                    <span className="text-sm font-semibold text-ink">{p.name}</span>
                  </button>
                ))}
              </div>
              <p className="mt-5 flex items-center gap-2 text-xs text-muted"><IconShield className="size-4 text-leaf-deep" />{t("cw.readonly")}</p>
            </div>
          )}

          {key === "method" && (
            <div>
              <div className="flex items-center gap-3"><ProviderMark id={provider} size="lg" /><h2 className="text-2xl font-bold tracking-tight">{t("cw.method.title", { name })}</h2></div>
              <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("cw.method.body")}</p>
              <div className="mt-6 grid gap-3">
                {method("own", IconKey, t(`cw.method.own.${provider}`), t(`cw.method.own.${provider}_body`), t("cw.recommended"), 0)}
                {method("other", IconLock, t(`cw.method.other.${provider}`), t(`cw.method.other.${provider}_body`), undefined, 1)}
              </div>
            </div>
          )}

          {key === "details" && (
            <form id="cw-form" onSubmit={handleSubmit((v) => create.mutate(toRequest(v)))} noValidate>
              <div className="flex items-center gap-3"><ProviderMark id={provider} size="lg" /><h2 className="text-2xl font-bold tracking-tight">{t("cw.details.title")}</h2></div>
              <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-600">{t("cw.details.body")}</p>
              {mode !== "other" && <SetupAsCode provider={provider} account={watch("account_ref")} table={watch("credential_ref")} />}
              <div className="mt-5 grid gap-4 sm:grid-cols-2">
                <label className="block text-xs font-medium text-slate-500">{t("cw.f.project")}
                  <select {...register("project_id")} defaultValue="" className={cn(field, "mt-1")}>
                    <option value="" disabled>{t("cw.f.project_pick")}</option>
                    {projects.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
                  </select>
                  {errors.project_id && <span role="alert" className="mt-1 block text-xs text-bad">{errors.project_id.message}</span>}
                </label>
                {provider === "aws" && (<>
                  <Field label={t("cw.f.aws_account")} error={fieldError("account_ref")}><input {...register("account_ref")} className={field} placeholder="123456789012" inputMode="numeric" /></Field>
                  {mode === "other" ? (
                    <Field wide label={t("cw.f.aws_role")} error={fieldError("credential_ref")}><input {...register("credential_ref")} className={field} placeholder="arn:aws:iam::123456789012:role/ReliabilixReadOnly" /></Field>
                  ) : (<>
                    <Field label={t("cw.f.aws_key")} error={fieldError("access_key_id")}><input {...register("access_key_id")} className={field} placeholder="AKIA…" autoComplete="off" spellCheck={false} /></Field>
                    <Field label={t("cw.f.aws_secret")} error={fieldError("client_secret")}><input {...register("client_secret")} type="password" autoComplete="new-password" spellCheck={false} className={field} /></Field>
                  </>)}
                </>)}
                {provider === "azure" && (<>
                  <Field label={t("cw.f.az_sub")} error={fieldError("account_ref")}><input {...register("account_ref")} className={field} placeholder="00000000-0000-0000-0000-000000000000" /></Field>
                  {mode === "other" ? (
                    <Field wide label={t("cw.f.az_export")} error={fieldError("credential_ref")}><input {...register("credential_ref")} className={field} placeholder="azblob://<tenant id>@account/container/exports/daily" /></Field>
                  ) : (<>
                    <Field label={t("cw.f.az_tenant")} error={fieldError("tenant_id")}><input {...register("tenant_id")} className={field} autoComplete="off" /></Field>
                    <Field label={t("cw.f.az_client")} error={fieldError("client_id")}><input {...register("client_id")} className={field} autoComplete="off" /></Field>
                    <Field label={t("cw.f.az_secret")} error={fieldError("client_secret")}><input {...register("client_secret")} type="password" autoComplete="new-password" spellCheck={false} className={field} /></Field>
                  </>)}
                </>)}
                {provider === "gcp" && (<>
                  <Field label={t("cw.f.gcp_project")} error={fieldError("account_ref")}><input {...register("account_ref")} className={field} placeholder="acme-prod-123" /></Field>
                  <Field wide label={t("cw.f.gcp_table")} error={fieldError("credential_ref")}><input {...register("credential_ref")} className={field} placeholder="bq://acme-billing/billing_export/gcp_billing_export_v1_XXXXXX" /></Field>
                  {mode !== "other" && <Field wide label={t("cw.f.gcp_key")} error={fieldError("client_secret")}><textarea {...register("client_secret")} rows={4} autoComplete="off" spellCheck={false} className={cn(field, "font-mono text-xs")} placeholder='{"type": "service_account", …}' /></Field>}
                </>)}
              </div>
              <p className="mt-4 flex items-start gap-2 text-xs leading-relaxed text-muted"><IconLock className="mt-0.5 size-4 shrink-0 text-leaf-deep" />{mode === "other" ? t("cw.sealed_other") : t("cw.sealed")}</p>
              {create.isError && <p role="alert" className="mt-3 text-sm text-bad">{create.error.message}</p>}
            </form>
          )}

          {key === "verify" && created && (
            <div>
              <div className="flex items-center gap-3"><ProviderMark id={provider} size="lg" /><h2 className="text-2xl font-bold tracking-tight">{t("cw.verify.title")}</h2></div>
              <div className="mt-4"><Setup created={created} /></div>
              {verify.isError && <p role="alert" className="mt-4 rounded-xl bg-[#f8eceb] px-4 py-3 text-sm text-[#a8504b]">{verify.error.message} — {t("cw.verify.retry")}{verifyHint(verify.error.message) && <span className="mt-1.5 block font-medium text-slate-600">{t(verifyHint(verify.error.message)!)}</span>}</p>}
            </div>
          )}

          {key === "done" && (
            <div className="text-center">
              <span className="mx-auto grid size-20 place-items-center rounded-full bg-leaf-soft text-leaf-deep"><IconCheck className="pop size-10" strokeWidth={2.4} /></span>
              <h2 className="mt-5 text-2xl font-bold tracking-tight">{t("cw.done.title", { name })}</h2>
              <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-slate-600">{t("cw.done.body")}</p>
            </div>
          )}
        </div>
      </div>

      <div className="flex items-center justify-between border-t border-line/70 bg-slate-50/60 px-6 py-3.5 sm:px-8">
        {verified ? (
          <><span /><button type="button" onClick={onClose} className="btn-leaf rounded-xl px-5 py-2.5 text-sm font-semibold">{t("cw.done.close")}</button></>
        ) : (
          <>
            <button type="button" onClick={() => go(step - 1)} disabled={step === 0 || (step === 3)}
              className="inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-100 disabled:invisible"><IconArrowLeft className="size-4" />{t("wiz.back")}</button>
            {step === 0 && <span className="text-xs text-muted">{t("cw.pick")}</span>}
            {step === 1 && <button type="button" onClick={() => go(2)} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold">{t("wiz.next")}<IconArrowRight className="size-4" /></button>}
            {step === 2 && <button type="submit" form="cw-form" disabled={create.isPending} className="btn-leaf inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold">{create.isPending ? t("cw.registering") : t("wiz.next")}{!create.isPending && <IconArrowRight className="size-4" />}</button>}
            {step === 3 && created && <button type="button" disabled={verify.isPending} onClick={() => verify.mutate(created.connection.id)} className="btn-leaf rounded-xl px-5 py-2.5 text-sm font-semibold">{verify.isPending ? t("cw.verifying") : t("cw.verify.do")}</button>}
          </>
        )}
      </div>
    </div>
  );
}

function Field({ label, error, wide, children }: { label: string; error?: string; wide?: boolean; children: React.ReactNode }) {
  return (
    <label className={cn("block text-xs font-medium text-slate-500", wide && "sm:col-span-2")}>{label}
      <span className="mt-1 block">{children}</span>
      {error && <span role="alert" className="mt-1 block text-xs text-bad">{error}</span>}
    </label>
  );
}
