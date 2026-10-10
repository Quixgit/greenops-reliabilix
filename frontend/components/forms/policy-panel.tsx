"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { z } from "zod";
import { Switch } from "@/components/operator/ui";
import { api } from "@/lib/api/client";

const policy = z.object({
  project_id: z.string(),
  allowed_regions: z.array(z.string()),
  ci_carbon_warn_kg: z.number().nullable(),
  ci_carbon_block_kg: z.number().nullable(),
  ci_cost_warn: z.number().nullable(),
  ci_cost_block: z.number().nullable(),
  alert_carbon_monthly_kg: z.number().nullable().optional(),
  auto_apply_types: z.array(z.string()).default([]),
});

const num = (v: string): number | null => (v.trim() === "" ? null : Number(v));
const input = "block h-10 w-full rounded-lg border border-line px-3 text-sm disabled:bg-slate-50";

/** Data residency, the CI gate and the monthly carbon alert limit of one project. The API validates every field. */
export function PolicyPanel({ projectId, canEdit }: { projectId: string; canEdit: boolean }) {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["policy", projectId], queryFn: () => api(`projects/${projectId}/policy`, policy) });
  const [f, setF] = useState({ regions: "", cw: "", cb: "", kw: "", kb: "", limit: "" });
  const [auto, setAuto] = useState<string[]>([]);
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  useEffect(() => {
    const p = q.data;
    if (p) setAuto(p.auto_apply_types);
    if (p) setF({
      regions: p.allowed_regions.join(", "), cw: p.ci_carbon_warn_kg?.toString() ?? "", cb: p.ci_carbon_block_kg?.toString() ?? "",
      kw: p.ci_cost_warn?.toString() ?? "", kb: p.ci_cost_block?.toString() ?? "", limit: p.alert_carbon_monthly_kg?.toString() ?? "",
    });
  }, [q.data]);
  const save = useMutation({
    mutationFn: () => api(`projects/${projectId}/policy`, policy, undefined, {
      method: "PUT",
      body: JSON.stringify({
        allowed_regions: f.regions.split(",").map((s) => s.trim()).filter(Boolean),
        ci_carbon_warn_kg: num(f.cw), ci_carbon_block_kg: num(f.cb), ci_cost_warn: num(f.kw), ci_cost_block: num(f.kb), alert_carbon_monthly_kg: num(f.limit), auto_apply_types: auto,
      }),
    }),
    onSuccess: (p) => { qc.setQueryData(["policy", projectId], p); setMsg({ ok: true, text: "Saved." }); },
    onError: (e) => setMsg({ ok: false, text: e.message }),
  });
  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement>) => { setF((s) => ({ ...s, [k]: e.target.value })); setMsg(null); };
  if (q.isLoading) return <p className="py-3 text-sm text-muted">Loading…</p>;
  if (q.isError) return <p role="alert" className="py-3 text-sm text-bad">The policy could not be loaded.</p>;
  return (
    <div className="space-y-4 rounded-xl bg-slate-50 p-4 ring-1 ring-line">
      <label className="block text-sm">
        <span className="font-medium">Allowed regions</span>
        <span className="ml-2 text-xs text-muted">data residency: region shifts are only proposed into these</span>
        <input className={`${input} mt-1`} value={f.regions} onChange={set("regions")} disabled={!canEdit} placeholder="eu-central-1, eu-west-1" />
      </label>
      <div className="grid gap-3 sm:grid-cols-4">
        {([["cw", "CI warn, kg CO₂e"], ["cb", "CI block, kg CO₂e"], ["kw", "CI warn, cost"], ["kb", "CI block, cost"]] as const).map(([k, label]) => (
          <label key={k} className="block text-sm"><span className="text-xs font-medium text-slate-600">{label}</span>
            <input className={`${input} mt-1`} inputMode="decimal" value={f[k]} onChange={set(k)} disabled={!canEdit} /></label>
        ))}
      </div>
      <label className="block max-w-xs text-sm">
        <span className="text-xs font-medium text-slate-600">Monthly carbon limit, kg CO₂e</span>
        <input className={`${input} mt-1`} inputMode="decimal" value={f.limit} onChange={set("limit")} disabled={!canEdit} placeholder="no limit" />
        <span className="mt-1 block text-xs text-muted">An alert opens at the warning level and at the limit.</span>
      </label>
      <div className="rounded-xl bg-white p-3 ring-1 ring-line">
        <p className="text-sm font-medium">Apply without approval</p>
        <p className="mb-2 text-xs text-muted">What the platform may do on its own for this project. Everything else waits for a person.</p>
        <ul className="divide-y divide-line/70 text-sm">
          <li className="flex items-center gap-3 py-2">
            <span className="min-w-0 flex-1"><span className="font-medium">Time shifting</span><span className="block text-xs text-muted">Run a flexible job in the cleanest hours of the grid forecast. Repeatable, moves no data.</span></span>
            <Switch label="Apply time shifting without approval" checked={auto.includes("time_shift")} disabled={!canEdit}
              onChange={(v) => { setAuto((a) => (v ? [...a, "time_shift"] : a.filter((x) => x !== "time_shift"))); setMsg(null); }} />
          </li>
          {[["Region moves", "Moves a workload to another region: data residency, price and latency need a person."], ["Rightsizing", "Changes production infrastructure: always needs approval."], ["Spot migration", "Changes how production runs: always needs approval."]].map(([t, d]) => (
            <li key={t} className="flex items-center gap-3 py-2 opacity-70">
              <span className="min-w-0 flex-1"><span className="font-medium">{t}</span><span className="block text-xs text-muted">{d}</span></span>
              <Switch label={`${t}: always requires approval`} checked={false} disabled onChange={() => undefined} />
            </li>
          ))}
        </ul>
      </div>
      {msg && <p role="status" className={`text-sm ${msg.ok ? "text-emerald-700" : "text-bad"}`}>{msg.text}</p>}
      {canEdit && <button disabled={save.isPending} onClick={() => save.mutate()} className="rounded-lg bg-primary px-3.5 py-1.5 text-sm font-semibold text-white disabled:opacity-60">Save policy</button>}
    </div>
  );
}
