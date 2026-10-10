"use client";

import { Activity, AlertTriangle, Clock, Layers } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardHeader } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorJobs } from "@/hooks/use-admin";
import { shortTime } from "@/lib/format-time";
import { PageTitle, Tile } from "./parts";
import { DataTable, Pill } from "./ui";

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
          <DataTable rowKey={(x) => x.name} rows={d.queues} empty="No queue has had a task yet."
            columns={[
              { key: "name", header: "Queue", sort: (a, b) => a.name.localeCompare(b.name), render: (x) => <span className="font-medium">{x.name}{x.paused && <span className="ml-2"><Pill tone="warn">paused</Pill></span>}</span> },
              { key: "pending", header: "Waiting", align: "right", sort: (a, b) => a.pending - b.pending, render: (x) => <span className="tabular-nums">{x.pending}</span> },
              { key: "active", header: "Running", align: "right", render: (x) => <span className="tabular-nums">{x.active}</span> },
              { key: "scheduled", header: "Scheduled", align: "right", hideBelow: "md", render: (x) => <span className="tabular-nums">{x.scheduled}</span> },
              { key: "retry", header: "Retrying", align: "right", sort: (a, b) => a.retry - b.retry, render: (x) => (x.retry > 0 ? <Pill tone="warn">{x.retry}</Pill> : <span className="text-muted">0</span>) },
              { key: "archived", header: "Gave up", align: "right", sort: (a, b) => a.archived - b.archived, render: (x) => (x.archived > 0 ? <Pill tone="bad">{x.archived}</Pill> : <span className="text-muted">0</span>) },
              { key: "done", header: "Done today", align: "right", hideBelow: "lg", render: (x) => <span className="tabular-nums">{x.processed_today}</span> },
              { key: "failed", header: "Failed today", align: "right", hideBelow: "lg", sort: (a, b) => a.failed_today - b.failed_today, render: (x) => <span className={x.failed_today ? "font-medium text-bad tabular-nums" : "tabular-nums text-muted"}>{x.failed_today}</span> },
              { key: "wait", header: "Oldest wait", hideBelow: "md", render: (x) => <span className="text-muted">{age(x.latency_seconds)}</span> },
            ]} />
        </Card>

        <Card index={5}>
          <CardHeader title="Failed tasks" />
          <DataTable rowKey={(f) => f.id} rows={d.failed} empty="No task is waiting for a retry and none has given up."
            columns={[
              { key: "type", header: "Task", render: (f) => <code className="text-xs font-semibold">{f.type}</code> },
              { key: "queue", header: "Queue", hideBelow: "md", render: (f) => <span className="text-muted">{f.queue}</span> },
              { key: "state", header: "State", sort: (a, b) => a.state.localeCompare(b.state), render: (f) => <Pill tone={f.state === "archived" ? "bad" : "warn"}>{f.state === "archived" ? "gave up" : "will retry"}</Pill> },
              { key: "try", header: "Attempt", align: "right", render: (f) => <span className="tabular-nums">{f.retried}/{f.max_retry}</span> },
              { key: "at", header: "Failed", hideBelow: "lg", sort: (a, b) => (a.last_failed_at ?? "").localeCompare(b.last_failed_at ?? ""), render: (f) => <span className="whitespace-nowrap text-muted">{f.last_failed_at ? shortTime(f.last_failed_at) : "—"}</span> },
              { key: "err", header: "Last error", render: (f) => <span className="line-clamp-2 break-words font-mono text-xs text-slate-700">{f.last_error || "no error text"}</span> },
            ]} />
        </Card>

        <Card index={6}>
          <CardHeader title="Periodic schedules" />
          <DataTable rowKey={(x) => x.type} rows={d.schedules} empty="The scheduler process has not reported any schedule. Is it running?"
            columns={[
              { key: "type", header: "Job", sort: (a, b) => a.type.localeCompare(b.type), render: (x) => <code className="text-xs font-semibold">{x.type}</code> },
              { key: "spec", header: "Schedule", render: (x) => <code className="text-xs">{x.spec}</code> },
              { key: "prev", header: "Last run", render: (x) => <span className="text-muted">{x.prev ? shortTime(x.prev) : "—"}</span> },
              { key: "next", header: "Next run", sort: (a, b) => a.next.localeCompare(b.next), render: (x) => <span className="text-muted">{shortTime(x.next)}</span> },
            ]} />
        </Card>
      </>)}
    </div>
  );
}
