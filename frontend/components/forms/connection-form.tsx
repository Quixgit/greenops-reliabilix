"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { connection, connectResponse, type ConnectResponse } from "@/lib/api/schemas";
import { useProjects } from "@/hooks/use-overview";
import { Field, input } from "./project-form";

const aws = z.object({
  project_id: z.string().min(1, "Choose a project"),
  provider: z.literal("aws"),
  account_ref: z.string().trim().regex(/^\d{12}$/, "AWS account id: 12 digits"),
  credential_ref: z.string().trim().regex(/^arn:aws:iam::\d{12}:role\/Reliabilix[\w+=,.@-]{0,100}$/, "Enter the IAM role ARN; the role name must start with \"Reliabilix\" (not an access key)"),
});
const gcp = z.object({
  project_id: z.string().min(1, "Choose a project"),
  provider: z.literal("gcp"),
  account_ref: z.string().trim().regex(/^[a-z][a-z0-9-]{4,28}[a-z0-9]$/, "GCP project id: 6-30 lowercase letters, digits or hyphens"),
  // The export table the customer enabled in Cloud Billing: bq://<project>/<dataset>/<table>
  credential_ref: z.string().trim().regex(/^bq:\/\/[a-z][a-z0-9-]{4,28}[a-z0-9]\/[A-Za-z0-9_]{1,1024}\/gcp_billing_export_(resource_)?v1_[A-Za-z0-9_]+$/, "Use bq://<project>/<dataset>/gcp_billing_export_v1_…"),
});
type Provider = "aws" | "gcp";

/** Step 1: register the connection and show what the customer must set up. Step 2: verify once it is in place. */
export function ConnectionForm({ initialProvider = "aws", onDone }: { initialProvider?: Provider; onDone?: () => void }) {
  const qc = useQueryClient();
  const projects = useProjects();
  const [created, setCreated] = useState<ConnectResponse | null>(null);
  type Values = { project_id: string; provider: Provider; account_ref: string; credential_ref: string };
  // The rules depend on the provider that is selected, so the resolver picks the schema per submit.
  const { register, handleSubmit, reset, watch, formState: { errors } } = useForm<Values>({
    resolver: (values, ctx, opts) => zodResolver(values.provider === "gcp" ? gcp : aws)(values as never, ctx, opts as never) as never,
    defaultValues: { provider: initialProvider, project_id: "", account_ref: "", credential_ref: "" },
  });
  const provider = watch("provider");

  const create = useMutation({
    mutationFn: (v: unknown) => api("cloud-accounts", connectResponse, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => { setCreated(r); reset({ provider: initialProvider, project_id: "", account_ref: "", credential_ref: "" }); void qc.invalidateQueries({ queryKey: ["connections"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  const verify = useMutation({
    mutationFn: (id: string) => api(`cloud-accounts/${id}/verify`, connection, undefined, { method: "POST" }),
    onSuccess: () => { setCreated(null); onDone?.(); void qc.invalidateQueries({ queryKey: ["connections"] }); },
  });

  if (created) {
    const { setup, connection: c } = created;
    const isGcp = c.provider === "gcp";
    return (
      <div className="space-y-4 text-sm">
        {isGcp ? (
          <>
            <p>To read the billing export of project <b>{c.account_ref}</b>, do these steps:</p>
            <ol className="list-decimal space-y-2 pl-5">{(setup.steps ?? []).map((s, i) => <li key={i} className="break-words">{s}</li>)}</ol>
            {setup.platform_account_id && <p className="text-muted">Give this service account read access to the dataset: <code className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs">{setup.platform_account_id}</code></p>}
            <p className="text-muted">Ownership label (key = your connection token, value = 1)</p>
            <code className="block break-all rounded-lg bg-slate-100 px-3 py-2 font-mono text-xs">{setup.external_id}</code>
          </>
        ) : (
          <>
            <p>Create an IAM role in AWS account <b>{c.account_ref}</b> that trusts Reliabilix, using this <b>External ID</b>:</p>
            <code className="block rounded-lg bg-slate-100 px-3 py-2 font-mono text-xs">{setup.external_id}</code>
            {setup.trust_policy && (<><p className="text-muted">Trust policy</p><pre className="overflow-x-auto rounded-lg bg-slate-100 p-3 text-xs">{JSON.stringify(setup.trust_policy, null, 2)}</pre></>)}
            <p className="text-muted">Permissions policy (read-only billing access)</p>
            <pre className="overflow-x-auto rounded-lg bg-slate-100 p-3 text-xs">{JSON.stringify(setup.permissions_policy, null, 2)}</pre>
          </>
        )}
        <button onClick={() => verify.mutate(c.id)} disabled={verify.isPending} className="brand-gradient rounded-xl px-4 py-2.5 font-semibold text-white disabled:opacity-60">
          {verify.isPending ? "Verifying…" : "Verify access"}
        </button>
        {verify.isError && <p role="alert" className="text-bad">{verify.error.message} — check the steps above, then try again.</p>}
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit((v) => create.mutate(v))} className="grid gap-3 sm:grid-cols-2" noValidate>
      <Field label="Provider">
        <select {...register("provider")} className={input}>
          <option value="aws">AWS</option><option value="gcp">Google Cloud</option><option disabled>Azure (coming soon)</option>
        </select>
      </Field>
      <Field label="Project" error={errors.project_id?.message}>
        <select {...register("project_id")} className={input} defaultValue="">
          <option value="" disabled>Select project…</option>
          {projects.data?.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
      </Field>
      {provider === "aws" ? (
        <>
          <Field label="AWS account id" error={errors.account_ref?.message}><input {...register("account_ref")} className={input} placeholder="123456789012" inputMode="numeric" /></Field>
          <Field label="IAM role ARN (read-only)" error={errors.credential_ref?.message}><input {...register("credential_ref")} className={input} placeholder="arn:aws:iam::123456789012:role/ReliabilixReadOnly" /></Field>
          <p className="text-xs text-muted sm:col-span-2">Reliabilix assumes this role with an External ID. We never ask for access keys.</p>
        </>
      ) : (
        <>
          <Field label="GCP project id" error={errors.account_ref?.message}><input {...register("account_ref")} className={input} placeholder="acme-prod-123" /></Field>
          <Field label="Billing export table" error={errors.credential_ref?.message}><input {...register("credential_ref")} className={input} placeholder="bq://acme-billing/billing_export/gcp_billing_export_v1_XXXXXX_XXXXXX_XXXXXX" /></Field>
          <p className="text-xs text-muted sm:col-span-2">Reliabilix reads the Cloud Billing export in BigQuery with its own service account, read-only. We never ask for a key file.</p>
        </>
      )}
      <div className="sm:col-span-2">
        <button disabled={create.isPending} className="brand-gradient rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{create.isPending ? "Registering…" : "Continue"}</button>
        {create.isError && <p role="alert" className="mt-2 text-sm text-bad">{create.error.message}</p>}
      </div>
    </form>
  );
}
