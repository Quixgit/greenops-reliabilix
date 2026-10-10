"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download } from "lucide-react";
import { useState } from "react";
import { z } from "zod";
import { Card, CardHeader } from "@/components/ui/card";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { api, apiVoid } from "@/lib/api/client";
import { shortTime } from "@/lib/format-time";

const deletion = z.object({
  scheduled: z.boolean(),
  requested_by: z.string().optional(),
  requested_at: z.string().optional(),
  execute_after: z.string().optional(),
});

/** Export and erasure of the whole organization. Owners only: the Go API enforces it again. */
export function DataPanel() {
  const qc = useQueryClient();
  const org = useQuery({ queryKey: ["tenant"], queryFn: () => api("tenant", z.object({ name: z.string() })), staleTime: 5 * 60_000 });
  const orgName = org.data?.name ?? "";
  const [typed, setTyped] = useState("");
  const q = useQuery({ queryKey: ["deletion"], queryFn: () => api("tenant/deletion", deletion) });
  const schedule = useMutation({
    mutationFn: () => api("tenant/deletion", deletion, undefined, { method: "POST", body: JSON.stringify({ confirm_name: typed }) }),
    onSuccess: () => { setTyped(""); void qc.invalidateQueries({ queryKey: ["deletion"] }); },
  });
  const cancel = useMutation({
    mutationFn: () => apiVoid("tenant/deletion", { method: "DELETE" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["deletion"] }),
  });
  const scheduled = q.data?.scheduled;
  return (
    <div className="space-y-5">
      <Card>
        <CardHeader title="Export your data" />
        <p className="text-sm text-muted">
          Everything the organization stores — projects, connections, usage, carbon results, recommendations, members and the audit
          log — as one NDJSON file. Secrets (API key hashes, notification targets, AWS ExternalIds) and generated report files are not included.
        </p>
        <a href="/api/proxy/tenant/export" download
          className="mt-3 inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-semibold text-primary ring-1 ring-line hover:bg-slate-50">
          <Download className="size-4" />Download export
        </a>
      </Card>
      <Card>
        <CardHeader title="Delete this organization" />
        {scheduled ? (
          <div className="space-y-3 text-sm">
            <p role="status" className="rounded-lg bg-red-50 p-3 text-bad ring-1 ring-red-200">
              Erasure is scheduled for <b>{q.data?.execute_after ? shortTime(q.data.execute_after) : "—"}</b>. Until then everything keeps
              working and you can cancel. After that all data of this organization is removed and cannot be recovered.
            </p>
            <ConfirmButton label="Cancel the erasure" confirmLabel="Yes, cancel it" onConfirm={() => cancel.mutate()} disabled={cancel.isPending}
              className="!text-slate-700 !ring-line hover:!bg-slate-50" />
          </div>
        ) : (
          <div className="space-y-3 text-sm">
            <p className="text-muted">
              Removes all data of the organization — projects, connections, usage and carbon history, recommendations, members, API keys
              and the audit log — after a grace period in which you can cancel. Download the export first.
            </p>
            <label className="block">
              <span className="text-xs text-muted">Type the organization name <b className="text-ink">{orgName}</b> to confirm</span>
              <input value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off"
                className="mt-1 block w-full max-w-sm rounded-lg border border-line px-3 py-2 text-sm" />
            </label>
            <button type="button" disabled={orgName === "" || typed.trim() !== orgName.trim() || schedule.isPending}
              onClick={() => schedule.mutate()}
              className="rounded-lg bg-bad px-3 py-1.5 text-sm font-semibold text-white disabled:opacity-50">
              Schedule erasure
            </button>
            {schedule.isError && <p role="alert" className="text-xs text-bad">{schedule.error.message}</p>}
          </div>
        )}
      </Card>
    </div>
  );
}
