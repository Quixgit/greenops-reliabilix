"use client";

import { useMemo, useState, type CSSProperties } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { CountUp } from "@/components/ui/count-up";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorMetrics, type MetricDay } from "@/hooks/use-operator-extra";
import { cn } from "@/lib/utils";
import { PageTitle } from "./parts";

type Key = Exclude<keyof MetricDay, "day">;
const SERIES: { key: Key; label: string; hint: string; color: string; kind: "bars" | "line" }[] = [
  { key: "organizations", label: "New organizations", hint: "Organizations created per day.", color: "#6fc3a8", kind: "bars" },
  { key: "users", label: "New users", hint: "People who signed in for the first time.", color: "#8db4ee", kind: "bars" },
  { key: "projects", label: "Projects created", hint: "Projects created by customers per day.", color: "#b8a9ee", kind: "bars" },
  { key: "connections", label: "Cloud connections added", hint: "Cloud accounts connected per day.", color: "#f2cf8a", kind: "bars" },
  { key: "reports", label: "Reports requested", hint: "Reports requested per day.", color: "#6fc3a8", kind: "bars" },
  { key: "gate_checks", label: "CI gate checks", hint: "Calls of the CI gate per day.", color: "#8db4ee", kind: "bars" },
  { key: "recommendations", label: "Recommendations created", hint: "Recommendations generated per day.", color: "#b8a9ee", kind: "bars" },
  { key: "active_organizations", label: "Active organizations", hint: "Organizations with at least one recorded action that day.", color: "#2f8f78", kind: "line" },
  { key: "deliveries", label: "Notification deliveries", hint: "Messages queued per day; the failed ones are in rose.", color: "#8db4ee", kind: "bars" },
  { key: "errors", label: "New platform errors", hint: "Error groups first seen per day.", color: "#eba7b3", kind: "bars" },
];
const dayLabel = (d: string) => new Date(`${d.slice(0, 10)}T00:00:00Z`).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" });

