"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useMemo, useState, type CSSProperties } from "react";
import { z } from "zod";
import { IconCheckCircle, IconCircleDashed, IconGauge, IconRefresh, IconWarning } from "@/components/ui/icons";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorMe } from "@/hooks/use-admin";
import { api } from "@/lib/api/client";
import { cn } from "@/lib/utils";
import { PageTitle } from "./parts";
import { Btn, Pill, Segmented, type PillTone } from "./ui";

const result = z.object({
  id: z.string(), group: z.string(), name: z.string(), description: z.string(),
  status: z.enum(["ok", "warn", "fail", "skipped"]).or(z.literal("")).default(""), ms: z.number().default(0), message: z.string().default(""), at: z.string().nullable().optional(),
});
type Result = z.infer<typeof result>;
const list = z.object({ items: z.array(result) }).transform((o) => o.items);

const TONE: Record<string, { tone: PillTone; text: string }> = {
  ok: { tone: "good", text: "ok" }, warn: { tone: "warn", text: "warning" }, fail: { tone: "bad", text: "failed" }, skipped: { tone: "neutral", text: "not applicable" }, "": { tone: "neutral", text: "not run" },
};
const GROUP_ORDER = ["Core", "Security", "Services", "Data sources", "Data", "Customers"];
const GROUP_TEXT: Record<string, string> = {
  Core: "The database, the queue, the workers and storage: if one of these is down the platform is.",
  Security: "Configuration that protects customers' data.",
  Services: "Outbound services the platform uses.",
  "Data sources": "Each grid and price source answers, with the keys saved in the console.",
  Data: "The figures themselves: fresh, with an origin, and agreeing.",
  Customers: "What customers run into: failing connections and undelivered notifications.",
};

