"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { z } from "zod";
import { Panel } from "./organization-tab";
import { Segmented } from "@/components/operator/ui";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { IconShield } from "@/components/ui/icons";
import { Skeleton } from "@/components/ui/skeleton";
import { api, apiVoid } from "@/lib/api/client";
import { shortTime } from "@/lib/format-time";

const grant = z.object({ id: z.string(), granted_by: z.string(), granted_at: z.string(), expires_at: z.string(), revoked_at: z.string().nullable(), note: z.string(), active: z.boolean() });
const status = z.object({
  grants: z.array(grant).nullable().transform((v) => v ?? []),
  views: z.array(z.object({ operator: z.string(), what: z.string(), at: z.string() })).nullable().transform((v) => v ?? []),
  min_hours: z.number(), max_hours: z.number(),
});

const DURATIONS = [{ value: "1", label: "1 hour" }, { value: "4", label: "4 hours" }, { value: "24", label: "1 day" }, { value: "72", label: "3 days" }];

/**
 * Support access: the organization decides when the platform's operators may look at its working detail, for how long, and sees every
 * look. Without a grant operators see only counts. A grant can be closed at any moment and never lasts more than a week.
 */
export function SupportAccessPanel() {
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["support-access"], queryFn: () => api("support-access", status) });
  const [hours, setHours] = useState("4");
  const [note, setNote] = useState("");
  const done = () => void qc.invalidateQueries({ queryKey: ["support-access"] });
  const grantM = useMutation({
    mutationFn: () => api("support-access", grant, undefined, { method: "POST", body: JSON.stringify({ hours: Number(hours), note: note.trim() }) }),
    onSuccess: () => { setNote(""); done(); },
  });
  const revoke = useMutation({ mutationFn: () => apiVoid("support-access", { method: "DELETE" }), onSuccess: done });
  const open = q.data?.grants.find((g) => g.active);
  return (
    <Panel index={0} icon={IconShield} title="Support access" hint="Let the platform's support team look at your working detail for a limited time."
      info="Without it, the people who run the platform see only counts of what your organization has. With it open, they can read your project names and policies, your connections and their last error, and your recent activity: read only. Every look is listed below.">
      {q.isLoading ? <Skeleton className="h-24" /> : (
        <div className="space-y-4">
          {open ? (
            <div className="rise flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-amber-50/70 px-4 py-3 ring-1 ring-amber-100">
              <p className="text-sm text-[#77621a]"><b>Open</b> until {shortTime(open.expires_at)}{open.note ? ` · “${open.note}”` : ""}. Opened by {open.granted_by.slice(0, 24)}.</p>
              <ConfirmButton label="Close now" confirmLabel="Close support access" disabled={revoke.isPending} onConfirm={() => revoke.mutate()} />
            </div>
          ) : (
            <div className="space-y-3 rounded-2xl bg-slate-50/70 p-4 ring-1 ring-line/60">
              <p className="text-sm text-slate-600">Closed. Open it when you want support to look at something; it closes by itself.</p>
              <div className="flex flex-wrap items-center gap-3">
                <Segmented label="How long" value={hours} onChange={setHours} options={DURATIONS} />
                <input value={note} onChange={(e) => setNote(e.target.value)} maxLength={300} placeholder="What for (optional)"
                  aria-label="What the support access is for" className="h-10 min-w-48 flex-1 rounded-xl border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-leaf/40" />
                <button type="button" disabled={grantM.isPending} onClick={() => grantM.mutate()} className="h-10 rounded-xl btn-leaf px-4 text-sm font-semibold text-white disabled:opacity-60">
                  {grantM.isPending ? "Opening…" : "Open support access"}
                </button>
              </div>
              {grantM.isError && <p role="alert" className="text-sm text-bad">{grantM.error.message}</p>}
            </div>
          )}
          <div>
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">Who looked</p>
            {(q.data?.views.length ?? 0) === 0 ? <p className="text-sm text-muted">Nobody has looked.</p> : (
              <ul className="divide-y divide-line/60 text-sm">
                {q.data!.views.slice(0, 8).map((v, i) => (
                  <li key={i} className="flex flex-wrap items-center justify-between gap-2 py-2"><span className="truncate">{v.operator} <span className="text-muted">· {v.what}</span></span><span className="text-xs text-muted">{shortTime(v.at)}</span></li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
    </Panel>
  );
}
