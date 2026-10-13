"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconCheckCircle as CheckCircle2, IconWarning as TriangleAlert, IconKey as KeyRound } from "@/components/ui/icons";
import { useState } from "react";
import { Pill } from "./ui";
import { Btn, inputCls } from "./ui";
import { api } from "@/lib/api/client";
import { managedIntegrations, testResult, type IntegrationField, type ManagedIntegration } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";

const SOURCE = { console: "Set here", environment: "From the environment", not_set: "Not set" } as const;

function FieldRow({ f, draft, onChange, onReset, canEdit, index }: {
  f: IntegrationField; draft: string | undefined; onChange: (v: string) => void; onReset: () => void; canEdit: boolean; index: number;
}) {
  const placeholder = f.secret ? (f.configured ? `${f.hint ?? "••••"} — type to replace` : f.placeholder ?? "Paste the key here") : f.placeholder ?? f.default ?? "";
  const tone = f.source === "not_set" ? (f.required ? "warn" : "neutral") : "good";
  return (
    <div className="rise rounded-2xl bg-slate-50/60 p-4 ring-1 ring-line/70 transition-shadow focus-within:ring-leaf/40" style={{ "--i": index } as React.CSSProperties}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <label htmlFor={`f-${f.key}`} className="text-sm font-semibold">{f.label}{f.required && <span className="text-[#b8625c]"> *</span>}</label>
        <span className="flex items-center gap-2">
          <code className="rounded-md bg-white px-1.5 py-0.5 text-[11px] text-slate-500 ring-1 ring-line">{f.env}</code>
          <Pill tone={tone}>{SOURCE[f.source]}</Pill>
        </span>
      </div>
      <div className="mt-2.5 flex gap-2">
        <div className="relative flex-1">
          {f.secret && <KeyRound className="pointer-events-none absolute left-3.5 top-1/2 size-4 -translate-y-1/2 text-slate-400" aria-hidden />}
          <input id={`f-${f.key}`} type={f.secret ? "password" : "text"} autoComplete="off" spellCheck={false}
            value={draft ?? (f.secret ? "" : f.value ?? "")} placeholder={placeholder} disabled={!canEdit}
            onChange={(e) => onChange(e.target.value)} className={`${inputCls} ${f.secret ? "pl-10" : ""}`} />
        </div>
        {canEdit && f.source === "console" && <Btn tone="quiet" onClick={onReset} title="Forget the value set here and use the deployment environment again">Use environment</Btn>}
      </div>
      {(f.help || (f.secret && f.configured) || f.updated_at) && (
        <p className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
          {f.help && <span>{f.help}</span>}
          {f.secret && f.configured && <span className="inline-flex items-center gap-1 text-leaf-deep"><KeyRound className="size-3" aria-hidden />stored sealed, never shown</span>}
          {f.updated_at && <span>changed {shortTime(f.updated_at)}{f.updated_by ? ` by ${f.updated_by.slice(0, 18)}` : ""}</span>}
        </p>
      )}
    </div>
  );
}

/** The settings of one integration: fields, save, test. Used on the page of the source or service it belongs to. */
export function IntegrationForm({ st, canEdit }: { st: ManagedIntegration; canEdit: boolean }) {
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
    <div>
      <div className="space-y-3">
        {st.fields.map((f, i) => (
          <FieldRow key={f.key} f={f} canEdit={canEdit} draft={draft[f.key]} index={i}
            onChange={(v) => { setDraft((d) => ({ ...d, [f.key]: v })); setResets((r) => { const n = new Set(r); n.delete(f.key); return n; }); }}
            onReset={() => { setResets((r) => new Set(r).add(f.key)); setDraft((d) => { const n = { ...d }; delete n[f.key]; return n; }); }} />
        ))}
      </div>
      {resets.size > 0 && <p className="mt-3 text-xs text-muted">Will return to the environment: {[...resets].join(", ")}.</p>}
      {msg && (
        <p role="status" className={`rise mt-4 flex items-start gap-2 rounded-xl p-3 text-sm ring-1 ${msg.ok ? "bg-leaf-soft text-leaf-deep ring-leaf/20" : "bg-rose-50 text-[#a8504b] ring-rose-200"}`}>
          {msg.ok ? <CheckCircle2 className="mt-0.5 size-4 shrink-0" /> : <TriangleAlert className="mt-0.5 size-4 shrink-0" />}{msg.text}
        </p>
      )}
      {(canEdit || st.testable) && (
        <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-line/70 pt-4">
          {canEdit && <Btn disabled={!dirty || save.isPending} onClick={() => save.mutate()}>{save.isPending ? "Saving…" : "Save"}</Btn>}
          {st.testable && canEdit && <Btn tone="quiet" disabled={test.isPending || dirty} onClick={() => test.mutate()} title={dirty ? "Save first: the test uses the saved values" : undefined}>{test.isPending ? "Testing…" : "Test connection"}</Btn>}
          {dirty && <span className="text-xs text-[#8a6420]">Unsaved changes</span>}
        </div>
      )}
    </div>
  );
}
