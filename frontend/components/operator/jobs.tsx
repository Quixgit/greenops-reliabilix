"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { IconClock as Clock, IconDatabase as Layers, IconRefresh, IconTimer, IconTrash, IconTrend as Activity, IconWarning as AlertTriangle } from "@/components/ui/icons";
import { Card, CardHeader } from "@/components/ui/card";
import { ConfirmButton } from "@/components/ui/confirm-button";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { useState } from "react";
import { useOperatorJobs, useOperatorMe } from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { changed } from "@/lib/api/schemas";
import { shortTime } from "@/lib/format-time";
import { cn } from "@/lib/utils";
import { taskName, cronWords, relative } from "./jobs-meta";
import { PageTitle, Tile } from "./parts";
import { Btn, EmptyPanel, Pill, RowList } from "./ui";

const age = (s: number) => (s < 1 ? "none waiting" : s < 90 ? `${Math.round(s)} s` : s < 5400 ? `${Math.round(s / 60)} min` : `${(s / 3600).toFixed(1)} h`);

const SEGMENTS = [["pending", "Waiting", "bg-pastel-blue"], ["active", "Running", "bg-leaf"], ["scheduled", "Scheduled", "bg-pastel-violet"], ["retry", "Retrying", "bg-pastel-amber"], ["archived", "Gave up", "bg-pastel-rose"]] as const;

