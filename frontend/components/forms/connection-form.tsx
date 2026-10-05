"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { connection } from "@/lib/api/schemas";
import { useProjects } from "@/hooks/use-overview";
import { Field, input } from "./project-form";

const schema = z.object({
  project_id: z.string().min(1, "Choose a project"),
  provider: z.literal("aws"),
  account_ref: z.string().trim().regex(/^\d{12}$/, "AWS account id: 12 digits"),
  credential_ref: z.string().trim().regex(/^arn:aws:iam::\d{12}:role\/.+/, "Enter the IAM role ARN (not an access key)"),
});
type Values = z.infer<typeof schema>;

export function ConnectionForm() {
  const qc = useQueryClient();
  const projects = useProjects();
  const { register, handleSubmit, reset, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { provider: "aws" } });
  const m = useMutation({
    mutationFn: (v: Values) => api("cloud-accounts", connection, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: () => { reset(); void qc.invalidateQueries({ queryKey: ["connections"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  return (
    <form onSubmit={handleSubmit((v) => m.mutate(v))} className="grid gap-3 sm:grid-cols-2" noValidate>
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
        <button disabled={m.isPending} className="brand-gradient rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{m.isPending ? "Connecting…" : "Connect account"}</button>
        {m.isError && <p role="alert" className="mt-2 text-sm text-bad">{m.error.message}</p>}
      </div>
    </form>
  );
}
