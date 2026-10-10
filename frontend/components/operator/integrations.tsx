"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, KeyRound, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorIntegrations, useOperatorMe, useOperatorSystem } from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { managedIntegrations, testResult, type IntegrationField, type ManagedIntegration } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";

const SOURCE = { console: "Set here", environment: "From the environment", not_set: "Not set" } as const;

function FieldRow({ f, draft, onChange, onReset, canEdit }: {
  f: IntegrationField; draft: string | undefined; onChange: (v: string) => void; onReset: () => void; canEdit: boolean;
}) {
  const placeholder = f.secret ? (f.configured ? `${f.hint ?? "••••"} — type to replace` : f.placeholder ?? "") : f.placeholder ?? f.default ?? "";
  return (
    <div className="grid gap-1.5 sm:grid-cols-[180px_1fr] sm:items-start">
      <label htmlFor={`f-${f.key}`} className="pt-2 text-sm font-medium">
        {f.label}{f.required && <span className="text-bad"> *</span>}
        <span className="mt-0.5 block text-[11px] font-normal text-muted">{f.env}</span>
      </label>
      <div className="space-y-1">
        <div className="flex gap-2">
          <input id={`f-${f.key}`} type={f.secret ? "password" : "text"} autoComplete="off" spellCheck={false}
            value={draft ?? (f.secret ? "" : f.value ?? "")} placeholder={placeholder} disabled={!canEdit}
            onChange={(e) => onChange(e.target.value)}
            className="block w-full rounded-lg border border-line px-3 py-2 text-sm disabled:bg-slate-50" />
          {canEdit && f.source === "console" && (
            <button type="button" onClick={onReset} className="shrink-0 rounded-lg px-2.5 text-xs font-medium text-muted ring-1 ring-line hover:bg-slate-50">Use environment</button>
          )}
        </div>
        <p className="flex flex-wrap items-center gap-x-2 text-xs text-muted">
          <Badge tone={f.source === "not_set" ? (f.required ? "warn" : "neutral") : "good"}>{SOURCE[f.source]}</Badge>
          {f.secret && f.configured && <span><KeyRound className="mr-1 inline size-3" />stored sealed, never shown</span>}
          {f.updated_at && <span>changed {shortTime(f.updated_at)}{f.updated_by ? ` by ${f.updated_by.slice(0, 18)}` : ""}</span>}
        </p>
        {f.help && <p className="text-xs text-muted">{f.help}</p>}
      </div>
    </div>
  );
}

function IntegrationCard({ st, canEdit }: { st: ManagedIntegration; canEdit: boolean }) {
  const qc = useQueryClient();
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [resets, setResets] = useState<Set<string>>(new Set());
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const save = useMutation({
    mutationFn: () => {
      const values: Record<string, string | null> = {};
      for (const [k, v] of Object.entries(draft)) if (v.trim() !== "") values[k] = v.trim();
      for (const k of resets) values[k] = null;
      return api(`operator/integrations/${st.id}`, managedIntegrations, undefined, { method: "PUT", body: JSON.stringify({ values }) });
    },
    onSuccess: (data) => {
      qc.setQueryData(["op-integrations"], data);
      setDraft({});
      setResets(new Set());
      setMsg({ ok: true, text: st.startup ? "Saved. It is applied when the services restart." : "Saved. It applies within a few seconds, without a restart." });
    },
    onError: (e) => setMsg({ ok: false, text: e.message }),
  });
  const test = useMutation({
    mutationFn: () => api(`operator/integrations/${st.id}/test`, testResult, undefined, { method: "POST", body: "{}" }),
    onSuccess: (r) => setMsg({ ok: r.ok, text: r.message }),
    onError: (e) => setMsg({ ok: false, text: e.message }),
  });
  const dirty = Object.values(draft).some((v) => v.trim() !== "") || resets.size > 0;
  return (
    <Card>
      <CardHeader title={<span className="flex items-center gap-2">{st.name}<Badge tone={st.ready ? "good" : "warn"}>{st.ready ? "Ready" : "Needs setup"}</Badge>{st.startup && <Badge tone="neutral">Applied at restart</Badge>}</span>} />
      <p className="-mt-2 mb-4 text-sm text-muted">{st.description}</p>
      <div className="space-y-4">
        {st.fields.map((f) => (
          <FieldRow key={f.key} f={f} canEdit={canEdit} draft={draft[f.key]}
            onChange={(v) => { setDraft((d) => ({ ...d, [f.key]: v })); setResets((r) => { const n = new Set(r); n.delete(f.key); return n; }); }}
            onReset={() => { setResets((r) => new Set(r).add(f.key)); setDraft((d) => { const n = { ...d }; delete n[f.key]; return n; }); }} />
        ))}
      </div>
      {resets.size > 0 && <p className="mt-3 text-xs text-muted">Will return to the environment: {[...resets].join(", ")}.</p>}
      {msg && (
        <p role="status" className={`mt-4 flex items-start gap-2 rounded-lg p-3 text-sm ring-1 ${msg.ok ? "bg-emerald-50 text-emerald-800 ring-emerald-200" : "bg-red-50 text-bad ring-red-200"}`}>
          {msg.ok ? <CheckCircle2 className="mt-0.5 size-4 shrink-0" /> : <TriangleAlert className="mt-0.5 size-4 shrink-0" />}{msg.text}
        </p>
      )}
      <div className="mt-4 flex flex-wrap gap-2">
        {canEdit && <button disabled={!dirty || save.isPending} onClick={() => save.mutate()} className="rounded-lg bg-primary px-3.5 py-1.5 text-sm font-semibold text-white disabled:opacity-50">Save</button>}
        {st.testable && canEdit && <button disabled={test.isPending || dirty} onClick={() => test.mutate()} title={dirty ? "Save first: the test uses the saved values" : undefined}
          className="rounded-lg px-3.5 py-1.5 text-sm font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-50">Test connection</button>}
      </div>
    </Card>
  );
}

export function IntegrationsView() {
  const q = useOperatorIntegrations();
  const sys = useOperatorSystem();
  const me = useOperatorMe();
  const canEdit = me.data?.role === "admin";
  const fixed = (sys.data?.integrations ?? []).filter((i) => !["Electricity Maps (grid carbon)", "E-mail (SMTP)", "Object storage (reports)", "AWS connector", "GCP connector", "Sentry (errors)", "Tracing (OTLP)"].includes(i.name));
  return (
    <div className="max-w-4xl space-y-5">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Integrations</h1>
        <p className="mt-1 text-sm text-muted">Keys and addresses of the external services. A value set here wins over the deployment environment; secrets are stored sealed and are never shown again.</p>
      </div>
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError ? <ErrorState /> : q.data!.map((st) => <IntegrationCard key={st.id} st={st} canEdit={canEdit} />)}
      {fixed.length > 0 && (
        <Card>
          <CardHeader title="Fixed by the deployment" />
          <p className="-mt-2 mb-3 text-sm text-muted">These are fixed by the deployment or are not available yet.</p>
          <ul className="divide-y divide-line/70 text-sm">
            {fixed.map((i) => (
              <li key={i.name} className="flex flex-wrap items-center justify-between gap-2 py-2.5">
                <span>{i.name}{i.note && <span className="ml-2 text-xs text-muted">{i.note}</span>}</span>
                <Badge tone={i.status === "ready" ? "good" : i.status === "stub" ? "stub" : "warn"}>{i.status === "ready" ? "Ready" : i.status === "stub" ? "Stand-in" : "Not set"}</Badge>
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}
