"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { project } from "@/lib/api/schemas";

const schema = z.object({
  name: z.string().trim().min(1, "Name is required").max(120, "Max 120 characters"),
  functional_unit: z.string().trim().min(1, "Functional unit is required").max(60, "Max 60 characters"),
});
type Values = z.infer<typeof schema>;

export function ProjectForm() {
  const qc = useQueryClient();
  const { register, handleSubmit, reset, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { functional_unit: "request" } });
  const m = useMutation({
    mutationFn: (v: Values) => api("projects", project, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: () => { reset({ name: "", functional_unit: "request" }); void qc.invalidateQueries({ queryKey: ["projects"] }); void qc.invalidateQueries({ queryKey: ["activity"] }); },
  });
  return (
    <form onSubmit={handleSubmit((v) => m.mutate(v))} className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-start" noValidate>
      <Field label="Project name" error={errors.name?.message}><input {...register("name")} className={input} placeholder="e.g. production-api" /></Field>
      <Field label="Functional unit (SCI)" error={errors.functional_unit?.message}><input {...register("functional_unit")} className={input} /></Field>
      <button disabled={m.isPending} className="brand-gradient mt-6 rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{m.isPending ? "Creating…" : "Create project"}</button>
      {m.isError && <p role="alert" className="text-sm text-bad sm:col-span-3">{m.error.message}</p>}
    </form>
  );
}

export const input = "w-full rounded-xl bg-white/80 px-3 py-2 text-sm ring-1 ring-line outline-none focus:ring-2 focus:ring-primary";
export function Field({ label, error, children }: { label: string; error?: string; children: React.ReactNode }) {
  return (
    <label className="block text-sm"><span className="mb-1 block font-medium text-slate-600">{label}</span>{children}
      {error && <span role="alert" className="mt-1 block text-xs text-bad">{error}</span>}</label>
  );
}
