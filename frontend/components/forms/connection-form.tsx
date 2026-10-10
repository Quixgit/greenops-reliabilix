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
const azure = z.object({
  project_id: z.string().min(1, "Choose a project"),
  provider: z.literal("azure"),
  account_ref: z.string().trim().regex(/^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$/, "Azure subscription id: a GUID"),
  // The Cost Management export folder: azblob://<tenant id>@<storage account>/<container>/<export directory>
  credential_ref: z.string().trim().regex(/^azblob:\/\/[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}@[a-z0-9]{3,24}\/[a-z0-9-]{3,63}\/[\w.-]+(\/[\w.-]+)*$/, "Use azblob://<tenant id>@<storage account>/<container>/<export directory>"),
});
// Azure, the usual way: the customer's own service principal (Cost Management Reader). The secret is sent once, sealed
// by the platform and never shown again.
const GUID = /^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$/;
const azureSp = z.object({
  project_id: z.string().min(1, "Choose a project"),
  provider: z.literal("azure"),
  account_ref: z.string().trim().regex(GUID, "Azure subscription id: a GUID"),
  tenant_id: z.string().trim().regex(GUID, "Directory (tenant) id: a GUID"),
  client_id: z.string().trim().regex(GUID, "Application (client) id: a GUID"),
  client_secret: z.string().trim().regex(/^[\x21-\x7e]{8,256}$/, "The client secret value (not the secret id)"),
});
type Provider = "aws" | "gcp" | "azure";

/** What the API takes: the Azure service principal is a reference plus a write-only secret. */
function toRequest(v: { project_id: string; provider: Provider; account_ref: string; credential_ref: string; mode: string; tenant_id: string; client_id: string; client_secret: string }) {
  if (v.provider === "azure" && v.mode !== "export") {
    return { project_id: v.project_id, provider: "azure", account_ref: v.account_ref.trim(),
      credential_ref: `azcost://${v.tenant_id.trim().toLowerCase()}@${v.client_id.trim().toLowerCase()}`, client_secret: v.client_secret.trim() };
  }
  return { project_id: v.project_id, provider: v.provider, account_ref: v.account_ref, credential_ref: v.credential_ref };
}

/** Step 1: register the connection and show what the customer must set up. Step 2: verify once it is in place. */
export function ConnectionForm({ initialProvider = "aws", onDone }: { initialProvider?: Provider; onDone?: () => void }) {
  const qc = useQueryClient();
  const projects = useProjects();
  const [created, setCreated] = useState<ConnectResponse | null>(null);
  type Values = { project_id: string; provider: Provider; account_ref: string; credential_ref: string; mode: "sp" | "export"; tenant_id: string; client_id: string; client_secret: string };
  // The rules depend on the provider that is selected, so the resolver picks the schema per submit.
  const { register, handleSubmit, reset, watch, formState: { errors } } = useForm<Values>({
    resolver: (values, ctx, opts) => zodResolver(values.provider === "gcp" ? gcp : values.provider === "azure" ? (values.mode === "export" ? azure : azureSp) : aws)(values as never, ctx, opts as never) as never,
    defaultValues: { provider: initialProvider, project_id: "", account_ref: "", credential_ref: "", mode: "sp", tenant_id: "", client_id: "", client_secret: "" },
  });
  const provider = watch("provider");
  const mode = watch("mode");

  const create = useMutation({
    mutationFn: (v: unknown) => api("cloud-accounts", connectResponse, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => { setCreated(r); reset({ provider: initialProvider, project_id: "", account_ref: "", credential_ref: "", mode: "sp", tenant_id: "", client_id: "", client_secret: "" }); void qc.invalidateQueries({ queryKey: ["connections"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  const verify = useMutation({
    mutationFn: (id: string) => api(`cloud-accounts/${id}/verify`, connection, undefined, { method: "POST" }),
    onSuccess: () => { setCreated(null); onDone?.(); void qc.invalidateQueries({ queryKey: ["connections"] }); },
  });

  if (created) {
    const { setup, connection: c } = created;
    const isGcp = c.provider === "gcp", isAzure = c.provider === "azure";
    const azureSpMode = isAzure && (c.credential_ref ?? "").startsWith("azcost://");
    return (
      <div className="space-y-4 text-sm">
        {azureSpMode ? (
          <>
            <p>Give the service principal read access to the costs of subscription <b>{c.account_ref}</b>:</p>
            <ol className="list-decimal space-y-2 pl-5">{(setup.steps ?? []).map((s, i) => <li key={i} className="break-words">{s}</li>)}</ol>
          </>
        ) : isAzure ? (
          <>
            <p>To read the Cost Management export of subscription <b>{c.account_ref}</b>, do these steps:</p>
            <ol className="list-decimal space-y-2 pl-5">{(setup.steps ?? []).map((s, i) => <li key={i} className="break-words">{s}</li>)}</ol>
            {setup.platform_account_id && <p className="text-muted">Grant this application read access to the container: <code className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs">{setup.platform_account_id}</code></p>}
            <p className="text-muted">Container ownership token (set as metadata, value = 1)</p>
            <code className="block break-all rounded-lg bg-slate-100 px-3 py-2 font-mono text-xs">{setup.external_id.replaceAll("-", "_")}</code>
          </>
        ) : isGcp ? (
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
    <form onSubmit={handleSubmit((v) => create.mutate(toRequest(v)))} className="grid gap-3 sm:grid-cols-2" noValidate>
      <Field label="Provider">
        <select {...register("provider")} className={input}>
          <option value="aws">AWS</option><option value="gcp">Google Cloud</option><option value="azure">Microsoft Azure</option>
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
      ) : provider === "azure" ? (
        <>
          <Field label="How to read Azure costs">
            <select {...register("mode")} className={input}>
              <option value="sp">Service principal (recommended)</option>
              <option value="export">Cost Management export in Blob Storage (advanced)</option>
            </select>
          </Field>
          <Field label="Azure subscription id" error={errors.account_ref?.message}><input {...register("account_ref")} className={input} placeholder="00000000-0000-0000-0000-000000000000" /></Field>
          {mode === "export" ? (
            <>
              <Field label="Cost Management export folder" error={errors.credential_ref?.message}><input {...register("credential_ref")} className={input} placeholder="azblob://<tenant id>@acmebilling/exports/reliabilix/daily-focus" /></Field>
              <p className="text-xs text-muted sm:col-span-2">Reliabilix reads the FOCUS cost and usage export from your Blob Storage container with its own Microsoft Entra application, read-only. This also gives measured virtual machine hours. We never ask for a key or a SAS token.</p>
            </>
          ) : (
            <>
              <Field label="Directory (tenant) id" error={(errors as Record<string, { message?: string }>).tenant_id?.message}><input {...register("tenant_id")} className={input} placeholder="00000000-0000-0000-0000-000000000000" autoComplete="off" /></Field>
              <Field label="Application (client) id" error={(errors as Record<string, { message?: string }>).client_id?.message}><input {...register("client_id")} className={input} placeholder="00000000-0000-0000-0000-000000000000" autoComplete="off" /></Field>
              <Field label="Client secret" error={(errors as Record<string, { message?: string }>).client_secret?.message}><input {...register("client_secret")} type="password" autoComplete="new-password" spellCheck={false} className={input} placeholder="the secret value" /></Field>
              <p className="text-xs text-muted sm:col-span-2">A service principal with the <b>Cost Management Reader</b> role on the subscription is enough: Reliabilix then reads the costs of every service. The secret is stored sealed, used only to read cost data and never shown again; delete the role assignment or the secret in Entra ID to revoke access.</p>
            </>
          )}
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