/** One series as bars (or a line) with a value on hover; two numbers on top: the sum over the window and the last day. */
function SeriesCard({ s, rows, index, wide }: { s: (typeof SERIES)[number]; rows: MetricDay[]; index: number; wide?: boolean }) {
  const [hover, setHover] = useState<number | null>(null);
  const vals = rows.map((r) => Number(r[s.key]));
  const failed = s.key === "deliveries" ? rows.map((r) => r.deliveries_failed) : null;
  const max = Math.max(1, ...vals);
  const sum = vals.reduce((a, b) => a + b, 0);
  const W = 320, H = 96, gap = 2;
  const bw = Math.max(2, (W - gap * (vals.length - 1)) / Math.max(1, vals.length));
  const x = (i: number) => i * (bw + gap);
  const y = (v: number) => H - 4 - (v / max) * (H - 12);
  const line = vals.map((v, i) => `${i ? "L" : "M"}${(x(i) + bw / 2).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  return (
    <Card index={index} interactive={false} className={cn(wide && "sm:col-span-2")}>
      <p className="text-[13px] font-medium text-slate-600" title={s.hint}>{s.label}</p>
      <p className="mt-0.5 flex items-baseline gap-2">
        <span className="text-2xl font-bold tabular-nums tracking-tight"><CountUp value={s.kind === "line" ? Math.max(0, ...vals) : sum} format={(n) => Math.round(n).toLocaleString()} /></span>
        <span className="text-xs text-muted">{s.kind === "line" ? "peak" : `in ${rows.length} days`}</span>
      </p>
      <div className="relative mt-3" onMouseLeave={() => setHover(null)}>
        <svg viewBox={`0 0 ${W} ${H}`} className="block h-24 w-full" preserveAspectRatio="none" role="img" aria-label={s.label}>
          {s.kind === "bars" ? vals.map((v, i) => (
            <g key={i} onMouseEnter={() => setHover(i)}>
              <rect x={x(i)} y={0} width={bw} height={H} fill="transparent" />
              <rect x={x(i)} y={y(v)} width={bw} height={Math.max(1.5, H - 4 - y(v))} rx="1.5" fill={s.color} opacity={hover === null || hover === i ? 1 : 0.45} className="bar-rise" style={{ transformOrigin: "50% 100%", animationDelay: `${i * 12}ms` } as CSSProperties} />
              {failed && failed[i] > 0 && <rect x={x(i)} y={y(failed[i])} width={bw} height={Math.max(1.5, H - 4 - y(failed[i]))} rx="1.5" fill="#e08b98" />}
            </g>
          )) : (
            <>
              <path d={`${line} L${x(vals.length - 1) + bw / 2},${H} L${bw / 2},${H} Z`} fill={s.color} opacity=".14" />
              <path d={line} fill="none" stroke={s.color} strokeWidth="2" strokeLinejoin="round" strokeLinecap="round" vectorEffect="non-scaling-stroke" />
              {vals.map((_, i) => <rect key={i} x={x(i)} y={0} width={bw} height={H} fill="transparent" onMouseEnter={() => setHover(i)} />)}
            </>
          )}
        </svg>
        {hover !== null && (
          <div className="pointer-events-none absolute -top-1 z-10 -translate-y-full rounded-lg border border-line bg-white px-2.5 py-1.5 text-xs shadow-lg"
            style={{ left: `${Math.min(86, Math.max(0, (hover / Math.max(1, vals.length - 1)) * 100))}%` }}>
            <p className="font-semibold">{dayLabel(rows[hover].day)}</p>
            <p className="text-slate-600">{vals[hover].toLocaleString()}{failed && failed[hover] > 0 ? ` · ${failed[hover]} failed` : ""}</p>
          </div>
        )}
      </div>
      <div className="mt-1 flex justify-between text-[10px] text-slate-400"><span>{dayLabel(rows[0].day)}</span><span>{dayLabel(rows[rows.length - 1].day)}</span></div>
    </Card>
  );
}

const PLAN_COLOR: Record<string, string> = { free: "#b9c4d6", team: "#6fc3a8", business: "#8db4ee", enterprise: "#b8a9ee" };

/** The platform's size now and how it grew: totals, a chart per kind of thing created, and the plans the organizations are on. */
export function MetricsBoard({ compact = false }: { compact?: boolean }) {
  const [days, setDays] = useState(30);
  const q = useOperatorMetrics(compact ? 30 : days);
  const m = q.data;
  const rows = m?.series ?? [];
  const shown = useMemo(() => (compact ? SERIES.filter((s) => ["organizations", "active_organizations", "gate_checks", "errors"].includes(s.key)) : SERIES), [compact]);
  const totals = m ? [
    ["Organizations", m.totals.organizations], ["Users", m.totals.users], ["Projects", m.totals.projects], ["Cloud connections", m.totals.connections],
    ["Reports", m.totals.reports], ["Recommendations", m.totals.recommendations], ["Open errors", m.totals.open_errors],
  ] as const : [];
  const planTotal = Math.max(1, ...(m?.plans.map((p) => p.organizations) ?? [1]));
  return (
    <div className="space-y-5">
      {!compact && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <PageTitle title="Platform metrics" sub="What exists, what was created and when. Counts only: nothing here shows a customer's data." />
          <div role="tablist" className="flex gap-1 rounded-xl bg-slate-100 p-1 text-sm">
            {[7, 30, 90].map((d) => <button key={d} role="tab" type="button" aria-selected={days === d} onClick={() => setDays(d)} className={cn("rounded-lg px-3 py-1.5 font-medium transition-all", days === d ? "bg-white text-ink shadow-sm" : "text-slate-500 hover:text-ink")}>{d} days</button>)}
          </div>
        </div>
      )}
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError || !m ? <ErrorState /> : (
        <>
          {!compact && (
            <div className="grid gap-3 sm:grid-cols-3 xl:grid-cols-7">
              {totals.map(([label, v], i) => (
                <Card key={label} index={i} interactive={false} className="p-4">
                  <p className="text-xs font-medium text-muted">{label}</p>
                  <p className="mt-1 text-2xl font-bold tabular-nums tracking-tight"><CountUp value={v} format={(n) => Math.round(n).toLocaleString()} /></p>
                </Card>
              ))}
            </div>
          )}
          <div className={cn("grid gap-4 sm:grid-cols-2", compact ? "xl:grid-cols-4" : "xl:grid-cols-3")}>
            {shown.map((s, i) => <SeriesCard key={s.key} s={s} rows={rows} index={i} />)}
          </div>
          {!compact && (
            <div className="grid gap-4 lg:grid-cols-2">
              <Card index={1} interactive={false}>
                <CardHeader title="Organizations by plan" />
                <ul className="space-y-3">
                  {m.plans.map((p, i) => (
                    <li key={p.plan}>
                      <div className="flex justify-between text-sm"><span className="font-medium capitalize">{p.plan}</span><span className="tabular-nums text-muted">{p.organizations}</span></div>
                      <span className="mt-1 block h-2 overflow-hidden rounded-full bg-slate-100"><span className="bar-x block h-full rounded-full" style={{ width: `${Math.max(3, (p.organizations / planTotal) * 100)}%`, background: PLAN_COLOR[p.plan] ?? "#8db4ee", "--d": `${i * 70}ms` } as CSSProperties} /></span>
                    </li>
                  ))}
                </ul>
              </Card>
              <Card index={2} interactive={false}>
                <CardHeader title="Subscriptions" />
                {m.subscriptions.length === 0 ? <p className="text-sm text-muted">No subscription is recorded yet: every organization is on the plan it was given by default.</p> : (
                  <ul className="flex flex-wrap gap-2">
                    {m.subscriptions.map((s) => <li key={s.status} className="rounded-xl bg-slate-50 px-4 py-3 text-sm"><span className="block text-xs capitalize text-muted">{s.status.replace("_", " ")}</span><span className="text-xl font-bold tabular-nums">{s.organizations}</span></li>)}
                  </ul>
                )}
                <p className="mt-3 text-xs text-muted">{m.totals.pending_erasures > 0 ? `${m.totals.pending_erasures} organization(s) wait for a scheduled erasure.` : "No organization is waiting for erasure."}</p>
              </Card>
            </div>
          )}
        </>
      )}
    </div>
  );
}

export function MetricsView() { return <MetricsBoard />; }
