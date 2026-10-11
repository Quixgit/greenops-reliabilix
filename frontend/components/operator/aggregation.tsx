"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { z } from "zod";
import { ProviderMark } from "@/components/connectors/provider-mark";
import { IconArrowDown as ArrowDown, IconArrowUp as ArrowUp, IconCheckCircle as CheckCircle2, IconDatabase as Database, IconGauge as Gauge, IconInfo as Info, IconRefresh as Refresh, IconWarning as AlertTriangle } from "@/components/ui/icons";
import { useEffect, useMemo, useState, type CSSProperties, type ReactNode } from "react";
import { Card, CardHeader } from "@/components/ui/card";
import { DateField } from "@/components/ui/date-field";
import { ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { useOperatorMe, useOperatorRegionSources } from "@/hooks/use-admin";
import { useAggregation, useExplain, type AggSource, type Aggregation, type ExplainSource } from "@/hooks/use-operator-extra";
import { api } from "@/lib/api/client";
import { shortTime } from "@/lib/format-time";
import { cn } from "@/lib/utils";
import { PageTitle, Tile } from "./parts";
import { DivergenceTab, SourcesTab } from "./source-settings";
import { Btn, DataTable, inputCls, Pill, Select, Switch, type Column, type PillTone } from "./ui";

const LABEL: Record<string, string> = { electricitymaps: "Electricity Maps", uk_carbon_intensity: "UK Carbon Intensity", eia: "EIA (US generation mix)", energy_charts: "Energy-Charts", ieso: "IESO (Ontario)", hydroquebec: "Hydro-Québec", entsoe: "ENTSO-E (Europe)", ons: "ONS (Brazil)", ember: "Ember" };
const label = (id: string) => LABEL[id] ?? id;
const FRESH: Record<AggSource["freshness"], { tone: PillTone; text: string }> = {
  fresh: { tone: "good", text: "fresh" }, aging: { tone: "warn", text: "aging" }, stale: { tone: "bad", text: "stale" }, none: { tone: "neutral", text: "no data" },
};
// One tone per basis, from the most to the least precise: green for a day's own readings, rose for an estimate.
const BASIS: Record<string, { text: string; hint: string; bar: string; tone: PillTone }> = {
  day_mean: { text: "Day's own readings", hint: "Mean of enough hourly readings of that day", bar: "bg-leaf", tone: "good" },
  month_mean: { text: "Monthly figure", hint: "The source's published monthly average", bar: "bg-pastel-blue", tone: "info" },
  partial_day_mean: { text: "Few readings", hint: "Fewer readings than the policy wants, nobody had more", bar: "bg-pastel-amber", tone: "warn" },
  latest_before: { text: "Last reading before", hint: "No readings that day; the last one within the accepted age", bar: "bg-pastel-violet", tone: "violet" },
  latest_overall: { text: "Estimate", hint: "The latest reading that exists, however old", bar: "bg-pastel-rose", tone: "bad" },
  live_provider: { text: "Asked of the provider", hint: "Nothing stored, so the provider was asked", bar: "bg-slate-300", tone: "neutral" },
  "": { text: "Not recorded", hint: "Calculated before the origin was recorded", bar: "bg-slate-200", tone: "neutral" },
};
const basis = (b: string) => BASIS[b] ?? { text: b, hint: "", bar: "bg-slate-300", tone: "neutral" as PillTone };
const ago = (iso: string | null) => {
  if (!iso) return "never";
  const h = (Date.now() - new Date(iso).getTime()) / 3_600_000;
  return h < 1 ? "under an hour ago" : h < 48 ? `${Math.round(h)} h ago` : `${Math.round(h / 24)} days ago`;
};

function useCanAct() { return useOperatorMe().data?.role === "admin"; }
const invalidate = (qc: ReturnType<typeof useQueryClient>) => { for (const k of ["op-aggregation", "op-region-sources", "op-settings", "settings", "op-explain"]) void qc.invalidateQueries({ queryKey: [k] }); };
const Err = ({ e }: { e: Error | null }) => (e ? <p role="alert" className="rise mt-3 rounded-xl bg-rose-50 px-3.5 py-2.5 text-sm text-[#c2566a] ring-1 ring-rose-200">{e.message}</p> : null);
const Note = ({ children }: { children: ReactNode }) => <p role="status" className="rise mt-3 rounded-xl bg-leaf-soft/60 px-3.5 py-2.5 text-sm text-slate-700 ring-1 ring-leaf/15">{children}</p>;

// ---- Policy -------------------------------------------------------------------------------------------------------------

function Stepper({ value, onChange, min, max, unit, label: aria, disabled }: { value: number; onChange: (n: number) => void; min: number; max: number; unit: string; label: string; disabled?: boolean }) {
  const set = (n: number) => onChange(Math.min(max, Math.max(min, Math.round(n) || min)));
  return (
    <div className="inline-flex items-center gap-2">
      <Btn tone="quiet" className="size-10 px-0" aria-label={`${aria}: less`} disabled={disabled || value <= min} onClick={() => set(value - 1)}>−</Btn>
      <input type="number" inputMode="numeric" aria-label={aria} value={value} min={min} max={max} disabled={disabled} onChange={(e) => set(Number(e.target.value))} className={cn(inputCls, "w-20 px-2 text-center tabular-nums")} />
      <Btn tone="quiet" className="size-10 px-0" aria-label={`${aria}: more`} disabled={disabled || value >= max} onClick={() => set(value + 1)}>+</Btn>
      <span className="text-sm text-muted">{unit}</span>
    </div>
  );
}

function SourceRow({ s, index, total, enabled, age, onAge, onMove, onToggle, canAct, common }: {
  s: AggSource; index: number; total: number; enabled: boolean; age: string; onAge: (v: string) => void; onMove: (d: -1 | 1) => void; onToggle: (v: boolean) => void; canAct: boolean; common: number;
}) {
  const f = FRESH[s.freshness];
  return (
    <li className={cn("rise flex flex-wrap items-center gap-x-4 gap-y-3 rounded-2xl p-3.5 ring-1 transition-all duration-300", enabled ? "bg-white ring-line hover:shadow-sm" : "bg-slate-50/70 opacity-75 ring-line/60")} style={{ "--i": Math.min(index, 8) } as CSSProperties}>
      <span className={cn("grid size-8 shrink-0 place-items-center rounded-full text-sm font-bold tabular-nums", enabled ? "bg-leaf-soft text-leaf-deep" : "bg-slate-100 text-slate-400")}>{enabled ? index + 1 : "–"}</span>
      <ProviderMark id={s.id} size="sm" dim={!enabled} />
      <div className="min-w-0 flex-1 basis-56">
        <p className="flex flex-wrap items-center gap-x-2 gap-y-1 font-semibold">{label(s.id)}
          <Pill tone="neutral">{s.granularity}</Pill>{s.computed && <Pill tone="violet">computed</Pill>}
        </p>
        <p className="mt-0.5 text-xs text-muted">{s.licence}</p>
      </div>
      <div className="flex min-w-36 flex-col items-start gap-1 text-xs text-muted">
        <Pill tone={f.tone} pulse={s.freshness === "fresh"}>{f.text}</Pill>
        <span>{s.newest ? `newest ${ago(s.newest)}` : "nothing stored"} · {s.regions} region{s.regions === 1 ? "" : "s"}{s.pinned_regions ? ` · pinned in ${s.pinned_regions}` : ""}</span>
      </div>
      {enabled && (
        <label className="flex items-center gap-2 text-xs text-muted">Accepted age
          <input value={age} onChange={(e) => onAge(e.target.value.replace(/\D/g, "").slice(0, 4))} disabled={!canAct} inputMode="numeric" placeholder={String(common)} aria-label={`Accepted age of ${label(s.id)} in hours`} className={cn(inputCls, "h-9 w-20 px-2 text-center tabular-nums")} />h
        </label>
      )}
      <div className="ml-auto flex items-center gap-1.5">
        {enabled && <>
          <Btn tone="quiet" className="size-9 px-0" aria-label={`Move ${label(s.id)} up`} disabled={!canAct || index === 0} onClick={() => onMove(-1)}><ArrowUp className="size-4" /></Btn>
          <Btn tone="quiet" className="size-9 px-0" aria-label={`Move ${label(s.id)} down`} disabled={!canAct || index === total - 1} onClick={() => onMove(1)}><ArrowDown className="size-4" /></Btn>
        </>}
        <Switch label={`Use ${label(s.id)}`} checked={enabled} disabled={!canAct || (enabled && total === 1)} onChange={onToggle} />
      </div>
    </li>
  );
}

function PolicyTab({ data }: { data: Aggregation }) {
  const qc = useQueryClient();
  const canAct = useCanAct();
  const p = data.policy;
  const initial = useMemo(() => ({
    order: p.priority.filter((id) => data.sources.some((s) => s.id === id)),
    ages: Object.fromEntries(data.sources.map((s) => [s.id, p.source_max_age_hours[s.id] ? String(p.source_max_age_hours[s.id]) : ""])) as Record<string, string>,
    min: p.min_day_readings, max: p.max_age_hours,
  }), [data, p]);
  const [order, setOrder] = useState(initial.order);
  const [ages, setAges] = useState(initial.ages);
  const [min, setMin] = useState(initial.min);
  const [max, setMax] = useState(initial.max);
  useEffect(() => { setOrder(initial.order); setAges(initial.ages); setMin(initial.min); setMax(initial.max); }, [initial]);
  const dirty = order.join() !== initial.order.join() || min !== initial.min || max !== initial.max || Object.keys(ages).some((k) => ages[k] !== initial.ages[k]);
  const save = useMutation({
    mutationFn: () => api("operator/settings", z.unknown(), undefined, { method: "PATCH", body: JSON.stringify({ values: {
      "carbon.provider_priority": order,
      "carbon.source_max_age_hours": order.filter((id) => ages[id]).map((id) => `${id}=${ages[id]}`),
      "carbon.min_day_readings": min, "carbon.max_reading_age_hours": max,
    } }) }),
    onSuccess: () => invalidate(qc),
  });
  const byId = new Map(data.sources.map((s) => [s.id, s]));
  const off = data.sources.filter((s) => !order.includes(s.id));
  const move = (i: number, d: -1 | 1) => setOrder((o) => { const n = [...o]; [n[i], n[i + d]] = [n[i + d], n[i]]; return n; });
  return (
    <div className="space-y-5">
      <Card index={0} interactive={false}>
        <CardHeader title="Priority of sources" icon={<Database className="size-4 text-leaf-deep" />} info="For every region and day the first source in this order that has enough readings is used. Sources are never averaged together." />
        <ol className="space-y-2.5">
          {order.map((id, i) => <SourceRow key={id} s={byId.get(id)!} index={i} total={order.length} enabled age={ages[id] ?? ""} common={max} canAct={canAct}
            onAge={(v) => setAges((a) => ({ ...a, [id]: v }))} onMove={(d) => move(i, d)} onToggle={() => setOrder((o) => o.filter((x) => x !== id))} />)}
        </ol>
        {off.length > 0 && <>
          <p className="mb-2 mt-5 text-xs font-semibold uppercase tracking-wide text-slate-400">Not used</p>
          <ol className="space-y-2.5">
            {off.map((s, i) => <SourceRow key={s.id} s={s} index={i} total={order.length} enabled={false} age="" common={max} canAct={canAct} onAge={() => undefined} onMove={() => undefined} onToggle={() => setOrder((o) => [...o, s.id])} />)}
          </ol>
        </>}
      </Card>

      <Card index={1} interactive={false}>
        <CardHeader title="Thresholds" icon={<Gauge className="size-4 text-leaf-deep" />} />
        <div className="grid gap-5 md:grid-cols-2">
          <div>
            <p className="font-medium">Readings a source needs per day</p>
            <p className="mb-2.5 mt-0.5 text-sm text-muted">A source&apos;s mean counts as the day&apos;s intensity only with at least this many hourly readings that day. Fewer is used only when no source has enough, and is marked partial.</p>
            <Stepper label="Readings per day" value={min} onChange={setMin} min={1} max={24} unit="readings" disabled={!canAct} />
          </div>
          <div>
            <p className="font-medium">Oldest reading used for a day without data</p>
            <p className="mb-2.5 mt-0.5 text-sm text-muted">The last reading before such a day is used if it is not older than this. A source can have its own limit in the list above; an older reading is only ever a marked estimate.</p>
            <Stepper label="Accepted age" value={max} onChange={setMax} min={1} max={720} unit="hours" disabled={!canAct} />
          </div>
        </div>
        <div className="mt-5 flex flex-wrap items-center gap-3 border-t border-line/70 pt-4">
          <Btn disabled={!canAct || !dirty || save.isPending} onClick={() => save.mutate()}>{save.isPending ? "Saving…" : "Save policy"}</Btn>
          {dirty && <Btn tone="quiet" onClick={() => { setOrder(initial.order); setAges(initial.ages); setMin(initial.min); setMax(initial.max); }}>Discard</Btn>}
          {dirty && <span className="text-xs text-[#a8761f]">Unsaved changes</span>}
          {save.isSuccess && !dirty && <span role="status" className="rise inline-flex items-center gap-1.5 text-sm text-leaf-deep"><CheckCircle2 className="size-4" />Saved. It applies to the next calculation; recalculate to apply it to history.</span>}
          {!canAct && <span className="text-xs text-muted">Only an operator with the admin role can change the policy.</span>}
        </div>
        <Err e={save.error} />
      </Card>
    </div>
  );
}

// ---- Regions ------------------------------------------------------------------------------------------------------------

function RegionsTab({ data }: { data: Aggregation }) {
  const qc = useQueryClient();
  const canAct = useCanAct();
  const q = useOperatorRegionSources();
  const rows = q.data?.items ?? [];
  const save = useMutation({
    mutationFn: (overrides: string[]) => api("operator/settings", z.unknown(), undefined, { method: "PATCH", body: JSON.stringify({ values: { "carbon.region_provider_overrides": overrides } }) }),
    onSuccess: () => invalidate(qc),
  });
  const pin = (region: string, source: string) => {
    const next = rows.filter((r) => r.region !== region && r.override).map((r) => `${r.region}=${r.override}`);
    if (source) next.push(`${region}=${source}`);
    save.mutate(next);
  };
  const options = [{ value: "", label: "Automatic" }, ...data.policy.priority.map((id) => ({ value: id, label: label(id) }))];
  type R = (typeof rows)[number];
  const columns: Column<R>[] = [
    { key: "region", header: "Region", sort: (a, b) => a.region.localeCompare(b.region), render: (r) => <span className="font-semibold">{r.region}</span> },
    { key: "used", header: "Used now", sort: (a, b) => a.effective.localeCompare(b.effective),
      render: (r) => (r.effective ? <span className="inline-flex items-center gap-2"><ProviderMark id={r.effective} size="sm" /><span>{label(r.effective)}</span></span> : <Pill tone="warn">no data</Pill>) },
    { key: "have", header: "What each source has", render: (r) => (
      <span className="flex flex-wrap gap-1.5">{r.sources.length === 0 ? <span className="text-xs text-muted">nothing stored</span> : r.sources.map((s) => (
        <span key={s.provider} title={s.newest ? `newest ${shortTime(s.newest)}` : undefined} className="inline-flex items-center gap-1.5 rounded-full bg-slate-50 px-2.5 py-0.5 text-xs ring-1 ring-line">
          <span className={cn("size-1.5 rounded-full", s.hourly_readings_30d > 0 ? "bg-leaf" : "bg-pastel-blue")} />{label(s.provider)}
          <span className="text-muted">{s.hourly_readings_30d > 0 ? `${s.hourly_readings_30d} h` : ""}{s.hourly_readings_30d > 0 && s.monthly_figures > 0 ? " · " : ""}{s.monthly_figures > 0 ? `${s.monthly_figures} mo` : ""}</span>
        </span>))}</span>) },
    { key: "pin", header: "Pin a source", width: "190px", render: (r) => <Select label={`Source for ${r.region}`} className="w-44" value={r.override} disabled={!canAct || save.isPending} onChange={(v) => pin(r.region, v)} options={options} /> },
  ];
  return (
    <div className="space-y-5">
      <div>
        <p className="mb-3 max-w-3xl text-sm text-muted">Every region your customers run in: what each source has (hourly readings of the last 30 days, monthly figures) and which one is used. “Automatic” follows the priority order. Pinning puts one source first for that region only; if it has no data for a day, the next in the order is used.</p>
        {q.isLoading ? <Skeleton className="h-40" /> : q.isError ? <ErrorState /> : (
          <DataTable columns={columns} rows={rows} rowKey={(r) => r.region} empty="No region has carbon calculations yet." initialSort={{ key: "region", dir: "asc" }} />
        )}
        <Err e={save.error} />
      </div>
      <div className="grid gap-5 xl:grid-cols-2">
        <BackfillCard canAct={canAct} regions={data.regions_in_use} />
        <RecalculateCard canAct={canAct} />
      </div>
    </div>
  );
}

const DAYS = [7, 30, 90, 180, 365].map((d) => ({ value: String(d), label: `${d} days` }));

/** Redo the carbon of every project with the grid data that exists now (after loading the past, or a policy change). */
function RecalculateCard({ canAct }: { canAct: boolean }) {
  const [days, setDays] = useState("90");
  const run = useMutation({ mutationFn: () => api("operator/carbon/recalculate", z.object({ queued: z.boolean(), from: z.string(), to: z.string() }), undefined, { method: "POST", body: JSON.stringify({ days: Number(days) }) }) });
  return (
    <Card index={2} interactive={false}>
      <CardHeader title="Recalculate carbon" icon={<Refresh className="size-4 text-leaf-deep" />} />
      <p className="text-sm text-muted">Redoes the calculations of every organization&apos;s projects with the data and the policy in force now, so figures that rested on an estimate use what was loaded since. It runs in the background; a window is replaced as a whole and results of other methodology versions are untouched.</p>
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <Select label="Window" value={days} onChange={setDays} options={DAYS} />
        <Btn disabled={!canAct || run.isPending} onClick={() => run.mutate()}>{run.isPending ? "Queuing…" : "Recalculate everything"}</Btn>
      </div>
      {run.data && <Note>Queued for {run.data.from} to {run.data.to}. Follow it on Jobs &amp; scheduler.</Note>}
      <Err e={run.error} />
    </Card>
  );
}

const backfillPlan = z.object({ regions: z.array(z.string()), days: z.number(), estimated_requests: z.number(), queued: z.boolean() });

/** Fetch past grid readings so history is measured, not estimated. The estimate is shown before anything is asked. */
function BackfillCard({ canAct, regions }: { canAct: boolean; regions: number }) {
  const iso = (d: Date) => d.toISOString().slice(0, 10);
  const [from, setFrom] = useState(iso(new Date(Date.now() - 90 * 86_400_000)));
  const [to, setTo] = useState(iso(new Date(Date.now() + 86_400_000)));
  const run = useMutation({ mutationFn: (dry: boolean) => api("operator/carbon/backfill", backfillPlan, undefined, { method: "POST", body: JSON.stringify({ from, to, dry_run: dry }) }) });
  const plan = run.data;
  return (
    <Card index={3} interactive={false}>
      <CardHeader title="Load past grid data" icon={<Database className="size-4 text-leaf-deep" />} />
      <p className="text-sm text-muted">Days calculated before readings were collected rest on an estimate. Load the past readings of the {regions} region{regions === 1 ? "" : "s"} in use so history is measured. It uses the provider&apos;s past-data feature (a plan option) and counts against its quota; the estimate is shown first, and existing readings are never overwritten.</p>
      <div className="mt-4 flex flex-wrap items-end gap-3">
        <label className="block"><span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide text-slate-400">From</span><span className="block w-44"><DateField value={from} onChange={setFrom} label="from" /></span></label>
        <label className="block"><span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide text-slate-400">To (exclusive)</span><span className="block w-44"><DateField value={to} onChange={setTo} label="to" /></span></label>
        <Btn tone="quiet" disabled={run.isPending || !canAct} onClick={() => run.mutate(true)}>Estimate</Btn>
        {plan && !plan.queued && plan.regions.length > 0 && canAct && <Btn disabled={run.isPending} onClick={() => run.mutate(false)}>Start loading</Btn>}
      </div>
      {plan && <Note>{plan.regions.length === 0 ? "No region has carbon calculations yet." : plan.queued
        ? <>Started: {plan.regions.length} regions over {plan.days} days. Follow it on Jobs &amp; scheduler.</>
        : <>{plan.regions.length} regions × {plan.days} days ≈ <b>{plan.estimated_requests}</b> provider requests.</>}</Note>}
      <Err e={run.error} />
    </Card>
  );
}

// ---- Explain ------------------------------------------------------------------------------------------------------------

const VERDICT: Record<ExplainSource["verdict"], { tone: PillTone; text: string }> = {
  chosen: { tone: "good", text: "used" }, outranked: { tone: "info", text: "had enough, ranked lower" }, too_few: { tone: "warn", text: "too few readings" },
  backup: { tone: "violet", text: "backup only" }, stale: { tone: "bad", text: "only old readings" }, no_data: { tone: "neutral", text: "no data" },
};

function reason(e: NonNullable<ReturnType<typeof useExplain>["data"]>): string {
  const c = e.chosen;
  if (!c) return "No trusted source has a stored reading for this region, so there is nothing to calculate with. Load the past or add a source that covers it.";
  const s = e.sources.find((x) => x.source === c.source);
  switch (c.basis) {
    case "day_mean": return `${label(c.source)} has ${s?.day_readings ?? "enough"} hourly readings that day (the policy wants ${e.min_day_readings}), and it is the first source in the order that does. Its mean is used.`;
    case "month_mean": return `No source has enough hourly readings that day, so ${label(c.source)}'s published monthly figure is used: it describes the right month, which a day with a few readings does not describe better.`;
    case "partial_day_mean": return `No source has ${e.min_day_readings} readings that day. ${label(c.source)} is the first with some (${s?.day_readings}), so its mean is used and marked as partial.`;
    case "latest_before": return `Nothing was read that day. ${label(c.source)}'s last reading before it (${s?.age_hours != null ? `${Math.round(s.age_hours)} h earlier` : "earlier"}) is within the age the policy accepts (${s?.max_age_hours} h) and is used.`;
    case "latest_overall": return `Nothing recent exists. The latest reading of ${label(c.source)} is used, however old: it is an estimate, and the figure is marked that way.`;
    default: return `${label(c.source)} was asked directly because nothing was stored.`;
  }
}

function Fact({ k, v }: { k: string; v: ReactNode }) { return <div><dt className="text-[11px] font-medium uppercase tracking-wide text-slate-400">{k}</dt><dd className="mt-0.5 text-sm tabular-nums">{v}</dd></div>; }
const g = (n: number | null) => (n == null ? "—" : `${Math.round(n)} g/kWh`);

function ExplainTab({ data }: { data: Aggregation }) {
  const regions = useOperatorRegionSources();
  const list = regions.data?.items ?? [];
  const today = new Date().toISOString().slice(0, 10);
  const [region, setRegion] = useState("");
  const [day, setDay] = useState(today);
  const current = region || list[0]?.region || "";
  const q = useExplain(current, day);
  const e = q.data;
  return (
    <div className="space-y-5">
      <Card index={0} interactive={false}>
        <CardHeader title="Why this figure?" icon={<Info className="size-4 text-leaf-deep" />} />
        <p className="max-w-3xl text-sm text-muted">Pick a region and a day. The engine says what every trusted source has, in the order the policy tries them, and why one of them was chosen. It reads stored readings only; no provider is called.</p>
        <div className="mt-4 flex flex-wrap items-end gap-3">
          <label className="block"><span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide text-slate-400">Region</span>
            <Select label="Region" className="w-56" value={current} onChange={setRegion} options={list.map((r) => ({ value: r.region, label: r.region }))} placeholder="No regions yet" /></label>
          <label className="block"><span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide text-slate-400">Day (UTC)</span><span className="block w-44"><DateField value={day} onChange={setDay} max={today} label="Day" /></span></label>
        </div>
      </Card>
      {list.length === 0 && !regions.isLoading ? null : q.isLoading ? <Skeleton className="h-48" /> : q.isError ? <ErrorState /> : e && (
        <div key={`${e.region}-${e.day}`} className="space-y-4">
          <div className={cn("rise rounded-2xl p-5 ring-1", e.chosen ? "bg-leaf-soft/50 ring-leaf/20" : "bg-rose-50 ring-rose-200")}>
            <div className="flex flex-wrap items-center gap-4">
              {e.chosen ? <ProviderMark id={e.chosen.source} size="lg" /> : <AlertTriangle className="size-9 text-[#d9707f]" aria-hidden />}
              <div className="min-w-0 flex-1">
                <p className="text-lg font-bold tracking-tight">{e.chosen ? <>{label(e.chosen.source)} · {Math.round(e.chosen.g_per_kwh)} <span className="text-sm font-medium text-muted">gCO₂e/kWh</span></> : "No figure can be calculated"}</p>
                <p className="mt-0.5 text-sm text-muted">{e.region} · {e.day}</p>
              </div>
              {e.chosen && <Pill tone={basis(e.chosen.basis).tone}>{basis(e.chosen.basis).text}</Pill>}
            </div>
            <p className="mt-3 text-sm text-slate-700">{reason(e)}</p>
          </div>
          <ol className="space-y-3">
            {e.sources.map((s, i) => {
              const v = VERDICT[s.verdict];
              return (
                <li key={s.source} className={cn("rise rounded-2xl bg-white p-4 ring-1", s.verdict === "chosen" ? "ring-leaf/40" : "ring-line")} style={{ "--i": i + 1 } as CSSProperties}>
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="grid size-7 place-items-center rounded-full bg-slate-100 text-xs font-bold tabular-nums text-slate-500">{s.position}</span>
                    <ProviderMark id={s.source} size="sm" />
                    <p className="flex-1 font-semibold">{label(s.source)}{s.pinned && <span className="ml-2 text-xs font-medium text-leaf-deep">pinned for this region</span>}</p>
                    <Pill tone={v.tone}>{v.text}</Pill>
                  </div>
                  <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-4">
                    <Fact k="Readings that day" v={`${s.day_readings} of ${e.min_day_readings} needed${s.day_mean != null ? ` · mean ${g(s.day_mean)}` : ""}`} />
                    <Fact k="Monthly figure" v={g(s.monthly)} />
                    <Fact k="Last reading before" v={s.latest_before != null ? `${g(s.latest_before)} · ${Math.round(s.age_hours ?? 0)} h before${s.within_max_age ? "" : " (too old)"}` : "—"} />
                    <Fact k="Accepted age" v={`${s.max_age_hours} h`} />
                  </dl>
                </li>
              );
            })}
          </ol>
          {data.sources.filter((s) => !e.order.includes(s.id)).length > 0 && (
            <p className="text-xs text-muted">Not in the policy, never used: {data.sources.filter((s) => !e.order.includes(s.id)).map((s) => label(s.id)).join(", ")}.</p>
          )}
        </div>
      )}
    </div>
  );
}

// ---- Provenance ---------------------------------------------------------------------------------------------------------

function ProvenanceTab({ data }: { data: Aggregation }) {
  const p = data.provenance;
  const byBasis = useMemo(() => {
    const m = new Map<string, number>();
    for (const r of p.rows) m.set(r.basis, (m.get(r.basis) ?? 0) + r.rows);
    return [...m.entries()].sort((a, b) => b[1] - a[1]);
  }, [p.rows]);
  const estimate = p.rows.filter((r) => r.basis === "latest_overall" || r.basis === "partial_day_mean").reduce((n, r) => n + r.rows, 0);
  const share = p.total ? estimate / p.total : 0;
  const cols: Column<(typeof p.rows)[number]>[] = [
    { key: "src", header: "Source", sort: (a, b) => a.source.localeCompare(b.source), render: (r) => (r.source ? <span className="inline-flex items-center gap-2"><ProviderMark id={r.source} size="sm" />{label(r.source)}</span> : <span className="text-muted">Not recorded</span>) },
    { key: "basis", header: "Basis", sort: (a, b) => a.basis.localeCompare(b.basis), render: (r) => <span title={basis(r.basis).hint}><Pill tone={basis(r.basis).tone}>{basis(r.basis).text}</Pill></span> },
    { key: "rows", header: "Rows", align: "right", sort: (a, b) => a.rows - b.rows, render: (r) => <span className="tabular-nums">{r.rows.toLocaleString()}</span> },
    { key: "share", header: "Share", align: "right", render: (r) => <span className="tabular-nums text-muted">{p.total ? `${Math.round((r.rows / p.total) * 100)}%` : "—"}</span> },
  ];
  return (
    <div className="space-y-5">
      <Card index={0} interactive={false}>
        <CardHeader title={`Where the intensity of the last ${p.days} days came from`} icon={<Gauge className="size-4 text-leaf-deep" />} />
        {p.total === 0 ? <p className="text-sm text-muted">No calculation rows in this window yet.</p> : (<>
          <div role="img" aria-label="Share of calculation rows by basis" className="flex h-4 overflow-hidden rounded-full bg-slate-100">
            {byBasis.map(([b, n]) => <span key={b} title={`${basis(b).text}: ${n.toLocaleString()}`} className={cn("grow-x h-full", basis(b).bar)} style={{ width: `${(n / p.total) * 100}%` }} />)}
          </div>
          <ul className="mt-4 grid gap-x-6 gap-y-2 sm:grid-cols-2 xl:grid-cols-3">
            {byBasis.map(([b, n]) => (
              <li key={b} className="flex items-start gap-2.5 text-sm" title={basis(b).hint}>
                <span className={cn("mt-1.5 size-2.5 shrink-0 rounded-full", basis(b).bar)} />
                <span><span className="font-medium">{basis(b).text}</span> <span className="tabular-nums text-muted">{Math.round((n / p.total) * 100)}% · {n.toLocaleString()}</span><span className="block text-xs text-muted">{basis(b).hint}</span></span>
              </li>
            ))}
          </ul>
          {share >= 0.25 && (
            <p className="rise mt-4 flex items-start gap-2.5 rounded-xl bg-amber-50 px-3.5 py-3 text-sm text-slate-700 ring-1 ring-amber-100">
              <AlertTriangle className="mt-0.5 size-4 shrink-0 text-amber-600" aria-hidden />
              <span><b>{Math.round(share * 100)}%</b> of these rows rest on an estimate or on few readings, not on the day&apos;s own data. Load the past of the regions in use (Regions tab) and recalculate to make them measured.</span>
            </p>
          )}
        </>)}
      </Card>
      {p.rows.length > 0 && <DataTable columns={cols} rows={p.rows} rowKey={(r) => `${r.source}|${r.basis}`} initialSort={{ key: "rows", dir: "desc" }} dense footer={`${p.total.toLocaleString()} rows in ${p.days} days · counts only, no organization or value is shown`} />}
    </div>
  );
}

// ---- Rules --------------------------------------------------------------------------------------------------------------

const NOT_YET = [
  ["Marginal intensity", "WattTime-style marginal figures are a different measure. They are not stored yet; when they are, they will be kept apart and never averaged with the average intensity."],
  ["A policy per metric", "Today one policy covers grid intensity. Prices and energy factors have one source each, so there is nothing to choose between yet."],
  ["Spend limits on paid APIs", "Only the request count per day is capped (Platform settings → Grid data sources). Spend is not metered because the price of a request depends on the provider's plan."],
  ["More sources", "ENTSO-E (Europe) as a measured source. A source is a connector plus an entry in the priority list; nothing else changes."],
];

function RulesTab({ data }: { data: Aggregation }) {
  return (
    <div className="grid gap-5 xl:grid-cols-5">
      <Card index={0} interactive={false} className="xl:col-span-3">
        <CardHeader title="What the engine does when it combines sources" icon={<Info className="size-4 text-leaf-deep" />} />
        <ol className="space-y-3">
          {data.rules.map((r, i) => (
            <li key={r.code} className="rise flex gap-3.5 rounded-2xl bg-slate-50/60 p-3.5 ring-1 ring-line/60" style={{ "--i": Math.min(i, 8) } as CSSProperties}>
              <span className="grid size-7 shrink-0 place-items-center rounded-full bg-leaf-soft text-xs font-bold tabular-nums text-leaf-deep">{i + 1}</span>
              <div><p className="font-semibold">{r.title}</p><p className="mt-0.5 text-sm text-muted">{r.text}</p></div>
            </li>
          ))}
        </ol>
      </Card>
      <Card index={1} interactive={false} className="h-fit xl:col-span-2">
        <CardHeader title="Not built yet" />
        <ul className="space-y-3.5">
          {NOT_YET.map(([t, text]) => <li key={t}><p className="flex items-center gap-2 font-medium"><Pill tone="violet">planned</Pill>{t}</p><p className="mt-1 text-sm text-muted">{text}</p></li>)}
        </ul>
      </Card>
    </div>
  );
}

// ---- Page ---------------------------------------------------------------------------------------------------------------

export function AggregationView() {
  const q = useAggregation();
  const [tab, setTab] = useState("policy");
  const d = q.data;
  const used = d?.sources.filter((s) => s.position > 0) ?? [];
  const stale = used.filter((s) => s.freshness === "stale" || s.freshness === "none").length;
  return (
    <div className="space-y-6">
      <PageTitle title="Aggregation & policies" sub="Which source wins for which region, what happens when it is stale, and where every figure came from. Sources are never averaged together; the choice is a policy you can read, change and check." />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Tile index={0} label="Sources in the policy" value={d ? used.length : undefined} icon={Database} color="lavender" note={d ? `of ${d.sources.length} with a connector` : undefined} />
        <Tile index={1} label="Regions in use" value={d?.regions_in_use} icon={Gauge} color="sky" />
        <Tile index={2} label="Without fresh data" value={d ? stale : undefined} icon={AlertTriangle} color="butter" tone={stale ? "warn" : undefined} note="sources in the policy with nothing recent" />
        <Tile index={3} label="From the day's own readings" value={d ? Math.round(d.provenance.precise_share * 100) : undefined} icon={CheckCircle2} color="sage" note={d ? `% of ${d.provenance.total.toLocaleString()} rows, ${d.provenance.days} days` : undefined} />
      </div>
      <PillTabs value={tab} onChange={setTab} label="Aggregation sections" tabs={[
        { value: "policy", label: "Policy" }, { value: "sources", label: "Sources & keys" }, { value: "divergence", label: "Source divergence" }, { value: "regions", label: "Regions and maintenance" }, { value: "explain", label: "Why this figure" },
        { value: "provenance", label: "Provenance" }, { value: "rules", label: "Rules" },
      ]} />
      {q.isLoading ? <Skeleton className="h-64" /> : q.isError || !d ? <ErrorState /> : (
        <div key={tab} className="page-enter">
          {tab === "policy" && <PolicyTab data={d} />}
          {tab === "sources" && <SourcesTab data={d} />}
          {tab === "divergence" && <DivergenceTab data={d} />}
          {tab === "regions" && <RegionsTab data={d} />}
          {tab === "explain" && <ExplainTab data={d} />}
          {tab === "provenance" && <ProvenanceTab data={d} />}
          {tab === "rules" && <RulesTab data={d} />}
        </div>
      )}
    </div>
  );
}