/** Queues, the tasks that failed (with what to do about them) and the periodic schedule, read from the job queue every 15 seconds. */
export function JobsView() {
  const q = useOperatorJobs();
  const me = useOperatorMe();
  const qc = useQueryClient();
  const canAct = me.data?.role === "admin";
  const [tab, setTab] = useState("failed");
  const d = q.data;
  const sum = (f: (x: NonNullable<typeof d>["queues"][number]) => number) => d?.queues.reduce((n, x) => n + f(x), 0) ?? null;
  const act = useMutation({
    mutationFn: (v: { queue: string; id: string; verb: "retry" | "delete" }) => api(`operator/jobs/${v.queue}/${v.id}/${v.verb}`, changed, undefined, { method: "POST" }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["op-jobs"] }),
  });
  const failed = d?.failed ?? [];
  return (
    <div className="space-y-6">
      <PageTitle title="Jobs & scheduler" sub="Queues, periodic schedules and tasks that failed, read from the job queue every 15 seconds. Task payloads are never shown: they carry organization ids." />
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError || !d ? (
        <ErrorState text="The job queue cannot be read. Check that Redis is reachable from the API." />
      ) : (<>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <Tile index={0} label="Waiting" value={sum((x) => x.pending)} icon={Layers} color="sky" />
          <Tile index={1} label="Running now" value={sum((x) => x.active)} icon={Activity} color="sage" />
          <Tile index={2} label="Retrying" value={sum((x) => x.retry)} icon={Clock} color="butter" tone={sum((x) => x.retry) ? "warn" : undefined} />
          <Tile index={3} label="Gave up" value={sum((x) => x.archived)} icon={AlertTriangle} color="peach" tone={sum((x) => x.archived) ? "bad" : undefined} />
        </div>

        <div className={cn("grid gap-4", d.queues.length > 1 && "lg:grid-cols-2")}>
          {d.queues.map((x, i) => {
            const total = SEGMENTS.reduce((n, [k]) => n + x[k], 0);
            return (
              <Card key={x.name} index={i} interactive={false}>
                <CardHeader title={<span className="flex items-center gap-2">{x.name}{x.paused && <Pill tone="warn">paused</Pill>}</span>} icon={<Layers className="size-4 text-leaf-deep" />} action={<span className="text-xs text-muted">oldest wait: {age(x.latency_seconds)}</span>} />
                <div role="img" aria-label="Tasks by state" className="flex h-3 overflow-hidden rounded-full bg-slate-100">
                  {total === 0 ? <span className="h-full w-full bg-leaf-soft" /> : SEGMENTS.map(([k, , c]) => x[k] > 0 && <span key={k} className={cn("grow-x h-full", c)} style={{ width: `${(x[k] / total) * 100}%` }} />)}
                </div>
                <ul className="mt-3 flex flex-wrap gap-x-5 gap-y-1.5 text-sm">
                  {SEGMENTS.map(([k, label, c]) => <li key={k} className="flex items-center gap-2"><span className={cn("size-2 rounded-full", c)} /><span className="text-muted">{label}</span><b className="tabular-nums">{x[k]}</b></li>)}
                </ul>
                <p className="mt-3 border-t border-line/70 pt-3 text-sm text-muted">Today: <b className="text-ink tabular-nums">{x.processed_today.toLocaleString()}</b> done · <b className={cn("tabular-nums", x.failed_today ? "text-[#d9707f]" : "text-ink")}>{x.failed_today}</b> failed</p>
              </Card>
            );
          })}
        </div>

        <PillTabs value={tab} onChange={setTab} label="Jobs sections" tabs={[{ value: "failed", label: `Failed tasks${failed.length ? ` (${failed.length})` : ""}` }, { value: "schedule", label: `Periodic schedule (${d.schedules.length})` }]} />

        <div key={tab} className="page-enter">
          {tab === "failed" ? (<>
            <RowList rows={failed} rowKey={(f) => f.id} pageSize={6}
              empty={<EmptyPanel title="Nothing has failed" text="No task is waiting for a retry and none has given up." />}
              row={(f) => {
                const n = taskName(f.type);
                const gaveUp = f.state === "archived";
                return (
                  <div className="group flex flex-wrap items-center gap-x-4 gap-y-2 rounded-2xl px-3 py-3.5 transition-colors hover:bg-slate-50">
                    <AlertTriangle className={cn("size-10 shrink-0", gaveUp ? "text-[#d9707f]" : "text-[#d9a441]")} strokeWidth={1.4} />
                    <div className="min-w-0 flex-1 basis-72">
                      <p className="flex flex-wrap items-center gap-2 font-semibold">{n.title}<Pill tone={gaveUp ? "bad" : "warn"}>{gaveUp ? "gave up" : "will retry"}</Pill>{f.last_error.startsWith("skip retry") && <Pill>not retried: it cannot succeed as it is</Pill>}
                        <span className="text-xs font-normal text-muted tabular-nums">attempt {f.retried}/{f.max_retry}</span></p>
                      <p className="mt-0.5 text-xs text-muted">{n.what && <>{n.what} · </>}<code>{f.type}</code> · {f.queue}{f.last_failed_at && <> · failed {relative(f.last_failed_at)} ({shortTime(f.last_failed_at)})</>}{f.next_attempt_at && !gaveUp && <> · next try {relative(f.next_attempt_at)}</>}</p>
                      <p className="mt-1.5 line-clamp-3 break-words rounded-lg bg-slate-50 px-3 py-2 font-mono text-xs text-slate-700">{(f.last_error || "no error text").replace(/^skip retry for the task\n?/, "")}</p>
                    </div>
                    {canAct && (
                      <div className="flex items-center gap-2">
                        <Btn tone="quiet" size="sm" disabled={act.isPending} onClick={() => act.mutate({ queue: f.queue, id: f.id, verb: "retry" })}><IconRefresh className="size-3.5" />Run again</Btn>
                        <ConfirmButton label="Remove" confirmLabel="Remove for good" disabled={act.isPending} onConfirm={() => act.mutate({ queue: f.queue, id: f.id, verb: "delete" })} className="!rounded-xl" />
                      </div>
                    )}
                  </div>
                );
              }} />
            {act.error && <p role="alert" className="rise mt-3 rounded-xl bg-rose-50 px-3.5 py-2.5 text-sm text-[#c2566a] ring-1 ring-rose-200">{act.error.message}</p>}
            {failed.length > 0 && <p className="mt-3 text-xs text-muted">Running a task again puts it back in the queue now. Removing it only drops this record; the periodic schedule starts the job again when it is due.</p>}
          </>) : (
            <RowList rows={[...d.schedules].sort((a, b) => a.next.localeCompare(b.next))} rowKey={(x) => x.type} pageSize={8}
              empty={<EmptyPanel tone="quiet" icon={IconTimer} title="No schedule reported" text="The scheduler process has not reported any schedule. Is it running?" />}
              row={(x) => {
                const n = taskName(x.type);
                return (
                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-2xl px-3 py-3 transition-colors hover:bg-slate-50">
                    <IconTimer className="size-10 shrink-0 text-[#5b8fdc]" strokeWidth={1.4} />
                    <div className="min-w-0 flex-1 basis-64">
                      <p className="font-semibold">{n.title}</p>
                      <p className="mt-0.5 text-xs text-muted">{n.what && <>{n.what} · </>}<code>{x.type}</code></p>
                    </div>
                    <div className="text-sm"><p className="font-medium">{cronWords(x.spec)}</p><p className="text-xs text-muted"><code>{x.spec}</code></p></div>
                    <div className="min-w-36 text-right text-sm"><p className="font-medium text-leaf-deep">next {relative(x.next)}</p><p className="text-xs text-muted">{x.prev ? `last ${relative(x.prev)}` : "not run since the scheduler started"}</p></div>
                  </div>
                );
              }} />
          )}
        </div>
      </>)}
    </div>
  );
}
