"use client";

import { IconCalculator as Calculator, IconCheckCircle as CheckCircle2, IconCircleDashed as CircleDashed, IconCloud as Cloud, IconCpu as Cpu, IconGauge as Gauge, IconHourglass as Hourglass, IconLeaf as Leaf, IconServer as Server, IconWarning as TriangleAlert, IconArrowRight as ArrowRight, IconDatabase as Database, IconKey as KeyRound } from "@/components/ui/icons";
import Link from "next/link";
import { ProviderMark } from "@/components/connectors/provider-mark";
import { useMemo, useState } from "react";
import { Card } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useOperatorDataSources, useOperatorIntegrations, useOperatorSystem } from "@/hooks/use-admin";
import type { DataSource, DataSourceAccess, DataSourceState, ManagedIntegration } from "@/lib/api/schemas";
import { Chip, PageTitle, Tile } from "./parts";
import { Pill, Segmented, type PillTone } from "./ui";

export const ACCESS: Record<DataSourceAccess, { label: string; cls: string }> = {
  free: { label: "Free", cls: "bg-leaf-soft text-leaf-deep ring-leaf-soft" },
  paid: { label: "Paid / plan-dependent", cls: "bg-amber-50 text-[#a8761f] ring-amber-200" },
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
  if (st.ready && !st.fields.some((f) => f.secret && f.required)) return <span className="inline-flex items-center gap-1.5 text-slate-500"><KeyRound className="size-3.5" aria-hidden />No key needed</span>;
  if (st.ready) return <span className="inline-flex items-center gap-1.5 text-leaf-deep"><KeyRound className="size-3.5" aria-hidden />{st.prepared ? "Credentials stored" : "Settings complete"}</span>;
  return <span className="inline-flex items-center gap-1.5 text-[#a8761f]"><KeyRound className="size-3.5" aria-hidden />{secret ? `${secret.label} not set` : "Needs setup"}</span>;
}

// The order and the voice of the sections: what a person wants to know first, from the climate data to the platform's own rules.
const SECTIONS: { id: string; title: string; text: string; icon: typeof Database; tint: string }[] = [
  { id: "grid_intensity", title: "Grid carbon intensity", text: "How clean the electricity is, by region and hour. The figures every footprint is multiplied by.", icon: Gauge, tint: "bg-leaf-soft text-leaf-deep" },
  { id: "inventory", title: "Emissions inventory", text: "Independent sector and country inventories to check our estimates against.", icon: Leaf, tint: "bg-emerald-50 text-emerald-700" },
  { id: "billing", title: "Cloud billing", text: "What customers' clouds cost and use. Read-only, with the customer's own access.", icon: Cloud, tint: "bg-sky-50 text-sky-700" },
  { id: "kubernetes", title: "Kubernetes", text: "Workload energy pushed by the customer's agent.", icon: Cpu, tint: "bg-violet-50 text-violet-700" },
  { id: "methodology", title: "Methodology", text: "The platform's own coefficients and rules.", icon: Calculator, tint: "bg-amber-50 text-[#a8761f]" },
];
// the source ids of billing and Kubernetes differ from the ids of their drawn marks
const MARK_OF: Record<string, string> = { "aws-cost": "aws", "gcp-billing": "gcp", "azure-billing": "azure", "k8s-agent": "kubernetes" };
const STATE_ORDER: Record<DataSourceState, number> = { attention: 0, not_configured: 1, ready: 2, planned: 3 };
// soft pastel pills, the same as on Aggregation: green works, amber waits for a person, rose failed, lavender is planned
const PILL_STATE: Record<DataSourceState, PillTone> = { ready: "good", attention: "bad", not_configured: "warn", planned: "violet" };
const PILL_ACCESS: Record<DataSourceAccess, PillTone> = { free: "good", paid: "warn", customer_provided: "info", internal: "violet" };

