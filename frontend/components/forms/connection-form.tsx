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

const schema = z.object({
  project_id: z.string().min(1, "Choose a project"),
  provider: z.literal("aws"),
  account_ref: z.string().trim().regex(/^\d{12}$/, "AWS account id: 12 digits"),
  credential_ref: z.string().trim().regex(/^arn:aws:iam::\d{12}:role\/.+/, "Enter the IAM role ARN (not an access key)"),
});
type Values = z.infer<typeof schema>;

/** Step 1: register the connection and show the ExternalId + policies. Step 2: verify access once the role exists. */
export function ConnectionForm() {
  const qc = useQueryClient();
  const projects = useProjects();
  const [created, setCreated] = useState<ConnectResponse | null>(null);
  const { register, handleSubmit, reset, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { provider: "aws" } });

  const create = useMutation({
    mutationFn: (v: Values) => api("cloud-accounts", connectResponse, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => { setCreated(r); reset(); void qc.invalidateQueries({ queryKey: ["connections"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  const verify = useMutation({
    mutationFn: (id: string) => api(`cloud-accounts/${id}/verify`, connection, undefined, { method: "POST" }),
    onSuccess: () => { setCreated(null); void qc.invalidateQueries({ queryKey: ["connections"] }); },
  });

  if (created) {
    const { setup, connection: c } = created;
    return (
      <div className="space-y-4 text-sm">
        <p>Create an IAM role in AWS account <b>{c.account_ref}</b> that trusts Reliabilix, using this <b>External ID</b>:</p>
        <code className="block rounded-lg bg-slate-100 px-3 py-2 font-mono text-xs">{setup.external_id}</code>
        {setup.trust_policy && (<><p className="text-muted">Trust policy</p><pre className="overflow-x-auto rounded-lg bg-slate-100 p-3 text-xs">{JSON.stringify(setup.trust_policy, null, 2)}</pre></>)}
        <p className="text-muted">Permissions policy (read-only billing access)</p>
        <pre className="overflow-x-auto rounded-lg bg-slate-100 p-3 text-xs">{JSON.stringify(setup.permissions_policy, null, 2)}</pre>
        <button onClick={() => verify.mutate(c.id)} disabled={verify.isPending} className="brand-gradient rounded-xl px-4 py-2.5 font-semibold text-white disabled:opacity-60">
          {verify.isPending ? "Verifying…" : "Verify access"}
        </button>
        {verify.isError && <p role="alert" className="text-bad">{verify.error.message} — check the trust policy and permissions, then try again.</p>}
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit((v) => create.mutate(v))} className="grid gap-3 sm:grid-cols-2" noValidate>
      <Field label="Project" error={errors.project_id?.message}>
        <select {...register("project_id")} className={input} defaultValue="">
          <option value="" disabled>Select project…</option>
          {projects.data?.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
        </select>
      </Field>
      <Field label="Provider"><select {...register("provider")} className={input}><option value="aws">AWS</option><option disabled>Azure (coming soon)</option><option disabled>GCP (coming soon)</option></select></Field>
      <Field label="AWS account id" error={errors.account_ref?.message}><input {...register("account_ref")} className={input} placeholder="123456789012" inputMode="numeric" /></Field>
      <Field label="IAM role ARN (read-only)" error={errors.credential_ref?.message}><input {...register("credential_ref")} className={input} placeholder="arn:aws:iam::123456789012:role/ReliabilixReadOnly" /></Field>
      <p className="text-xs text-muted sm:col-span-2">Reliabilix assumes this role with an External ID. We never ask for access keys.</p>
      <div className="sm:col-span-2">
        <button disabled={create.isPending} className="brand-gradient rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{create.isPending ? "Registering…" : "Continue"}</button>
        {create.isError && <p role="alert" className="mt-2 text-sm text-bad">{create.error.message}</p>}
      </div>
    </form>
  );
}