/** The platform's self-tests: run them all or one at a time, with how long each took and what it found. */
export function DiagnosticsView() {
  const catalog = useQuery({ queryKey: ["op-diagnostics"], queryFn: () => api("operator/diagnostics", list) });
  const me = useOperatorMe();
  const canRun = me.data?.role === "admin";
  const [results, setResults] = useState<Record<string, Result>>({});
  const [running, setRunning] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("all");
  const [ranAt, setRanAt] = useState<number | null>(null);
  const run = useMutation({
    mutationFn: (ids: string[]) => {
      setRunning((s) => new Set([...s, ...(ids.length ? ids : (catalog.data ?? []).map((c) => c.id))]));
      return api("operator/diagnostics/run", list, undefined, { method: "POST", body: JSON.stringify({ ids }) });
    },
    onSuccess: (items) => { setResults((r) => ({ ...r, ...Object.fromEntries(items.map((x) => [x.id, x])) })); setRanAt(Date.now()); },
    onSettled: (_d, _e, ids) => setRunning((s) => { const n = new Set(s); (ids.length ? ids : [...n]).forEach((i) => n.delete(i)); return n; }),
  });

  const rows = useMemo(() => (catalog.data ?? []).map((c) => results[c.id] ?? c), [catalog.data, results]);
  const counts = useMemo(() => {
    const c = { ok: 0, warn: 0, fail: 0, skipped: 0, pending: 0 };
    for (const r of rows) { if (r.status === "") c.pending++; else c[r.status]++; }
    return c;
  }, [rows]);
  const shown = rows.filter((r) => filter === "all" || r.status === filter || (filter === "pending" && r.status === ""));
  const groups = [...new Set(shown.map((r) => r.group))].sort((a, b) => (GROUP_ORDER.indexOf(a) + 99) % 99 - (GROUP_ORDER.indexOf(b) + 99) % 99);
  const total = rows.length;
  const health = total - counts.pending - counts.skipped > 0 ? Math.round((counts.ok / (total - counts.pending - counts.skipped)) * 100) : null;

  return (
    <div className="space-y-6">
      <PageTitle title="Diagnostics" sub="Self-tests of the platform: its database, queue, workers, storage, configuration, outbound services and its figures. They read and probe; the only thing written is one small object in storage, which is deleted again."
        action={canRun ? <Btn onClick={() => run.mutate([])} disabled={run.isPending || catalog.isLoading}><IconRefresh className={cn("size-4", run.isPending && "animate-spin")} aria-hidden />{run.isPending ? "Running…" : ranAt ? "Run all again" : "Run all tests"}</Btn> : undefined} />

      <section aria-label="Summary" className="rise relative overflow-hidden rounded-3xl bg-gradient-to-br from-leaf-soft/70 via-sky-50/60 to-violet-50/50 p-5 ring-1 ring-white/70">
        <div className="flex flex-wrap items-center gap-x-8 gap-y-4">
          <div className="flex items-center gap-4">
            <span className={cn("grid size-16 place-items-center rounded-2xl text-xl font-bold tabular-nums", health === null ? "bg-white/70 text-slate-400" : health === 100 ? "bg-leaf-soft text-leaf-deep" : health >= 70 ? "bg-amber-50 text-[#8a6218]" : "bg-rose-50 text-[#c2566a]")}>
              {health === null ? <IconGauge className="size-7" aria-hidden /> : `${health}%`}
            </span>
            <div>
              <p className="text-lg font-bold tracking-tight">{health === null ? "Not run yet" : counts.fail > 0 ? "Something needs attention" : counts.warn > 0 ? "Working, with warnings" : "All tests pass"}</p>
              <p className="text-xs text-muted">{ranAt ? `Last run ${new Date(ranAt).toLocaleTimeString()}` : "Run the tests to see the state of the platform."}</p>
            </div>
          </div>
          <dl className="flex flex-wrap gap-3">
            {([["ok", "Passed", "text-leaf-deep"], ["warn", "Warnings", "text-[#a8761f]"], ["fail", "Failed", "text-[#c2566a]"], ["skipped", "Not applicable", "text-slate-500"], ["pending", "Not run", "text-slate-400"]] as const).map(([k, label, cls]) => (
              <div key={k} className="rounded-2xl bg-white/70 px-4 py-2.5 ring-1 ring-white/80">
                <dt className="text-[11px] font-medium text-muted">{label}</dt>
                <dd className={cn("text-xl font-bold tabular-nums", cls)}>{counts[k]}</dd>
              </div>
            ))}
          </dl>
        </div>
      </section>

      <div className="overflow-x-auto">
        <Segmented label="Filter" value={filter} onChange={setFilter} options={[
          { value: "all", label: "All", count: total }, { value: "fail", label: "Failed", count: counts.fail }, { value: "warn", label: "Warnings", count: counts.warn },
          { value: "ok", label: "Passed", count: counts.ok }, { value: "skipped", label: "Not applicable", count: counts.skipped },
        ]} />
      </div>

      {catalog.isLoading ? <Skeleton className="h-64" /> : catalog.isError ? <ErrorState /> : (
        <div className="space-y-8">
          {groups.map((g, gi) => (
            <section key={g} aria-labelledby={`dg-${g}`} className="space-y-3">
              <div className="rise" style={{ "--i": Math.min(gi, 6) } as CSSProperties}>
                <h2 id={`dg-${g}`} className="text-[15px] font-semibold tracking-tight">{g}</h2>
                {GROUP_TEXT[g] && <p className="text-[13px] text-muted">{GROUP_TEXT[g]}</p>}
              </div>
              <ul className="grid gap-3 md:grid-cols-2">
                {shown.filter((r) => r.group === g).map((r, i) => {
                  const busy = running.has(r.id);
                  const t = TONE[r.status];
                  return (
                    <li key={r.id} className={cn("rise flex flex-col gap-2 rounded-2xl bg-white p-4 ring-1 transition-all duration-300 hover:shadow-md",
                      r.status === "fail" ? "ring-rose-200" : r.status === "warn" ? "ring-amber-200" : "ring-line")} style={{ "--i": Math.min(i, 10) } as CSSProperties}>
                      <div className="flex items-start gap-3">
                        <span className={cn("mt-0.5 grid size-9 shrink-0 place-items-center rounded-xl",
                          busy ? "bg-sky-50 text-sky-600" : r.status === "ok" ? "bg-leaf-soft text-leaf-deep" : r.status === "warn" ? "bg-amber-50 text-[#a8761f]" : r.status === "fail" ? "bg-rose-50 text-[#c2566a]" : "bg-slate-100 text-slate-400")}>
                          {busy ? <IconRefresh className="size-[18px] animate-spin" aria-hidden /> : r.status === "ok" ? <IconCheckCircle className="size-[18px]" aria-hidden /> : r.status === "warn" || r.status === "fail" ? <IconWarning className="size-[18px]" aria-hidden /> : <IconCircleDashed className="size-[18px]" aria-hidden />}
                        </span>
                        <div className="min-w-0 flex-1">
                          <p className="flex flex-wrap items-center gap-x-2 gap-y-1 font-semibold">{r.name}<Pill tone={busy ? "info" : t.tone}>{busy ? "running" : t.text}</Pill></p>
                          <p className="mt-0.5 text-xs text-muted">{r.description}</p>
                        </div>
                        {canRun && <Btn tone="quiet" size="sm" disabled={run.isPending} onClick={() => run.mutate([r.id])} aria-label={`Run: ${r.name}`}>Run</Btn>}
                      </div>
                      {r.message && !busy && (
                        <p className={cn("rounded-xl px-3 py-2 text-[13px] ring-1", r.status === "fail" ? "bg-rose-50/70 text-[#a8485c] ring-rose-100" : r.status === "warn" ? "bg-amber-50/70 text-[#8a6218] ring-amber-100" : "bg-slate-50 text-slate-600 ring-line/60")}>
                          {r.message}{r.ms > 0 && <span className="ml-2 text-[11px] text-muted">· {r.ms} ms</span>}
                        </p>
                      )}
                    </li>
                  );
                })}
              </ul>
            </section>
          ))}
          {groups.length === 0 && <p className="text-sm text-muted">Nothing in this filter.</p>}
        </div>
      )}
      {run.isError && <p role="alert" className="rounded-xl bg-rose-50 px-3.5 py-2.5 text-sm text-[#c2566a] ring-1 ring-rose-200">{run.error.message}</p>}
      {!canRun && <p className="text-xs text-muted">Only an operator with the admin role can run the tests.</p>}
    </div>
  );
}
