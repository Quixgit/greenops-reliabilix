"use client";

import { Activity, AlertTriangle, Clock, Layers } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorJobs } from "@/hooks/use-admin";
import { shortTime } from "@/lib/format-time";
import { PageTitle, Tile, th } from "./parts";

const age = (s: number) => (s < 1 ? "none waiting" : s < 90 ? `${Math.round(s)} s` : s < 5400 ? `${Math.round(s / 60)} min` : `${(s / 3600).toFixed(1)} h`);

/** Read-only queue telemetry. Actions (run, cancel, backfill) come later: they are only safe for idempotent jobs. */
export function JobsView() {
  const q = useOperatorJobs();
  const d = q.data;
  const sum = (f: (x: NonNullable<typeof d>["queues"][number]) => number) => d?.queues.reduce((n, x) => n + f(x), 0) ?? null;
  return (
    <div className="space-y-5">
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

        <Card index={4}>
          <CardHeader title="Queues" />
          {d.queues.length === 0 ? <EmptyState icon={Layers} text="No queue has had a task yet." /> : (
            <div className="overflow-x-auto"><table className="w-full text-sm">
              <thead><tr><th className={th}>Queue</th><th className={th}>Waiting</th><th className={th}>Running</th><th className={th}>Scheduled</th><th className={th}>Retrying</th><th className={th}>Gave up</th><th className={th}>Done today</th><th className={th}>Failed today</th><th className={th}>Oldest wait</th></tr></thead>
              <tbody className="divide-y divide-line/70">{d.queues.map((x) => (
                <tr key={x.name}><td className="py-2 pr-3 font-medium">{x.name}{x.paused && <Badge tone="warn" className="ml-2">paused</Badge>}</td>
                  <td className="pr-3">{x.pending}</td><td className="pr-3">{x.active}</td><td className="pr-3">{x.scheduled}</td>
                  <td className="pr-3">{x.retry > 0 ? <Badge tone="warn">{x.retry}</Badge> : 0}</td>
                  <td className="pr-3">{x.archived > 0 ? <Badge tone="bad">{x.archived}</Badge> : 0}</td>
                  <td className="pr-3">{x.processed_today}</td><td className="pr-3">{x.failed_today}</td>
                  <td className="text-muted">{age(x.latency_seconds)}</td></tr>
              ))}</tbody>
            </table></div>
          )}
        </Card>

        <Card index={5}>
          <CardHeader title="Failed tasks" />
          {d.failed.length === 0 ? <EmptyState icon={AlertTriangle} text="No task is waiting for a retry and none has given up." /> : (
            <ul className="divide-y divide-line/70 text-sm">{d.failed.map((f) => (
              <li key={f.id} className="space-y-1 py-3">
                <p className="flex flex-wrap items-center gap-2"><code className="text-xs font-semibold">{f.type}</code><Badge>{f.queue}</Badge>
                  <Badge tone={f.state === "archived" ? "bad" : "warn"}>{f.state === "archived" ? "gave up" : "will retry"}</Badge>
                  <span className="text-xs text-muted">attempt {f.retried}/{f.max_retry}{f.last_failed_at ? ` · failed ${shortTime(f.last_failed_at)}` : ""}{f.next_attempt_at && f.state !== "archived" ? ` · next ${shortTime(f.next_attempt_at)}` : ""}</span></p>
                <p className="break-words font-mono text-xs text-slate-700">{f.last_error || "no error text"}</p>
              </li>
            ))}</ul>
          )}
        </Card>

        <Card index={6}>
          <CardHeader title="Periodic schedules" />
          {d.schedules.length === 0 ? <EmptyState icon={Clock} text="The scheduler process has not reported any schedule. Is it running?" /> : (
            <div className="overflow-x-auto"><table className="w-full text-sm">
              <thead><tr><th className={th}>Job</th><th className={th}>Schedule</th><th className={th}>Last run</th><th className={th}>Next run</th></tr></thead>
              <tbody className="divide-y divide-line/70">{d.schedules.map((s) => (
                <tr key={s.type}><td className="py-2 pr-3 font-medium"><code className="text-xs">{s.type}</code></td><td className="pr-3"><code className="text-xs">{s.spec}</code></td>
                  <td className="pr-3 text-muted">{s.prev ? shortTime(s.prev) : "—"}</td><td className="text-muted">{shortTime(s.next)}</td></tr>
              ))}</tbody>
            </table></div>
          )}
        </Card>
      </>)}
    </div>
  );
}
