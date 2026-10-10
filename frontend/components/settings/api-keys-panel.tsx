"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { KeyRound } from "lucide-react";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Field, input } from "@/components/forms/project-form";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { SecretOnce } from "@/components/ui/secret-once";
import { Skeleton } from "@/components/ui/skeleton";
import { useApiKeys } from "@/hooks/use-admin";
import { api, apiVoid } from "@/lib/api/client";
import { apiKeyCreated } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";

const schema = z.object({ name: z.string().trim().min(1, "Name is required").max(100, "Max 100 characters"), role: z.enum(["ci", "viewer", "agent"]) });
type Values = z.infer<typeof schema>;

export function ApiKeysPanel() {
  const qc = useQueryClient();
  const [key, setKey] = useState<string | null>(null);
  const q = useApiKeys();
  const { register, handleSubmit, reset, formState: { errors } } = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { role: "ci" } });
  const create = useMutation({
    mutationFn: (v: Values) => api("api-keys", apiKeyCreated, undefined, { method: "POST", body: JSON.stringify(v) }),
    onSuccess: (r) => { setKey(r.key); reset({ name: "", role: "ci" }); void qc.invalidateQueries({ queryKey: ["api-keys"] }); },
  });
  const revoke = useMutation({ mutationFn: (id: string) => apiVoid(`api-keys/${id}`, { method: "DELETE" }), onSuccess: () => void qc.invalidateQueries({ queryKey: ["api-keys"] }) });
  return (
    <div className="space-y-5">
      <Card>
        <CardHeader title="Create an API key" />
        <div className="space-y-3">
          {key && <SecretOnce title="API key" value={key} onDismiss={() => setKey(null)} />}
          <form onSubmit={handleSubmit((v) => create.mutate(v))} className="grid gap-3 sm:grid-cols-[2fr_1fr_auto] sm:items-start" noValidate>
            <Field label="Name" error={errors.name?.message}><input {...register("name")} className={input} placeholder="e.g. github-actions" /></Field>
            <Field label="Access"><select {...register("role")} className={input}><option value="ci">CI gate only</option><option value="viewer">Read only</option><option value="agent">Kubernetes agent</option></select></Field>
            <button disabled={create.isPending} className="brand-gradient mt-6 rounded-xl px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60">{create.isPending ? "Creating…" : "Create"}</button>
            {create.isError && <p role="alert" className="text-sm text-bad sm:col-span-3">{create.error.message}</p>}
          </form>
        </div>
      </Card>
      <Card>
        <CardHeader title="API keys" />
        {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={KeyRound} text="No API keys. Create one for a CI pipeline that calls the CI gate." /> : (
          <ul className="divide-y divide-line/70 text-sm">
            {q.data!.map((k) => (
              <li key={k.id} className="flex flex-wrap items-center justify-between gap-3 py-2.5">
                <div className="min-w-0">
                  <p className="flex items-center gap-2 font-medium">{k.name}<Badge>{k.role}</Badge>{k.revoked_at && <Badge tone="bad">revoked</Badge>}</p>
                  <p className="text-xs text-muted"><code>{k.prefix}…</code> · created {shortTime(k.created_at)} · {k.last_used_at ? `last used ${shortTime(k.last_used_at)}` : "never used"}</p>
                </div>
                {!k.revoked_at && <ConfirmButton label="Revoke" confirmLabel="Revoke key" disabled={revoke.isPending} onConfirm={() => revoke.mutate(k.id)} />}
              </li>
            ))}
          </ul>
        )}
        {revoke.isError && <p role="alert" className="mt-2 text-sm text-bad">{revoke.error.message}</p>}
      </Card>
    </div>
  );
}
