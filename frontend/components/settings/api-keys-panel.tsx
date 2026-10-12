"use client";


import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconKey as KeyRound, IconPlus as Plus } from "@/components/ui/icons";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Field, input } from "@/components/forms/project-form";
import { Badge } from "@/components/ui/badge";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { SecretOnce } from "@/components/ui/secret-once";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useApiKeys } from "@/hooks/use-admin";
import { api, apiVoid } from "@/lib/api/client";
import { apiKeyCreated } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { useT } from "@/i18n/provider";
import { Panel } from "./organization-tab";

const schema = z.object({ name: z.string().trim().min(1, "Name is required").max(100, "Max 100 characters"), role: z.enum(["ci", "viewer", "agent", "scim", "iac"]) });
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
  const t = useT();
  const active = (q.data ?? []).filter((k) => !k.revoked_at).length;
  const pg = usePaged(q.data ?? [], 10);
  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <Panel info={t("info.set_keys")} index={0} icon={KeyRound} title={t("settings.keys.title")} hint={t("settings.keys.hint", { count: active })} className="lg:col-span-2">
        {q.isLoading ? <Skeleton className="h-24" /> : q.isError ? <ErrorState /> : q.data!.length === 0 ? <EmptyState icon={KeyRound} text="No API keys. Create one for a CI pipeline that calls the CI gate." /> : (
          <ul className="text-sm">
            {pg.slice.map((k) => (
              <li key={k.id} className="flex flex-wrap items-center justify-between gap-3 border-t border-line/70 py-2.5 transition-colors hover:bg-slate-50/70">
                <div className="min-w-0">
                  <p className="flex items-center gap-2 font-medium">{k.name}<Badge>{k.role}</Badge>{k.revoked_at && <Badge tone="bad">revoked</Badge>}</p>
                  <p className="text-xs text-muted"><code>{k.prefix}…</code> · created {shortTime(k.created_at)} · {k.last_used_at ? `last used ${shortTime(k.last_used_at)}` : "never used"}</p>
                </div>
                {!k.revoked_at && <ConfirmButton label="Revoke" confirmLabel="Revoke key" disabled={revoke.isPending} onConfirm={() => revoke.mutate(k.id)} />}
              </li>
            ))}
          </ul>
        )}
        <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
        {revoke.isError && <p role="alert" className="mt-2 text-sm text-bad">{revoke.error.message}</p>}
      </Panel>
      <Panel info={t("info.set_keys_new")} index={1} icon={Plus} title={t("settings.keys.create")} hint={t("settings.keys.create_hint")}>
        <div className="space-y-3">
          {key && <SecretOnce title="API key" value={key} onDismiss={() => setKey(null)} />}
          <form onSubmit={handleSubmit((v) => create.mutate(v))} className="grid gap-3" noValidate>
            <Field label="Name" error={errors.name?.message}><input {...register("name")} className={input} placeholder="e.g. github-actions" /></Field>
            <Field label="Access"><select {...register("role")} className={input}><option value="ci">CI gate only</option><option value="viewer">Read only</option><option value="agent">Kubernetes agent</option><option value="scim">{t("settings.keys.role.scim")}</option><option value="iac">{t("settings.keys.role.iac")}</option></select></Field>
            <button disabled={create.isPending} className="rounded-xl btn-leaf px-4 py-2.5 text-sm font-semibold text-white active:scale-[0.98] disabled:opacity-60">{create.isPending ? "Creating…" : "Create"}</button>
            {create.isError && <p role="alert" className="text-sm text-bad">{create.error.message}</p>}
          </form>
        </div>
      </Panel>
    </div>
  );
}
