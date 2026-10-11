"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ProviderMark } from "@/components/connectors/provider-mark";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { Badge, statusTone } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Pagination, usePaged } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnection, useConnectionCosts, useProjects, useSyncRuns } from "@/hooks/use-overview";
import { useT } from "@/i18n/provider";
import { connectionState, stateTone } from "@/lib/connection-state";
import { api, apiVoid } from "@/lib/api/client";
import { formatCost } from "@/lib/formatters";
import { z } from "zod";
import type { DataQuality } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";

const QUALITY = {
  ok: { tone: "good", label: "Matches the cloud" },
  mismatch: { tone: "bad", label: "Does not match the cloud" },
  unchecked: { tone: "neutral", label: "Not checked" },
} as const;

function Quality({ q }: { q: DataQuality | null }) {
  if (!q) return <p className="text-sm text-muted">No synchronization has been checked yet.</p>;
  const m = QUALITY[q.status];
  return (
    <div className="space-y-2 text-sm">
      <p className="flex items-center gap-2"><Badge tone={m.tone}>{m.label}</Badge><span className="text-xs text-muted">checked {shortTime(q.checked_at)}</span></p>
      {q.status !== "unchecked" && (
        <p className="text-muted">
          {q.checked_days} days compared with the cloud's own totals; {q.mismatch_days} differ
          {q.mismatch_days > 0 && <> (largest difference {q.max_diff_pct.toFixed(1)}%)</>}.
        </p>
      )}
      {q.mismatched_days && q.mismatched_days.length > 0 && <p className="text-xs text-muted">Days: {q.mismatched_days.join(", ")}. They are read again on the next sync.</p>}
      {q.note && <p className="text-xs text-muted">{q.note}</p>}
    </div>
  );
}

export function AccountView({ id, canWrite }: { id: string; canWrite: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const conn = useConnection(id);
  const runs = useSyncRuns(id);
  const router = useRouter();
  const projects = useProjects();
  const costs = useConnectionCosts();
  const pg = usePaged(runs.data ?? [], 10);
  const disconnect = useMutation({
    mutationFn: () => apiVoid(`cloud-accounts/${id}`, { method: "DELETE" }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["connections"] }); router.push("/connectors"); },
  });
  const act = useMutation({
    mutationFn: (what: "sync" | "verify") => api(`cloud-accounts/${id}/${what}`, z.unknown(), undefined, { method: "POST", body: "{}" }),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ["connection", id] }); void qc.invalidateQueries({ queryKey: ["sync-runs", id] }); },
  });
  const c = conn.data?.connection;
  return (
    <div className="space-y-5">
      <Link href="/connectors" className="inline-flex items-center gap-1.5 text-sm text-muted hover:text-ink"><ArrowLeft className="size-4" />Connectors</Link>
      {conn.isLoading ? <Skeleton className="h-32" /> : conn.isError || !c ? <ErrorState text="This account could not be loaded." /> : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h1 className="flex items-center gap-3 text-2xl font-bold tracking-tight">
              <ProviderMark id={c.provider} size="lg" />
              <span><span className="uppercase">{c.provider}</span> <span className="font-medium text-muted">{c.account_ref}</span></span>
            </h1>
            {canWrite && (
              <div className="flex gap-2">
                <button disabled={act.isPending} onClick={() => act.mutate("verify")} className="rounded-lg px-3 py-1.5 text-sm font-semibold ring-1 ring-line hover:bg-slate-50 disabled:opacity-60">Test connection</button>
                <button disabled={act.isPending} onClick={() => act.mutate("sync")} className="rounded-lg bg-primary px-3 py-1.5 text-sm font-semibold text-white disabled:opacity-60">Sync now</button>
                <ConfirmButton label={t("account.disconnect")} confirmLabel={t("account.disconnect_confirm")} disabled={disconnect.isPending} onConfirm={() => disconnect.mutate()} className="px-3 py-1.5 text-sm" />
              </div>
            )}
          </div>
          {act.isError && <p role="alert" className="text-sm text-bad">{act.error.message}</p>}
          <div className="grid gap-5 lg:grid-cols-2">
            <Card index={0}>
              <CardHeader title="Connection" />
              <dl className="grid grid-cols-[130px_1fr] gap-y-2 text-sm">
                <dt className="text-muted">{t("connectors.col.project")}</dt><dd>{projects.data?.find((p) => p.id === c.project_id)?.name ?? "—"}</dd>
                <dt className="text-muted">{t("connectors.col.access")}</dt><dd>{t(`connectors.access.${c.access || "role"}`)}</dd>
                <dt className="text-muted">Status</dt><dd><Badge tone={stateTone(connectionState(c))}>{t(`connection.state.${connectionState(c)}`)}</Badge></dd>
                <dt className="text-muted">Last sync</dt><dd>{c.last_sync_at ? shortTime(c.last_sync_at) : "never"}</dd>
                <dt className="text-muted">Data read up to</dt><dd>{c.synced_through ? shortTime(c.synced_through) : "—"}</dd>
                {(() => { const m = costs.data?.find((x) => x.connection_id === c.id); return m ? <><dt className="text-muted">{t("account.cost")}</dt><dd>{formatCost(m.cost)} · {t("connectors.days_with_data", { count: m.days })}</dd></> : null; })()}
                <dt className="text-muted">Last error</dt><dd className={c.last_error ? "text-bad" : ""}>{c.last_error ?? "none"}</dd>
              </dl>
            </Card>
            <Card index={1}>
              <CardHeader title="Data quality" />
              <Quality q={c.data_quality} />
            </Card>
          </div>
          {(conn.data?.setup?.steps?.length ?? 0) > 0 && (
            <Card index={2}>
              <CardHeader title={t("account.how_to_fix")} />
              <ol className="list-decimal space-y-2 pl-5 text-sm">{conn.data!.setup!.steps!.map((st, i) => <li key={i} className="break-words">{st}</li>)}</ol>
            </Card>
          )}
          {disconnect.isError && <p role="alert" className="text-sm text-bad">{disconnect.error.message}</p>}
          <Card index={3}>
            <CardHeader title="Sync history" />
            {runs.isLoading ? <Skeleton className="h-24" /> : runs.isError ? <ErrorState /> : runs.data!.length === 0 ? <EmptyState icon={ArrowLeft} text="No synchronization has run yet." /> : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead className="text-xs text-muted"><tr><th className="py-2 pr-3 font-medium">Started</th><th className="pr-3 font-medium">Status</th><th className="pr-3 font-medium">Records</th><th className="pr-3 font-medium">Data quality</th><th className="font-medium">Error</th></tr></thead>
                  <tbody className="divide-y divide-line/70">
                    {pg.slice.map((r) => (
                      <tr key={r.id}>
                        <td className="py-2 pr-3 whitespace-nowrap">{shortTime(r.started_at)}</td>
                        <td className="pr-3"><Badge tone={statusTone(r.status)}>{r.status}</Badge></td>
                        <td className="pr-3">{r.records}</td>
                        <td className="pr-3">{r.quality ? <Badge tone={QUALITY[r.quality.status].tone}>{r.quality.status}</Badge> : <span className="text-muted">—</span>}</td>
                        <td className="max-w-xs truncate text-muted" title={r.error ?? ""}>{r.error ?? ""}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <Pagination page={pg.page} pages={pg.pages} total={pg.total} size={pg.size} onPage={pg.setPage} onSize={pg.setSize} />
              </div>
            )}
          </Card>
        </>
      )}
    </div>
  );
}
