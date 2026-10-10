"use client";

import { ArrowRight, CheckCircle2, CircleDashed, Database, Hourglass, KeyRound, Server, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { api } from "@/lib/api/client";
import { z } from "zod";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { PillTabs } from "@/components/ui/tabs";
import { SettingsEditor } from "@/components/settings/settings-editor";
import { useOperatorDataSources, useOperatorIntegrations, useOperatorMe, useOperatorRegionSources, useOperatorSystem } from "@/hooks/use-admin";
import { shortTime } from "@/lib/format-time";
import type { DataSource, DataSourceAccess, DataSourceState, ManagedIntegration } from "@/lib/api/schemas";
import { Chip, PageTitle, Tile } from "./parts";

export const ACCESS: Record<DataSourceAccess, { label: string; cls: string }> = {
  free: { label: "Free", cls: "bg-emerald-50 text-emerald-700 ring-emerald-200" },
  paid: { label: "Paid / plan-dependent", cls: "bg-amber-50 text-amber-700 ring-amber-200" },
  customer_provided: { label: "Customer-provided", cls: "bg-sky-50 text-sky-700 ring-sky-200" },
  internal: { label: "Internal", cls: "bg-violet-50 text-violet-700 ring-violet-200" },
};
export const STATE: Record<DataSourceState, { label: string; tone: "good" | "warn" | "bad" | "neutral" | "stub"; icon: typeof CheckCircle2 }> = {
  ready: { label: "Working", tone: "good", icon: CheckCircle2 },
  attention: { label: "Needs attention", tone: "bad", icon: TriangleAlert },
  not_configured: { label: "Not configured", tone: "warn", icon: CircleDashed },
  planned: { label: "Planned", tone: "stub", icon: Hourglass },
};
/** Integrations that are payment providers: managed on Plans & billing, not among the platform services. */
export const PAYMENT_IDS = ["stripe", "paddle", "liqpay", "wayforpay", "paymentlink", "invoice"];
export const CATEGORY: Record<string, string> = { inventory: "Emissions inventory", grid_intensity: "Grid carbon intensity", billing: "Cloud billing", usage: "Usage", kubernetes: "Kubernetes", methodology: "Methodology" };

/** One line about the settings of a source: whether its key or address is in place. */
function settingsLine(st: ManagedIntegration | undefined) {
  if (!st) return null;
  const secret = st.fields.find((f) => f.secret && f.required) ?? st.fields.find((f) => f.required);
  if (st.ready) return <span className="inline-flex items-center gap-1.5 text-emerald-700"><KeyRound className="size-3.5" aria-hidden />{st.prepared ? "Credentials stored" : "Settings complete"}</span>;
  return <span className="inline-flex items-center gap-1.5 text-amber-700"><KeyRound className="size-3.5" aria-hidden />{secret ? `${secret.label} not set` : "Needs setup"}</span>;
}

function SourceCard({ s, st, index }: { s: DataSource; st?: ManagedIntegration; index: number }) {
  const state = STATE[s.state];
  return (
    <Card index={index} className="flex flex-col gap-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h3 className="text-[15px] font-semibold tracking-tight">{s.name}</h3>
          <p className="text-xs text-muted">{CATEGORY[s.category] ?? s.category}</p>
        </div>
        <div className="flex flex-wrap gap-1.5">
          <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${ACCESS[s.access].cls}`}>{ACCESS[s.access].label}</span>
          <Badge tone={state.tone}><state.icon className="mr-1 size-3" aria-hidden />{state.label}</Badge>
        </div>
      </div>
      <div className="flex flex-wrap gap-1.5">{s.metrics.map((m) => <Chip key={m}>{m}</Chip>)}</div>
      <div className="mt-auto flex flex-wrap items-center justify-between gap-2 border-t border-line/70 pt-3 text-sm">
        <span className="text-xs">
          {settingsLine(st) ?? (s.connections != null ? <span className="text-muted">{s.connections} customer connection{s.connections === 1 ? "" : "s"}{s.in_error ? ` · ${s.in_error} failing` : ""}</span> : <span className="text-muted">No platform settings</span>)}
        </span>
        <Link href={`/operator/data-sources/${s.id}`} className="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 font-medium text-primary ring-1 ring-line hover:bg-slate-50">
          {st ? "Settings" : "Details"}<ArrowRight className="size-3.5" aria-hidden />
        </Link>
      </div>
    </Card>
  );
}

const SRC_LABEL: Record<string, string> = { electricitymaps: "Electricity Maps", ember: "Ember", uk_carbon_intensity: "UK Carbon Intensity" };
const OFFERED = ["electricitymaps", "ember"];

/** Which source serves each region in use: what each source has, the choice in force and a selector to pin one. */
function RegionSources({ canAct }: { canAct: boolean }) {
  const qc = useQueryClient();
  const q = useOperatorRegionSources();
  const save = useMutation({
    mutationFn: (overrides: string[]) => api("operator/settings", z.unknown(), undefined, { method: "PATCH", body: JSON.stringify({ values: { "carbon.region_provider_overrides": overrides } }) }),
    onSuccess: () => {
      for (const k of ["op-region-sources", "settings", "op-settings"]) void qc.invalidateQueries({ queryKey: [k] });
    },
  });
  const rows = q.data?.items ?? [];
  const set = (region: string, source: string) => {
    const next = rows.filter((r) => r.region !== region && r.override).map((r) => `${r.region}=${r.override}`);
    if (source) next.push(`${region}=${source}`);
    save.mutate(next);
  };
  return (
    <Card interactive={false}>
      <h3 className="text-[15px] font-semibold tracking-tight">Source per region</h3>
      <p className="mt-1 text-sm text-muted">For every region your customers run in: what each source has, and which one is used. “Automatic” follows the priority order above. Pinning a source puts it first for that region only; if it has no data for a day, the next source in the order is used.</p>
      {q.isLoading ? <Skeleton className="mt-3 h-24" /> : q.isError ? <ErrorState /> : rows.length === 0 ? (
        <p className="mt-3 text-sm text-muted">No region has carbon calculations yet.</p>
      ) : (
        <div className="mt-3 overflow-x-auto">
          <table className="w-full text-sm">
            <thead><tr><th className="py-1.5 pr-3 text-left text-xs font-medium text-muted">Region</th>{OFFERED.map((p) => <th key={p} className="py-1.5 pr-3 text-left text-xs font-medium text-muted">{SRC_LABEL[p]}</th>)}<th className="py-1.5 pr-3 text-left text-xs font-medium text-muted">Used now</th><th className="py-1.5 text-left text-xs font-medium text-muted">Source</th></tr></thead>
            <tbody className="divide-y divide-line/70">
              {rows.map((r) => (
                <tr key={r.region}>
                  <td className="py-2 pr-3 font-medium">{r.region}</td>
                  {OFFERED.map((p) => {
                    const s = r.sources.find((x) => x.provider === p);
                    return <td key={p} className="pr-3 text-xs text-muted">{s ? <>{s.hourly_readings_30d > 0 ? `${s.hourly_readings_30d} hourly (30 d)` : ""}{s.hourly_readings_30d > 0 && s.monthly_figures > 0 ? " · " : ""}{s.monthly_figures > 0 ? `${s.monthly_figures} months` : ""}{s.newest ? <span className="block">newest {shortTime(s.newest)}</span> : null}</> : "—"}</td>;
                  })}
                  <td className="pr-3">{r.effective ? <Badge tone="good">{SRC_LABEL[r.effective] ?? r.effective}</Badge> : <Badge tone="warn">no data</Badge>}</td>
                  <td>
                    <select aria-label={`Source for ${r.region}`} value={r.override} disabled={!canAct || save.isPending} onChange={(e) => set(r.region, e.target.value)} className="rounded-lg bg-white/80 px-2 py-1 text-sm ring-1 ring-line">
                      <option value="">Automatic</option>{OFFERED.map((p) => <option key={p} value={p}>{SRC_LABEL[p]}</option>)}
                    </select>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {save.isError && <p role="alert" className="mt-2 text-sm text-bad">{save.error.message}</p>}
    </Card>
  );
}

const backfillPlan = z.object({ regions: z.array(z.string()), days: z.number(), estimated_requests: z.number(), queued: z.boolean() });

/** Fetch past grid readings so history is measured, not estimated. The estimate is shown before anything is asked. */
function BackfillCard({ canAct }: { canAct: boolean }) {
  const iso = (d: Date) => d.toISOString().slice(0, 10);
  const [from, setFrom] = useState(iso(new Date(Date.now() - 90 * 86_400_000)));
  const [to, setTo] = useState(iso(new Date(Date.now() + 86_400_000)));
  const run = useMutation({
    mutationFn: (dry: boolean) => api("operator/carbon/backfill", backfillPlan, undefined, { method: "POST", body: JSON.stringify({ from, to, dry_run: dry }) }),
  });
  const plan = run.data;
  return (
    <Card interactive={false}>
      <h3 className="text-[15px] font-semibold tracking-tight">Load past grid data</h3>
      <p className="mt-1 text-sm text-muted">Days that were calculated before readings were collected use an estimate. Load the past readings of the regions in use so history is measured. It uses the provider’s past-data feature (a plan option) and counts against its request quota; the estimate is shown first. Existing readings are never overwritten.</p>
      <div className="mt-3 flex flex-wrap items-end gap-3 text-sm">
        <label className="block"><span className="mb-1 block text-xs text-muted">From</span><input type="date" value={from} onChange={(e) => setFrom(e.target.value)} className="rounded-lg bg-white/80 px-2.5 py-1.5 ring-1 ring-line" /></label>
        <label className="block"><span className="mb-1 block text-xs text-muted">To (exclusive)</span><input type="date" value={to} onChange={(e) => setTo(e.target.value)} className="rounded-lg bg-white/80 px-2.5 py-1.5 ring-1 ring-line" /></label>
        <button type="button" disabled={run.isPending || !canAct} onClick={() => run.mutate(true)} className="rounded-xl px-3.5 py-2 font-medium ring-1 ring-line hover:bg-slate-50 disabled:opacity-50">Estimate</button>
        {plan && !plan.queued && plan.regions.length > 0 && canAct && (
          <button type="button" disabled={run.isPending} onClick={() => run.mutate(false)} className="brand-gradient rounded-xl px-4 py-2 font-semibold text-white disabled:opacity-50">Start loading</button>
        )}
      </div>
      {plan && (
        <p role="status" className="mt-3 rounded-xl bg-slate-50 px-3 py-2 text-sm">
          {plan.regions.length === 0 ? "No region has carbon calculations yet." : plan.queued
            ? <>Started: {plan.regions.length} regions over {plan.days} days. Follow it on Jobs &amp; scheduler.</>
            : <>{plan.regions.length} regions × {plan.days} days ≈ <b>{plan.estimated_requests}</b> provider requests.</>}
        </p>
      )}
      {run.isError && <p role="alert" className="mt-2 text-sm text-bad">{run.error.message}</p>}
    </Card>
  );
}

export function DataSourcesView() {
  const q = useOperatorDataSources();
  const ints = useOperatorIntegrations();
  const sys = useOperatorSystem();
  const me = useOperatorMe();
  const [tab, setTab] = useState("sources");
  const [filter, setFilter] = useState("all");
  const data = q.data;
  const byId = useMemo(() => new Map((ints.data ?? []).map((i) => [i.id, i])), [ints.data]);
  const linked = useMemo(() => new Set((data ?? []).map((s) => s.integration_id).filter(Boolean)), [data]);
  const services = (ints.data ?? []).filter((i) => !linked.has(i.id) && !PAYMENT_IDS.includes(i.id)); // payment providers live on Plans & billing
  const count = (st: DataSourceState) => data?.filter((s) => s.state === st).length ?? null;
  const shown = (data ?? []).filter((s) => filter === "all" || s.access === filter || (filter === "planned" && s.state === "planned"));
  const fixed = (sys.data?.integrations ?? []).filter((i) => ["Auth0 (sign-in)", "Notifications key"].includes(i.name));

  return (
    <div className="space-y-5">
      <PageTitle title="Data sources" sub="Everything the platform reads and the services it runs on. Open a source to see its terms and state and to set its keys and addresses on the same page." />
      <PillTabs value={tab} onChange={setTab} label="Data sources sections" tabs={[{ value: "sources", label: "Data sources" }, { value: "choice", label: "Which source is used" }, { value: "services", label: "Platform services" }]} />

      {tab === "choice" ? (
        <div className="space-y-3">
          <Card interactive={false}>
            <h3 className="text-[15px] font-semibold tracking-tight">How the grid carbon intensity is chosen</h3>
            <ul className="mt-2 list-disc space-y-1.5 pl-5 text-sm text-muted">
              <li>For each region and day, the first source in the priority list that has enough readings that day is used. A source with fewer readings is used only when none has enough, and is marked as partial.</li>
              <li>Sources are <b className="text-ink">never averaged together</b>: they measure differently and are licensed differently. Others are shown beside the chosen one.</li>
              <li>Average and marginal intensity are different quantities and are stored separately; only average intensity feeds the carbon figures today.</li>
              <li>Every calculation row records the source and basis it used, and customers see where their figures came from on Carbon &amp; Energy.</li>
              <li>A change applies to the next calculation; re-run a window to apply it to history.</li>
            </ul>
          </Card>
          <SettingsEditor mode="platform" path="operator/settings" historyPath="operator/settings/history" canEdit={me.data?.role === "admin"} onlyGroup="carbon" hideKeys={["carbon.region_provider_overrides"]} />
          <RegionSources canAct={me.data?.role === "admin"} />
          <BackfillCard canAct={me.data?.role === "admin"} />
        </div>
      ) : tab === "sources" ? (<>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <Tile index={0} label="Registered sources" value={data ? data.filter((s) => s.state !== "planned").length : null} icon={Database} color="lavender" />
          <Tile index={1} label="Working" value={count("ready")} icon={CheckCircle2} color="sage" />
          <Tile index={2} label="Need attention" value={data ? (count("attention") ?? 0) + (count("not_configured") ?? 0) : null} icon={TriangleAlert} color="peach" tone={(count("attention") ?? 0) > 0 ? "bad" : undefined} note="Failing or not configured" />
          <Tile index={3} label="Planned" value={count("planned")} icon={Hourglass} color="sky" />
        </div>
        <PillTabs value={filter} onChange={setFilter} label="Filter sources" tabs={[
          { value: "all", label: "All" }, { value: "free", label: "Free" }, { value: "paid", label: "Paid" },
          { value: "customer_provided", label: "Customer-provided" }, { value: "internal", label: "Internal" }, { value: "planned", label: "Planned" },
        ]} />
        {q.isLoading ? <Skeleton className="h-56" /> : q.isError ? <ErrorState /> : shown.length === 0 ? <EmptyState icon={Database} text="No sources in this filter." /> : (
          <div className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">{shown.map((s, i) => <SourceCard key={s.id} s={s} st={byId.get(s.integration_id)} index={i} />)}</div>
        )}
        <p className="text-xs text-muted">Spend on paid APIs is not metered yet, so no figure is shown. Priorities between sources, fallbacks and cost limits come with the aggregation section.</p>
      </>) : (
        <>
          {ints.isLoading ? <Skeleton className="h-40" /> : ints.isError ? <ErrorState /> : (
            <div className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">
              {services.map((st, i) => (
                <Card key={st.id} index={i} className="flex flex-col gap-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2"><Server className="size-4 text-slate-400" aria-hidden /><h3 className="text-[15px] font-semibold tracking-tight">{st.name}</h3></div>
                    <Badge tone={st.ready ? "good" : "warn"}>{st.ready ? "Ready" : "Needs setup"}</Badge>
                  </div>
                  <p className="text-sm text-muted">{st.description}</p>
                  <div className="mt-auto flex items-center justify-between gap-2 border-t border-line/70 pt-3 text-sm">
                    <span className="text-xs text-muted">{st.startup ? "Applied at restart" : "Applies within seconds"}</span>
                    <Link href={`/operator/data-sources/${st.id}`} className="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 font-medium text-primary ring-1 ring-line hover:bg-slate-50">Settings<ArrowRight className="size-3.5" aria-hidden /></Link>
                  </div>
                </Card>
              ))}
            </div>
          )}
          {fixed.length > 0 && (
            <Card interactive={false}>
              <h3 className="mb-2 text-[15px] font-semibold tracking-tight">Fixed by the deployment</h3>
              <ul className="divide-y divide-line/70 text-sm">
                {fixed.map((i) => (
                  <li key={i.name} className="flex flex-wrap items-center justify-between gap-2 py-2.5">
                    <span>{i.name}{i.note && <span className="ml-2 text-xs text-muted">{i.note}</span>}</span>
                    <Badge tone={i.status === "ready" ? "good" : i.status === "stub" ? "stub" : "warn"}>{i.status === "ready" ? "Ready" : i.status === "stub" ? "Stand-in" : "Not set"}</Badge>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