function SourceCard({ s, st, index }: { s: DataSource; st?: ManagedIntegration; index: number }) {
  const state = STATE[s.state];
  return (
    <Link href={`/operator/data-sources/${s.id}`} aria-label={`${s.name}: ${state.label}`}
      className="glass rise lift group relative flex flex-col gap-3 overflow-hidden p-4 outline-none focus-visible:ring-2 focus-visible:ring-leaf/50"
      style={{ "--i": Math.min(index, 12) } as React.CSSProperties}>
      <div className="flex items-start gap-3">
        <span className="grid size-12 shrink-0 place-items-center rounded-2xl bg-slate-50 ring-1 ring-line/70 transition-transform duration-500 group-hover:scale-105 group-hover:-rotate-2">
          <ProviderMark id={MARK_OF[s.id] ?? (s.integration_id || s.id)} size="md" dim={s.state === "planned"} />
        </span>
        <div className="min-w-0 flex-1">
          <h3 className="text-[15px] font-semibold leading-snug tracking-tight">{s.name}</h3>
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
            <Pill tone={PILL_STATE[s.state]} pulse={s.state === "ready"}>{state.label}</Pill>
            <Pill tone={PILL_ACCESS[s.access]}>{ACCESS[s.access].label}</Pill>
          </div>
        </div>
        <ArrowRight className="mt-1 size-4 shrink-0 text-slate-300 transition-all duration-300 group-hover:translate-x-0.5 group-hover:text-leaf-deep" aria-hidden />
      </div>
      <p className="line-clamp-2 text-[13px] leading-relaxed text-muted">{s.coverage}</p>
      <div className="mt-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5 border-t border-line/60 pt-3 text-xs">
        {settingsLine(st) ?? (s.connections != null ? <span className="text-muted">{s.connections} customer connection{s.connections === 1 ? "" : "s"}{s.in_error ? ` · ${s.in_error} failing` : ""}</span> : <span className="text-muted">No platform settings</span>)}
        {s.metrics[0] && <span className="max-w-[55%] truncate text-slate-400" title={s.metrics.join(" · ")}>{s.metrics[0]}</span>}
      </div>
    </Link>
  );
}

function SectionHeader({ meta, shown, working, index }: { meta: (typeof SECTIONS)[number]; shown: number; working: number; index: number }) {
  return (
    <div className="rise flex items-center gap-3.5" style={{ "--i": Math.min(index, 8) } as React.CSSProperties}>
      <span className={`grid size-11 shrink-0 place-items-center rounded-2xl ${meta.tint} shadow-sm`}><meta.icon className="size-5" strokeWidth={1.6} aria-hidden /></span>
      <div className="min-w-0 flex-1">
        <h2 id={`sec-${meta.id}`} className="flex flex-wrap items-baseline gap-x-2.5 text-lg font-bold tracking-tight">{meta.title}
          <span className="text-xs font-medium text-muted">{shown} source{shown === 1 ? "" : "s"}{working > 0 ? ` · ${working} working` : ""}</span>
        </h2>
        {meta.text && <p className="text-[13px] text-muted">{meta.text}</p>}
      </div>
      <span className="hidden h-px flex-1 bg-gradient-to-r from-line to-transparent md:block" aria-hidden />
    </div>
  );
}

