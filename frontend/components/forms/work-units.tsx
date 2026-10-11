"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { z } from "zod";
import { Calendar } from "@/components/ui/calendar";
import { IconGauge } from "@/components/ui/icons";
import { api } from "@/lib/api/client";
import { nf } from "@/lib/formatters";
import { shortTime } from "@/lib/format-time";

const units = z.object({ id: z.string(), project_id: z.string(), period_start: z.string(), period_end: z.string(), units: z.number() });
const list = z.object({ items: z.array(units) }).transform((o) => o.items);

/** What the project produced in a period (requests, builds, jobs): the denominator that turns totals into cost and CO₂e per unit. */
export function WorkUnits({ projectId, unit, canEdit }: { projectId: string; unit: string; canEdit: boolean }) {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["units", projectId], queryFn: () => api(`projects/${projectId}/functional-units`, list) });
  const [range, setRange] = useState<{ from?: string; to?: string }>({});
  const [n, setN] = useState("");
  const value = Number(n.replace(/\s/g, "").replace(",", "."));
  const valid = !!range.from && Number.isFinite(value) && value > 0;
  const save = useMutation({
    mutationFn: () => api(`projects/${projectId}/functional-units`, units, undefined, { method: "POST", body: JSON.stringify({ period_start: range.from, period_end: range.to ?? range.from, units: value }) }),
    onSuccess: () => { setN(""); setRange({}); void qc.invalidateQueries({ queryKey: ["units", projectId] }); void qc.invalidateQueries({ queryKey: ["efficiency"] }); void qc.invalidateQueries({ queryKey: ["carbon-summary"] }); },
  });
  const unitName = unit.replace(/_/g, " ");
  return (
    <div className="space-y-4">
      <p className="text-sm leading-relaxed text-slate-600">Report how many <b>{unitName}</b> this project served. Cost and CO₂e per {unitName} appear on the Overview from the days you report.</p>
      {canEdit && (
        <div className="grid gap-4 rounded-2xl border border-line bg-slate-50/50 p-3 sm:grid-cols-[auto_1fr]">
          <Calendar mode="range" from={range.from} to={range.to} max={new Date().toISOString().slice(0, 10)} onPick={(f, t) => setRange({ from: f, to: t })} />
          <div className="flex flex-col justify-between gap-3">
            <div className="text-sm">
              <p className="text-xs font-medium text-slate-500">Period</p>
              <p className="mt-0.5 font-medium">{range.from ? `${range.from}${range.to ? ` → ${range.to}` : ""}` : "Pick the first and the last day"}</p>
              <label className="mt-3 block"><span className="mb-1 block text-xs font-medium text-slate-500">Units served in the period</span>
                <input value={n} onChange={(e) => setN(e.target.value)} inputMode="decimal" placeholder="e.g. 1 200 000" className="w-full rounded-xl border border-line bg-white px-3 py-2 text-sm" /></label>
            </div>
            {save.isError && <p role="alert" className="text-xs text-bad">{save.error.message}</p>}
            <button type="button" disabled={!valid || save.isPending} onClick={() => save.mutate()} className="rounded-xl bg-leaf-deep px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:brightness-110 disabled:opacity-50">{save.isPending ? "Saving…" : "Report units"}</button>
          </div>
        </div>
      )}
      <ul className="text-sm">
        {(q.data ?? []).slice(0, 6).map((u) => (
          <li key={u.id} className="flex items-center gap-3 border-t border-line/70 py-2.5">
            <IconGauge className="size-5 text-leaf-deep" />
            <span className="flex-1 text-muted">{shortTime(u.period_start)} → {shortTime(u.period_end)}</span>
            <span className="font-semibold tabular-nums">{nf(0).format(u.units)}</span>
          </li>
        ))}
        {(q.data ?? []).length === 0 && <li className="border-t border-line/70 py-3 text-muted">Nothing reported yet.</li>}
      </ul>
    </div>
  );
}
