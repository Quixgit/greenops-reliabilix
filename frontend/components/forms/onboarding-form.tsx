"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { api } from "@/lib/api/client";
import { tenantRef } from "@/lib/api/schemas";
import { Field, input } from "./project-form";

const schema = z.object({ organization_name: z.string().trim().min(1, "Organization name is required").max(200, "Max 200 characters") });
type Values = z.infer<typeof schema>;

export function OnboardingForm() {
  const router = useRouter();
  const { register, handleSubmit, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema) });
  const m = useMutation({
    mutationFn: (v: Values) => api("onboarding", tenantRef, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: () => router.replace("/overview"),
  });
  return (
    <form onSubmit={handleSubmit((v) => m.mutate(v))} className="space-y-3" noValidate>
      <Field label="Organization name" error={errors.organization_name?.message}><input {...register("organization_name")} className={input} placeholder="Acme Inc." autoFocus /></Field>
      <button disabled={m.isPending} className="brand-gradient w-full rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{m.isPending ? "Creating…" : "Create organization"}</button>
      {m.isError && <p role="alert" className="text-sm text-bad">{m.error.message}</p>}
    </form>
  );
}