export function DataSourcesView() {
  const q = useOperatorDataSources();
  const ints = useOperatorIntegrations();
  const sys = useOperatorSystem();
  const [tab, setTab] = useState("sources");
  const [filter, setFilter] = useState("all");
  const data = q.data;
  const byId = useMemo(() => new Map((ints.data ?? []).map((i) => [i.id, i])), [ints.data]);
  const linked = useMemo(() => new Set((data ?? []).map((s) => s.integration_id).filter(Boolean)), [data]);
  const services = (ints.data ?? []).filter((i) => !linked.has(i.id) && !PAYMENT_IDS.includes(i.id)); // payment providers live on Plans & billing
  const count = (st: DataSourceState) => data?.filter((s) => s.state === st).length ?? null;
  const all = data ?? [];
  const shown = all.filter((s) => filter === "all" || s.access === filter || (filter === "planned" && s.state === "planned"));
  const fixed = (sys.data?.integrations ?? []).filter((i) => ["Auth0 (sign-in)", "Notifications key"].includes(i.name));
  // Sections in the order above, the ones that need a person first inside each; a category nobody named still gets a place at the end.
  const known = new Set(SECTIONS.map((x) => x.id));
  const sections = [...SECTIONS, ...[...new Set(shown.map((s) => s.category))].filter((c) => !known.has(c)).map((c) => ({ id: c, title: CATEGORY[c] ?? c, text: "", icon: Database, tint: "bg-slate-100 text-slate-600" }))]
    .map((meta) => ({ meta, items: shown.filter((s) => s.category === meta.id).sort((a, b) => STATE_ORDER[a.state] - STATE_ORDER[b.state] || a.name.localeCompare(b.name)) }))
    .filter((x) => x.items.length > 0);
  const free = data?.filter((s) => s.access === "free" && s.state !== "planned").length ?? null;

  return (
    <div className="space-y-6">
      <PageTitle title="Data sources" sub="Everything the platform reads, by what it is for. Open a source to see its terms and state and to set its keys and addresses on the same page." />
      <Segmented value={tab} onChange={setTab} label="Data sources sections" options={[{ value: "sources", label: "Data sources" }, { value: "services", label: "Platform services" }]} />

      {tab === "sources" ? (<>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <Tile index={0} label="Registered sources" value={data ? data.filter((s) => s.state !== "planned").length : null} icon={Database} color="lavender" note={free != null ? `${free} free to use` : undefined} />
          <Tile index={1} label="Working" value={count("ready")} icon={CheckCircle2} color="sage" />
          <Tile index={2} label="Need attention" value={data ? (count("attention") ?? 0) + (count("not_configured") ?? 0) : null} icon={TriangleAlert} color="peach" tone={(count("attention") ?? 0) > 0 ? "bad" : undefined} note="Failing or not configured" />
          <Tile index={3} label="Planned" value={count("planned")} icon={Hourglass} color="sky" />
        </div>

        <div className="rise overflow-x-auto" style={{ "--i": 4 } as React.CSSProperties}>
          <Segmented value={filter} onChange={setFilter} label="Filter sources" options={[
            { value: "all", label: "All", count: all.length }, { value: "free", label: "Free", count: all.filter((s) => s.access === "free").length },
            { value: "paid", label: "Paid", count: all.filter((s) => s.access === "paid").length },
            { value: "customer_provided", label: "Customer-provided", count: all.filter((s) => s.access === "customer_provided").length },
            { value: "internal", label: "Internal", count: all.filter((s) => s.access === "internal").length },
            { value: "planned", label: "Planned", count: all.filter((s) => s.state === "planned").length },
          ]} />
        </div>

        {q.isLoading ? <Skeleton className="h-56" /> : q.isError ? <ErrorState /> : sections.length === 0 ? <EmptyState icon={Database} text="No sources match." /> : (
          <div className="space-y-9">
            {sections.map(({ meta, items }, si) => (
              <section key={meta.id} aria-labelledby={`sec-${meta.id}`} className="space-y-3.5">
                <SectionHeader meta={meta} index={si} shown={items.length} working={items.filter((s) => s.state === "ready").length} />
                <div className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">{items.map((s, i) => <SourceCard key={s.id} s={s} st={byId.get(s.integration_id)} index={i} />)}</div>
              </section>
            ))}
          </div>
        )}
        <Link href="/operator/aggregation" className="group flex items-center gap-3 rounded-2xl bg-leaf-soft/50 px-4 py-3.5 text-sm ring-1 ring-leaf/15 transition hover:bg-leaf-soft/80">
          <span className="grid size-9 shrink-0 place-items-center rounded-xl bg-white text-leaf-deep ring-1 ring-leaf/20 transition-transform duration-300 group-hover:translate-x-0.5"><ArrowRight className="size-4" aria-hidden /></span>
          <span><b>Which source is used?</b> Priority between sources, freshness limits, a pin per region, where each figure came from, how the sources differ and a recalculation are on Aggregation &amp; policies.</span>
        </Link>
        <p className="text-xs text-muted">Spend on paid APIs is not metered yet, so no figure is shown; only the number of requests per day is capped.</p>
      </>) : (
        <>
          {ints.isLoading ? <Skeleton className="h-40" /> : ints.isError ? <ErrorState /> : (
            <div className="grid gap-4 lg:grid-cols-2 2xl:grid-cols-3">
              {services.map((st, i) => (
                <Card key={st.id} index={i} className="flex flex-col gap-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2"><Server className="size-4 text-slate-400" aria-hidden /><h3 className="text-[15px] font-semibold tracking-tight">{st.name}</h3></div>
                    <Pill tone={st.ready ? "good" : "warn"}>{st.ready ? "Ready" : "Needs setup"}</Pill>
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
                    <Pill tone={i.status === "ready" ? "good" : i.status === "stub" ? "violet" : "warn"}>{i.status === "ready" ? "Ready" : i.status === "stub" ? "Stand-in" : "Not set"}</Pill>
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
